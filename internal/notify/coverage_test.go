package notify

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

func TestParseCooldownAndDispatchEmpty(t *testing.T) {
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
	if err := Dispatch(context.Background(), Event{}, nil); err == nil || !strings.Contains(err.Error(), "no notifiers configured") {
		t.Fatalf("expected no notifiers error, got %v", err)
	}
}

func TestBuildNotifiersFilterAndErrors(t *testing.T) {
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

func TestCommandWebhookAndNtfyNotifyBranches(t *testing.T) {
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

func TestStateForResultWarnBranch(t *testing.T) {
	t.Parallel()

	if got := stateForResult(runner.CheckResult{Passed: true}); got != StateOK {
		t.Fatalf("expected ok, got %q", got)
	}
	if got := stateForResult(runner.CheckResult{Passed: false, ExitCode: 1}); got != StateCrit {
		t.Fatalf("expected crit, got %q", got)
	}
	if got := stateForResult(runner.CheckResult{Passed: false, ExitCode: 0}); got != StateWarn {
		t.Fatalf("expected warn, got %q", got)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
