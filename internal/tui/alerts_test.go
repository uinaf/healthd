package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRecentAlertsParsesAndTails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "alerts.log")
	content := "\n" +
		"2026-02-27T08:37:00Z [crit] openclaw-up-to-date (services) - expected exit_code=0, got 1\n" +
		"bad line\n" +
		"2026-02-27T13:00:00Z [recovered] colima-running (services) - ok\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write log: %v", err)
	}

	alerts, err := LoadRecentAlerts(path, 1)
	if err != nil {
		t.Fatalf("LoadRecentAlerts error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}
	if alerts[0].State != "recovered" {
		t.Fatalf("expected recovered state, got %q", alerts[0].State)
	}
	if alerts[0].CheckName != "colima-running" {
		t.Fatalf("expected check name colima-running, got %q", alerts[0].CheckName)
	}
}

func TestLoadRecentAlertsMissingFileIsEmpty(t *testing.T) {
	t.Parallel()

	alerts, err := LoadRecentAlerts(filepath.Join(t.TempDir(), "missing.log"), 10)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("expected no alerts, got %d", len(alerts))
	}
}
