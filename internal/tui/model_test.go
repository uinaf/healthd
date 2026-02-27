package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

func stubRunChecks(results []runner.CheckResult) runChecksFunc {
	return func(_ context.Context, _ []config.CheckConfig, _ string) []runner.CheckResult {
		return results
	}
}

func TestViewShowsLoadingBeforeResults(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s"}, nil, false)
	view := m.View()
	if !strings.Contains(view, "loading") {
		t.Fatalf("expected loading text, got: %q", view)
	}
}

func TestViewRendersResults(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s", Timeout: "5s"}, []config.CheckConfig{
		{Name: "a", Group: "grp", Command: "true"},
		{Name: "b", Group: "grp", Command: "false"},
	}, false)
	m.runChecks = stubRunChecks([]runner.CheckResult{
		{Name: "a", Group: "grp", Passed: true, Reason: "ok", Duration: 5 * time.Millisecond},
		{Name: "b", Group: "grp", Passed: false, Reason: "exit_code=1", Duration: 3 * time.Millisecond},
	})

	// Simulate Init -> checksMsg
	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "2 checks") {
		t.Fatalf("expected check count in view, got: %q", view)
	}
	if !strings.Contains(view, "1 ok") {
		t.Fatalf("expected 1 ok in view, got: %q", view)
	}
	if !strings.Contains(view, "1 fail") {
		t.Fatalf("expected 1 fail in view, got: %q", view)
	}
	if !strings.Contains(view, "grp") {
		t.Fatalf("expected group name in view, got: %q", view)
	}
}

func TestViewWatchModeShowsRefreshing(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "30s", Timeout: "5s"}, []config.CheckConfig{
		{Name: "a", Group: "g", Command: "true"},
	}, true)
	m.runChecks = stubRunChecks([]runner.CheckResult{
		{Name: "a", Group: "g", Passed: true, Reason: "ok"},
	})

	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "refreshing every") {
		t.Fatalf("expected refreshing text in watch mode, got: %q", view)
	}
	if !strings.Contains(view, "press q to quit") {
		t.Fatalf("expected quit hint in watch mode, got: %q", view)
	}
}

func TestNonWatchModeQuits(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s", Timeout: "5s"}, []config.CheckConfig{
		{Name: "a", Group: "g", Command: "true"},
	}, false)
	m.runChecks = stubRunChecks([]runner.CheckResult{
		{Name: "a", Group: "g", Passed: true, Reason: "ok"},
	})

	cmd := m.Init()
	msg := cmd()
	_, nextCmd := m.Update(msg)

	// Non-watch should return tea.Quit
	if nextCmd == nil {
		t.Fatal("expected quit command, got nil")
	}
	quitMsg := nextCmd()
	if _, ok := quitMsg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg, got %T", quitMsg)
	}
}

func TestKeyQQuits(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s"}, nil, true)
	_, cmd := m.Update(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}}))
	if cmd == nil {
		t.Fatal("expected quit command on 'q' key")
	}
}

func TestResultsAccessor(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s"}, nil, false)
	if len(m.Results()) != 0 {
		t.Fatal("expected empty results initially")
	}
}

func TestUngroupedChecks(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s", Timeout: "5s"}, []config.CheckConfig{
		{Name: "a", Command: "true"},
	}, false)
	m.runChecks = stubRunChecks([]runner.CheckResult{
		{Name: "a", Group: "", Passed: true, Reason: "ok"},
	})

	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "ungrouped") {
		t.Fatalf("expected 'ungrouped' heading, got: %q", view)
	}
}

func TestTimedOutCheckRendering(t *testing.T) {
	t.Parallel()
	m := NewModel(config.Config{Interval: "10s", Timeout: "5s"}, []config.CheckConfig{
		{Name: "slow", Group: "g", Command: "sleep 10"},
	}, false)
	m.runChecks = stubRunChecks([]runner.CheckResult{
		{Name: "slow", Group: "g", Passed: false, TimedOut: true, Reason: "timed out"},
	})

	cmd := m.Init()
	msg := cmd()
	updated, _ := m.Update(msg)
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "⚠") {
		t.Fatalf("expected '!' indicator for timed out, got: %q", view)
	}
}
