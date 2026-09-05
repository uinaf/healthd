package notify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

func TestParseCooldown(t *testing.T) {
	t.Parallel()

	if d, err := ParseCooldown(""); err != nil || d != 0 {
		t.Fatalf("expected 0,nil for empty cooldown, got %v,%v", d, err)
	}
	if _, err := ParseCooldown("bad"); err == nil || !strings.Contains(err.Error(), "parse cooldown") {
		t.Fatalf("expected parse error, got %v", err)
	}
	if _, err := ParseCooldown("-1s"); err == nil || !strings.Contains(err.Error(), "non-negative") {
		t.Fatalf("expected non-negative error, got %v", err)
	}
}

func TestDispatchRejectsAnEmptyNotifierSet(t *testing.T) {
	t.Parallel()

	if err := Dispatch(context.Background(), Event{}, nil); err == nil || !strings.Contains(err.Error(), "no notifiers configured") {
		t.Fatalf("expected no notifiers error, got %v", err)
	}
}

func TestBuildNotifiersFiltersAndValidatesBackends(t *testing.T) {
	t.Parallel()

	cfg := config.NotifyConfig{Backends: []config.NotifyBackendConfig{
		{Name: "cmd", Type: "command", Command: "true"},
		{Name: "web", Type: "webhook", URL: "http://127.0.0.1"},
	}}

	notifiers, err := BuildNotifiers(cfg, []string{"webhook"})
	if err != nil {
		t.Fatalf("BuildNotifiers() error = %v", err)
	}
	if len(notifiers) != 1 || notifiers[0].Name() != "web" {
		t.Fatalf("unexpected filtered notifiers: %+v", notifiers)
	}

	_, err = BuildNotifiers(config.NotifyConfig{Backends: []config.NotifyBackendConfig{{
		Type:    "command",
		Command: "true",
		Timeout: "bad",
	}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout parse error, got %v", err)
	}

	_, err = BuildNotifiers(config.NotifyConfig{Backends: []config.NotifyBackendConfig{{Type: "unknown"}}}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported backend type") {
		t.Fatalf("expected unsupported type error, got %v", err)
	}

	notifiers, err = BuildNotifiers(config.NotifyConfig{Backends: []config.NotifyBackendConfig{{
		Type:  "ntfy",
		Topic: "alerts",
	}}}, nil)
	if err != nil {
		t.Fatalf("BuildNotifiers() error = %v", err)
	}
	if len(notifiers) != 1 {
		t.Fatalf("expected one notifier, got %d", len(notifiers))
	}
	ntfy, ok := notifiers[0].(*ntfyNotifier)
	if !ok {
		t.Fatalf("expected ntfy notifier, got %T", notifiers[0])
	}
	if ntfy.Name() != "ntfy" || ntfy.url != "https://ntfy.sh/alerts" {
		t.Fatalf("unexpected ntfy notifier: name=%q url=%q", ntfy.Name(), ntfy.url)
	}
}

func TestBackendsReportDeliveryFailures(t *testing.T) {
	t.Parallel()

	cmdNotifier := &commandNotifier{name: "cmd", command: `echo fail; exit 2`, timeout: time.Second}
	if err := cmdNotifier.Notify(context.Background(), Event{CheckName: "api", State: StateCrit}); err == nil || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("expected command failure, got %v", err)
	}

	var capturedBody string
	successClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodPost {
				t.Fatalf("unexpected method %q", req.Method)
			}
			if req.Header.Get("Content-Type") != "application/json" {
				t.Fatalf("missing content-type header")
			}
			body, _ := io.ReadAll(req.Body)
			capturedBody = string(body)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}, nil
		}),
	}

	webhook := &webhookNotifier{name: "web", url: "http://local/webhook", client: successClient}
	if webhook.Name() != "web" {
		t.Fatalf("unexpected webhook name %q", webhook.Name())
	}
	if err := webhook.Notify(context.Background(), Event{CheckName: "api", State: StateWarn}); err != nil {
		t.Fatalf("webhook notify failed: %v", err)
	}
	if !strings.Contains(capturedBody, `"check_name":"api"`) {
		t.Fatalf("unexpected webhook payload %q", capturedBody)
	}

	failClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("nope")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	if err := (&webhookNotifier{name: "web", url: "http://local/webhook", client: failClient}).Notify(context.Background(), Event{}); err == nil || !strings.Contains(err.Error(), "webhook status 502") {
		t.Fatalf("expected webhook status error, got %v", err)
	}

	var ntfyBody string
	ntfyClient := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			ntfyBody = string(body)
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Body:       io.NopCloser(strings.NewReader("nope")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	ntfy := &ntfyNotifier{name: "ntfy-main", url: "http://local/ntfy", client: ntfyClient}
	if ntfy.Name() != "ntfy-main" {
		t.Fatalf("unexpected ntfy name %q", ntfy.Name())
	}
	if err := ntfy.Notify(context.Background(), Event{CheckName: "api", State: StateCrit, Reason: "bad"}); err == nil || !strings.Contains(err.Error(), "ntfy status 502") {
		t.Fatalf("expected ntfy status error, got %v", err)
	}
	if !strings.Contains(ntfyBody, "api -> crit (bad)") {
		t.Fatalf("unexpected ntfy body %q", ntfyBody)
	}
}

func TestStateForResultMapsHealthStates(t *testing.T) {
	t.Parallel()

	if got := StateForResult(runner.CheckResult{Passed: true}); got != StateOK {
		t.Fatalf("expected ok, got %q", got)
	}
	if got := StateForResult(runner.CheckResult{Passed: false, ExitCode: 1}); got != StateCrit {
		t.Fatalf("expected crit, got %q", got)
	}
	if got := StateForResult(runner.CheckResult{Passed: false, ExitCode: 0}); got != StateWarn {
		t.Fatalf("expected warn, got %q", got)
	}
	if got := StateForResult(runner.CheckResult{Canceled: true, ExitCode: -1}); got != StateOK {
		t.Fatalf("expected canceled to be non-crit, got %q", got)
	}
}

func TestTrackerIgnoresCanceledResults(t *testing.T) {
	t.Parallel()

	tracker := NewTracker(0)
	if _, ok := tracker.EventFor(runner.CheckResult{Name: "x", Canceled: true, ExitCode: -1}); ok {
		t.Fatal("expected canceled result to produce no event")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestCommandNotifierBoundedCancellation(t *testing.T) {
	for _, parentCancel := range []bool{false, true} {
		t.Run(fmt.Sprintf("parentCancel=%v", parentCancel), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timeout := 100 * time.Millisecond
			if parentCancel {
				timeout = 2 * time.Second
				timer := time.AfterFunc(100*time.Millisecond, cancel)
				defer timer.Stop()
			}
			notifier := &commandNotifier{name: "synthetic", command: `trap '' TERM; sleep 3 & wait`, timeout: timeout}
			start := time.Now()
			err := notifier.Notify(ctx, Event{})
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("completion exceeded bound: %v", elapsed)
			}
			want := context.DeadlineExceeded
			if parentCancel {
				want = context.Canceled
			}
			if !errors.Is(err, want) {
				t.Errorf("expected %v, got %v", want, err)
			}
		})
	}
}

func TestCommandNotifierBoundsFailureOutput(t *testing.T) {
	notifier := &commandNotifier{name: "synthetic", command: `head -c 200000 /dev/zero; head -c 200000 /dev/zero >&2; exit 1`, timeout: time.Second}
	err := notifier.Notify(context.Background(), Event{})
	if err == nil {
		t.Fatal("expected failure")
	}
	if len(err.Error()) > 2*64*1024+100 {
		t.Fatalf("unbounded failure output: %d bytes", len(err.Error()))
	}
}

func TestNotifierDetachedPipeHelper(t *testing.T) {
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
	cmd := exec.Command(os.Args[0], "-test.run=^TestNotifierDetachedPipeHelper$")
	cmd.Env = append(os.Environ(), "HEALTHD_TEST_PIPE_MODE=child")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestCommandNotifierPreservesPipeFailure(t *testing.T) {
	t.Setenv("HELPER", os.Args[0])
	t.Setenv("HEALTHD_TEST_PIPE_MODE", "parent")
	// Remove only the race runtime's artificial post-exit delay in the fixture.
	t.Setenv("GORACE", "atexit_sleep_ms=0")
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
			t.Setenv("HEALTHD_TEST_PIPE_DONE", doneFile)
			notifier := &commandNotifier{name: "synthetic", command: `"$HELPER" -test.run='^TestNotifierDetachedPipeHelper$'; exit ` + exit, timeout: 3 * time.Second}
			start := time.Now()
			err := notifier.Notify(context.Background(), Event{})
			if !errors.Is(err, exec.ErrWaitDelay) {
				t.Errorf("pipe failure lost: %v", err)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Errorf("pipe drain exceeded bound: %v", elapsed)
			}
		})
	}
}
