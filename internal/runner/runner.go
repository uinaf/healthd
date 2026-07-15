package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

// maxCaptureBytes caps how much stdout/stderr is retained per check so a
// runaway command cannot unbounded-grow the process heap.
const maxCaptureBytes = 64 * 1024

type CheckResult struct {
	Name      string
	Group     string
	Command   string
	Stdout    string
	Stderr    string
	ExitCode  int
	Duration  time.Duration
	Passed    bool
	TimedOut  bool
	Canceled  bool
	Reason    string
	Timestamp time.Time
}

func RunChecks(ctx context.Context, checks []config.CheckConfig, defaultTimeout string) []CheckResult {
	results := make([]CheckResult, 0, len(checks))
	for _, check := range checks {
		results = append(results, runCheck(ctx, check, defaultTimeout))
	}
	return results
}

func FilterChecks(checks []config.CheckConfig, only []string, groups []string) []config.CheckConfig {
	nameFilter := toSet(only)
	groupFilter := toSet(groups)

	filtered := make([]config.CheckConfig, 0, len(checks))
	for _, check := range checks {
		if len(nameFilter) > 0 {
			if _, ok := nameFilter[check.Name]; !ok {
				continue
			}
		}
		if len(groupFilter) > 0 {
			if _, ok := groupFilter[check.Group]; !ok {
				continue
			}
		}
		filtered = append(filtered, check)
	}

	return filtered
}

func AllPassed(results []CheckResult) bool {
	for _, result := range results {
		if result.Canceled {
			continue
		}
		if !result.Passed {
			return false
		}
	}
	return true
}

func runCheck(parent context.Context, check config.CheckConfig, defaultTimeout string) CheckResult {
	start := time.Now()
	result := CheckResult{
		Name:      check.Name,
		Group:     check.Group,
		Command:   check.Command,
		ExitCode:  -1,
		Passed:    false,
		Timestamp: start,
	}

	timeout, err := resolveTimeout(check.Timeout, defaultTimeout)
	if err != nil {
		result.Reason = fmt.Sprintf("invalid timeout: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", check.Command)
	cmd.Env = mergedEnv(check.Env)

	stdout := newLimitedBuffer(maxCaptureBytes)
	stderr := newLimitedBuffer(maxCaptureBytes)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()
	result.Duration = time.Since(start)

	if ctx.Err() == context.DeadlineExceeded && parent.Err() == nil {
		// Check-local timeout only when the parent loop context is still live.
		result.TimedOut = true
		result.ExitCode = -1
		result.Reason = "timed out"
		return result
	}

	// Only mark canceled when the command was actually interrupted. A process
	// that already exited with a normal status must keep that result even if
	// the parent context is shutting down.
	if isContextInterrupted(runErr, ctx, parent) {
		result.Canceled = true
		result.ExitCode = -1
		result.Reason = "canceled"
		return result
	}

	result.ExitCode = extractExitCode(runErr)
	passed, reason := evaluateExpectations(check.Expect, result.Stdout, result.ExitCode)
	result.Passed = passed
	result.Reason = reason

	if runErr != nil && result.Reason == "" {
		result.Reason = runErr.Error()
	}

	// Truncated stdout is only a failure when expectations depend on stdout.
	// Exit-code-only checks keep their result; capture is still capped for memory.
	if stdout.Truncated() && expectsStdout(check.Expect) {
		result.Passed = false
		switch {
		case result.Reason == "" || result.Reason == "ok":
			result.Reason = "output truncated"
		case strings.Contains(result.Reason, "truncated"):
		default:
			result.Reason += " (output truncated)"
		}
	}

	return result
}

// isContextInterrupted reports whether the command failed because the run
// context was canceled/deadline-killed, rather than a normal process exit.
func isContextInterrupted(runErr error, ctx, parent context.Context) bool {
	if runErr == nil {
		return false
	}
	if ctx.Err() == nil && parent.Err() == nil {
		return false
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) && exitErr.ExitCode() >= 0 {
		// Process called exit(N) — keep that outcome even if shutdown raced in.
		return false
	}
	return true
}

func resolveTimeout(checkTimeout string, defaultTimeout string) (time.Duration, error) {
	raw := strings.TrimSpace(checkTimeout)
	if raw == "" {
		raw = strings.TrimSpace(defaultTimeout)
	}
	if raw == "" {
		return 0, errors.New("missing timeout")
	}
	return time.ParseDuration(raw)
}

func mergedEnv(overrides map[string]string) []string {
	envMap := map[string]string{}

	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		envMap[key] = value
	}

	for key, value := range overrides {
		envMap[key] = value
	}

	keys := make([]string, 0, len(envMap))
	for key := range envMap {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+envMap[key])
	}
	return result
}

func extractExitCode(err error) int {
	if err == nil {
		return 0
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}

	return -1
}

func evaluateExpectations(expect config.ExpectConfig, stdout string, exitCode int) (bool, string) {
	trimmed := strings.TrimSpace(stdout)

	if !hasExpectation(expect) {
		if exitCode == 0 {
			return true, "ok"
		}
		return false, fmt.Sprintf("expected exit_code=0, got %d", exitCode)
	}

	if expect.ExitCode != nil && exitCode != *expect.ExitCode {
		return false, fmt.Sprintf("expected exit_code=%d, got %d", *expect.ExitCode, exitCode)
	}

	if expect.Equals != nil && trimmed != *expect.Equals {
		return false, fmt.Sprintf("expected equals %q", *expect.Equals)
	}

	if expect.Not != nil && trimmed == *expect.Not {
		return false, fmt.Sprintf("expected output to not equal %q", *expect.Not)
	}

	if expect.Contains != nil && !strings.Contains(trimmed, *expect.Contains) {
		return false, fmt.Sprintf("expected output to contain %q", *expect.Contains)
	}

	if expect.NotContains != nil && strings.Contains(trimmed, *expect.NotContains) {
		return false, fmt.Sprintf("expected output to not contain %q", *expect.NotContains)
	}

	if expect.Min != nil || expect.Max != nil {
		value, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			// Do not echo stdout into Reason — it flows to alerts.log and notifiers.
			return false, "expected numeric output"
		}
		if expect.Min != nil && value < *expect.Min {
			return false, fmt.Sprintf("expected output >= %v", *expect.Min)
		}
		if expect.Max != nil && value > *expect.Max {
			return false, fmt.Sprintf("expected output <= %v", *expect.Max)
		}
	}

	if expect.Regex != nil {
		re, err := regexp.Compile(*expect.Regex)
		if err != nil {
			return false, fmt.Sprintf("invalid regex: %v", err)
		}
		if !re.MatchString(trimmed) {
			return false, fmt.Sprintf("expected output to match regex %q", *expect.Regex)
		}
	}

	return true, "ok"
}

func hasExpectation(expect config.ExpectConfig) bool {
	return expect.ExitCode != nil || expectsStdout(expect)
}

func expectsStdout(expect config.ExpectConfig) bool {
	return expect.Equals != nil ||
		expect.Not != nil ||
		expect.Contains != nil ||
		expect.NotContains != nil ||
		expect.Min != nil ||
		expect.Max != nil ||
		expect.Regex != nil
}

func toSet(values []string) map[string]struct{} {
	set := map[string]struct{}{}

	for _, raw := range values {
		for _, part := range strings.Split(raw, ",") {
			value := strings.TrimSpace(part)
			if value == "" {
				continue
			}
			set[value] = struct{}{}
		}
	}

	return set
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func newLimitedBuffer(limit int) *limitedBuffer {
	return &limitedBuffer{limit: limit}
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.limit <= 0 {
		l.truncated = true
		return len(p), nil
	}
	remaining := l.limit - l.buf.Len()
	if remaining <= 0 {
		l.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		if _, err := l.buf.Write(p[:remaining]); err != nil {
			return 0, err
		}
		l.truncated = true
		return len(p), nil
	}
	return l.buf.Write(p)
}

func (l *limitedBuffer) String() string {
	return l.buf.String()
}

func (l *limitedBuffer) Truncated() bool {
	return l.truncated
}
