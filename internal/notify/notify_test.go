package notify

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

func TestTrackerTransitionsAndSuppression(t *testing.T) {
	timestamps := []time.Time{
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 1, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 2, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 3, 0, 0, time.UTC),
	}
	tracker := NewTracker(0)

	result := runner.CheckResult{Name: "api", Timestamp: timestamps[0], Passed: false, ExitCode: 1, Reason: "exit 1"}
	event, ok := tracker.EventFor(result)
	if !ok {
		t.Fatal("expected initial failing transition event")
	}
	if event.State != StateCrit {
		t.Fatalf("expected crit state, got %q", event.State)
	}

	result.Timestamp = timestamps[1]
	event, ok = tracker.EventFor(result)
	if ok {
		t.Fatalf("expected duplicate state to be suppressed, got %+v", event)
	}

	result.Timestamp = timestamps[2]
	result.Passed = true
	result.ExitCode = 0
	result.Reason = "ok"
	event, ok = tracker.EventFor(result)
	if !ok {
		t.Fatal("expected recovery event")
	}
	if event.State != StateRecovered {
		t.Fatalf("expected recovered state, got %q", event.State)
	}
	if event.Previous != StateCrit {
		t.Fatalf("expected previous crit, got %q", event.Previous)
	}

	result.Timestamp = timestamps[3]
	event, ok = tracker.EventFor(result)
	if ok {
		t.Fatalf("expected ok->ok suppression, got %+v", event)
	}
}

func TestTrackerCooldownSuppressesRapidTransitions(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tracker := NewTracker(30 * time.Second)
	tracker.now = func() time.Time { return now }

	result := runner.CheckResult{Name: "db", Timestamp: now, Passed: false, ExitCode: 1, Reason: "exit 1"}
	if _, ok := tracker.EventFor(result); !ok {
		t.Fatal("expected first event")
	}

	now = now.Add(5 * time.Second)
	result.Passed = true
	result.ExitCode = 0
	result.Reason = "ok"
	if _, ok := tracker.EventFor(result); ok {
		t.Fatal("expected cooldown suppression")
	}

	now = now.Add(40 * time.Second)
	result.Passed = false
	result.ExitCode = 2
	result.Reason = "exit 2"
	event, ok := tracker.EventFor(result)
	if !ok {
		t.Fatal("expected event after cooldown elapsed")
	}
	if event.State != StateCrit {
		t.Fatalf("expected crit state, got %q", event.State)
	}
}

func TestDispatchRunsBackendsInParallelAndIsolatesFailures(t *testing.T) {
	var mu sync.Mutex
	hits := map[string]int{}

	notifiers := []Notifier{
		&stubNotifier{name: "slow-success", delay: 120 * time.Millisecond, hits: hits, mu: &mu},
		&stubNotifier{name: "fast-fail", delay: 10 * time.Millisecond, fail: errors.New("boom"), hits: hits, mu: &mu},
		&stubNotifier{name: "fast-success", delay: 20 * time.Millisecond, hits: hits, mu: &mu},
	}

	start := time.Now()
	err := Dispatch(context.Background(), Event{CheckName: "api", State: StateCrit}, notifiers)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if !strings.Contains(err.Error(), "fast-fail") {
		t.Fatalf("expected notifier name in error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if hits["slow-success"] != 1 || hits["fast-success"] != 1 || hits["fast-fail"] != 1 {
		t.Fatalf("expected all notifiers to run exactly once, hits=%v", hits)
	}
	if elapsed >= 220*time.Millisecond {
		t.Fatalf("expected parallel dispatch, elapsed=%v", elapsed)
	}
}

func TestBuildNotifiersAndCommandBackend(t *testing.T) {
	tmp := t.TempDir()
	marker := filepath.Join(tmp, "marker.txt")
	cfg := config.NotifyConfig{
		Backends: []config.NotifyBackendConfig{
			{
				Name:    "cmd",
				Type:    "command",
				Command: fmt.Sprintf("printf \"$HEALTHD_EVENT_STATE\" > %s", marker),
			},
		},
	}

	notifiers, err := BuildNotifiers(cfg, []string{"cmd"})
	if err != nil {
		t.Fatalf("BuildNotifiers() error = %v", err)
	}
	if len(notifiers) != 1 {
		t.Fatalf("expected one notifier, got %d", len(notifiers))
	}

	err = Dispatch(context.Background(), Event{CheckName: "api", State: StateWarn}, notifiers)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	content, readErr := os.ReadFile(marker)
	if readErr != nil {
		t.Fatalf("failed to read marker: %v", readErr)
	}
	if strings.TrimSpace(string(content)) != string(StateWarn) {
		t.Fatalf("expected command backend to receive env state, got %q", string(content))
	}
}

type stubNotifier struct {
	name  string
	delay time.Duration
	fail  error
	hits  map[string]int
	mu    *sync.Mutex
}

func (n *stubNotifier) Name() string { return n.name }

func (n *stubNotifier) Notify(ctx context.Context, event Event) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(n.delay):
	}

	n.mu.Lock()
	n.hits[n.name]++
	n.mu.Unlock()

	return n.fail
}
