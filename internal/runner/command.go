package runner

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// commandExecutionError marks supervision and pipe failures independently of
// the shell exit status, which an output-only expectation may otherwise accept.
type commandExecutionError struct{ cause error }

func (e *commandExecutionError) Error() string { return "command execution failed: " + e.cause.Error() }
func (e *commandExecutionError) Unwrap() error { return e.cause }

const terminationGrace = 100 * time.Millisecond

// CommandOutput retains at most 64 KiB from each stream.
type CommandOutput struct {
	Stdout          string
	Stderr          string
	StdoutTruncated bool
}

// RunCommand owns the shell's process group until its children have been
// signaled. Commands must not daemonize or leave this process group.
func RunCommand(ctx context.Context, command string, env []string) (CommandOutput, error) {
	if err := ctx.Err(); err != nil {
		return CommandOutput{}, &commandExecutionError{cause: interruptionCause(ctx, err)}
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, stderr := newLimitedBuffer(maxCaptureBytes), newLimitedBuffer(maxCaptureBytes)
	buffers := [2]*limitedBuffer{stdout, stderr}
	var readers, writers [2]*os.File
	for i := range readers {
		reader, writer, err := os.Pipe()
		if err != nil {
			return CommandOutput{}, &commandExecutionError{cause: err}
		}
		readers[i], writers[i] = reader, writer
		defer reader.Close()
		defer writer.Close()
	}
	cmd.Stdout, cmd.Stderr = writers[0], writers[1]
	if err := cmd.Start(); err != nil {
		return CommandOutput{}, &commandExecutionError{cause: err}
	}
	copied := make(chan error, 2)
	for i := range readers {
		_ = writers[i].Close()
		go func() {
			_, err := io.Copy(buffers[i], readers[i])
			copied <- err
		}()
	}
	// Observe exit without reaping: the leader reserves its PID/PGID until
	// the last group signal, even when a child outlives the shell.
	pid := cmd.Process.Pid
	watchErr := waitForExit(ctx, pid)
	if errors.Is(watchErr, context.Canceled) || errors.Is(watchErr, context.DeadlineExceeded) {
		watchErr = interruptionCause(ctx, watchErr)
	}
	var cleanupErr error
	if watchErr != nil {
		cleanupErr = signalGroup(pid, syscall.SIGTERM)
		time.Sleep(terminationGrace)
	}
	killErr := signalGroup(pid, syscall.SIGKILL)
	if ignoreCleanupErrorAfterExit(pid, killErr) {
		killErr = nil
	}
	cleanupErr = errors.Join(cleanupErr, killErr)
	if watchErr != nil {
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	// Drain separately: exec.Cmd.Wait hides ErrWaitDelay when the shell has
	// already returned an ExitError. Output-only checks may accept that exit.
	timer := time.NewTimer(terminationGrace)
	defer timer.Stop()
	var drainErr error
	for remaining := 2; remaining > 0; {
		select {
		case err := <-copied:
			drainErr = errors.Join(drainErr, err)
			remaining--
		case <-timer.C:
			drainErr = errors.Join(drainErr, exec.ErrWaitDelay)
			for _, reader := range readers {
				_ = reader.Close()
			}
		}
	}
	waitErr := cmd.Wait()
	output := CommandOutput{Stdout: stdout.String(), Stderr: stderr.String(), StdoutTruncated: stdout.Truncated()}
	err := errors.Join(watchErr, cleanupErr, drainErr, waitErr)
	var exitErr *exec.ExitError
	if watchErr != nil || cleanupErr != nil || drainErr != nil || (waitErr != nil && !errors.As(waitErr, &exitErr)) {
		return output, &commandExecutionError{cause: err}
	}
	return output, waitErr
}

func signalGroup(pid int, signal syscall.Signal) error {
	err := syscall.Kill(-pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// Retain both the standard context error and its first-winner cause before
// cleanup, so callers never infer the outcome from later context changes.
func interruptionCause(ctx context.Context, err error) error {
	cause := context.Cause(ctx)
	if cause == nil || errors.Is(err, cause) {
		return err
	}
	return errors.Join(err, cause)
}
