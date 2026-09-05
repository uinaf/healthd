package runner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

func TestRunChecksOwnedChildren(t *testing.T) {
	for _, mode := range []string{"timeout", "parent cancel", "leader exit"} {
		t.Run(mode, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			command := `sh -c 'trap "" TERM; echo $$ > "$PID_FILE"; sleep 3' & `
			if mode == "leader exit" {
				command += `while [ ! -s "$PID_FILE" ]; do :; done; exit 0`
			} else {
				command += `wait`
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := "100ms"
			if mode == "parent cancel" {
				timeout = "2s"
			}
			done := make(chan CheckResult, 1)
			start := time.Now()
			go func() {
				done <- RunChecks(ctx, []config.CheckConfig{{Name: "synthetic", Command: command, Timeout: timeout, Env: map[string]string{"PID_FILE": pidFile}}}, "2s")[0]
			}()
			if mode == "parent cancel" {
				waitForPIDFile(t, pidFile)
				cancel()
			}
			result := <-done
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("completion exceeded bound: %v", elapsed)
			}
			switch mode {
			case "timeout":
				if !result.TimedOut || result.Canceled {
					t.Errorf("expected local timeout: %+v", result)
				}
			case "parent cancel":
				if !result.Canceled || result.TimedOut {
					t.Errorf("expected parent cancellation: %+v", result)
				}
			case "leader exit":
				if !result.Passed || result.ExitCode != 0 {
					t.Errorf("leader result changed: %+v", result)
				}
			}
			pid := waitForPIDFile(t, pidFile)
			assertProcessStopped(t, pid)
		})
	}
}

func waitForPIDFile(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
				return pid
			}
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("child did not become ready")
	return 0
}

func TestRunCommandCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	marker := filepath.Join(t.TempDir(), "started")
	_, err := RunCommand(ctx, `touch "$MARKER"`, append(os.Environ(), "MARKER="+marker))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled, got %v", err)
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("command started: %v", err)
	}
}

func TestRunCommandCaptureBound(t *testing.T) {
	output, err := RunCommand(context.Background(), `head -c 200000 /dev/zero; head -c 200000 /dev/zero >&2`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Stdout) != maxCaptureBytes || len(output.Stderr) != maxCaptureBytes || !output.StdoutTruncated {
		t.Fatalf("unexpected capture lengths: stdout=%d stderr=%d truncated=%v", len(output.Stdout), len(output.Stderr), output.StdoutTruncated)
	}
}

// This subprocess creates a finite, detached writer to exercise WaitDelay.
// It deliberately violates the command ownership contract; no live service is used.
func TestDetachedPipeHelper(t *testing.T) {
	mode := os.Getenv("HEALTHD_TEST_PIPE_MODE")
	if mode == "" {
		return
	}
	if mode == "child" {
		time.Sleep(2 * time.Second)
		_ = os.Stdout.Close()
		_ = os.Stderr.Close()
		if err := os.WriteFile(os.Getenv("HEALTHD_TEST_PIPE_DONE"), []byte("done"), 0o600); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestDetachedPipeHelper$")
	cmd.Env = append(os.Environ(), "HEALTHD_TEST_PIPE_MODE=child")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// GORACE removes only the helper runtime's artificial post-exit sleep.
func TestRunChecksPipeFailureCannotPassExpectation(t *testing.T) {
	for _, exit := range []string{"0", "7"} {
		t.Run("exit"+exit, func(t *testing.T) {
			doneFile := filepath.Join(t.TempDir(), "detached.done")
			t.Cleanup(func() {
				deadline := time.Now().Add(3 * time.Second)
				for time.Now().Before(deadline) {
					if _, err := os.Stat(doneFile); err == nil {
						return
					}
					time.Sleep(10 * time.Millisecond)
				}
				t.Error("detached fixture did not finish")
			})
			expected := ""
			start := time.Now()
			result := RunChecks(context.Background(), []config.CheckConfig{{
				Name: "synthetic", Command: `"$HELPER" -test.run='^TestDetachedPipeHelper$'; exit ` + exit,
				Env:    map[string]string{"HELPER": os.Args[0], "HEALTHD_TEST_PIPE_MODE": "parent", "GORACE": "atexit_sleep_ms=0", "HEALTHD_TEST_PIPE_DONE": doneFile},
				Expect: config.ExpectConfig{Equals: &expected},
			}}, "3s")[0]
			if result.Passed || result.ExitCode != -1 || !strings.Contains(result.Reason, "WaitDelay") {
				t.Fatalf("pipe failure accepted or lost: %+v", result)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("pipe drain exceeded bound: %v", elapsed)
			}
		})
	}
}

func TestRunChecksOutputOnlyAcceptsOrdinaryNonzeroExit(t *testing.T) {
	expected := "ok"
	result := RunChecks(context.Background(), []config.CheckConfig{{Name: "synthetic", Command: `printf ok; exit 7`, Expect: config.ExpectConfig{Equals: &expected}}}, "1s")[0]
	if !result.Passed || result.ExitCode != 7 {
		t.Fatalf("ordinary output-only semantics changed: %+v", result)
	}
}

func TestRunCommandAllowsTermCleanup(t *testing.T) {
	ready := filepath.Join(t.TempDir(), "ready")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var output CommandOutput
	var runErr error
	go func() {
		defer close(done)
		output, runErr = RunCommand(ctx, `trap 'printf cleaned; exit 0' TERM; echo $$ > "$READY"; while :; do :; done`, append(os.Environ(), "READY="+ready))
	}()
	waitForPIDFile(t, ready)
	cancel()
	<-done
	if output.Stdout != "cleaned" || !errors.Is(runErr, context.Canceled) {
		t.Fatalf("TERM cleanup/cancellation lost: output=%q err=%v", output.Stdout, runErr)
	}
}

// Done is first read after the watcher misses an exit. Release the child at
// exactly that boundary, and independently observe its exit without reaping it.
type exitOnDoneContext struct {
	context.Context
	beforeDone func()
	once       sync.Once
}

func (c *exitOnDoneContext) Done() <-chan struct{} {
	c.once.Do(c.beforeDone)
	return c.Context.Done()
}

func TestWaitForExitPrefersExitAtCancellationBoundary(t *testing.T) {
	cmd := exec.Command("sh", "-c", "read ignored; exit 7")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	t.Cleanup(func() {
		_ = stdin.Close()
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := &exitOnDoneContext{Context: parent, beforeDone: func() {
		if err := stdin.Close(); err != nil {
			t.Fatal(err)
		}
		observation, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		if err := waitForExit(observation, cmd.Process.Pid); err != nil {
			t.Fatalf("observe released child: %v", err)
		}
		awaitZombie(t, cmd.Process.Pid)
		cancel()
	}}
	if err := waitForExit(ctx, cmd.Process.Pid); err != nil {
		t.Fatalf("already-observable exit lost to cancellation: %v", err)
	}
	err = cmd.Wait()
	waited = true
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 7 {
		t.Fatalf("exit status lost or leader reaped early: %v", err)
	}
}

func TestRunCheckClassifiesObservedOutcomeAfterCleanup(t *testing.T) {
	exit7 := exec.Command("sh", "-c", "exit 7").Run()
	if exit7 == nil {
		t.Fatal("expected fixture exit 7")
	}
	for _, tc := range []struct {
		name                                  string
		deadline                              bool
		parentCancel                          bool
		observed                              error
		wantTimeout, wantCanceled, wantPassed bool
		wantExit                              int
		wantReason                            string
	}{
		{name: "completed exit after local deadline", deadline: true, wantPassed: true, wantExit: 0, wantReason: "ok"},
		{name: "ordinary failure after local deadline", deadline: true, observed: exit7, wantExit: 7, wantReason: "expected exit_code=0, got 7"},
		{name: "drain failure after local deadline", deadline: true, observed: &commandExecutionError{cause: exec.ErrWaitDelay}, wantExit: -1, wantReason: "WaitDelay"},
		{name: "drain failure after parent cancellation", parentCancel: true, observed: &commandExecutionError{cause: exec.ErrWaitDelay}, wantExit: -1, wantReason: "WaitDelay"},
		{name: "supervision failure after parent cancellation", parentCancel: true, observed: &commandExecutionError{cause: errors.New("observation failed")}, wantExit: -1, wantReason: "observation failed"},
		{name: "completed exit after parent cancellation", parentCancel: true, wantPassed: true, wantExit: 0, wantReason: "ok"},
		{name: "observed local timeout before parent cancellation", deadline: true, parentCancel: true, observed: &commandExecutionError{cause: errors.Join(context.DeadlineExceeded, errCheckTimeout)}, wantTimeout: true, wantExit: -1, wantReason: "timed out"},
		{name: "observed parent cancellation", parentCancel: true, observed: &commandExecutionError{cause: context.Canceled}, wantCanceled: true, wantExit: -1, wantReason: "canceled"},
		{name: "observed parent deadline", observed: &commandExecutionError{cause: context.DeadlineExceeded}, wantCanceled: true, wantExit: -1, wantReason: "canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := "1s"
			if tc.deadline {
				timeout = "1ms"
			}
			// The command boundary supplies an already-observed outcome after cleanup
			// changes the contexts. This makes classification races deterministic.
			result := runCheckWithCommand(parent, config.CheckConfig{Name: "synthetic", Command: "unused"}, timeout, func(ctx context.Context, _ string, _ []string) (CommandOutput, error) {
				if tc.deadline {
					<-ctx.Done()
				}
				if tc.parentCancel {
					cancel()
				}
				return CommandOutput{}, tc.observed
			})
			if result.TimedOut != tc.wantTimeout || result.Canceled != tc.wantCanceled || result.Passed != tc.wantPassed || result.ExitCode != tc.wantExit || !strings.Contains(result.Reason, tc.wantReason) {
				t.Fatalf("observed outcome changed during cleanup: %+v", result)
			}
		})
	}
}

func TestRunCommandRetainsInterruptionCause(t *testing.T) {
	for _, preStart := range []bool{false, true} {
		t.Run(strconv.FormatBool(preStart), func(t *testing.T) {
			cause := errors.New("fixture timeout")
			ctx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Millisecond, cause)
			defer cancel()
			if preStart {
				<-ctx.Done()
			}
			_, err := RunCommand(ctx, "sleep 3", nil)
			if !errors.Is(err, context.DeadlineExceeded) || !errors.Is(err, cause) {
				t.Fatalf("interruption cause lost: %v", err)
			}
		})
	}
}
