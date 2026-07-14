package loop

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

func TestRunAdditionalBranches(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	if err := Run(context.Background(), config.Config{Interval: "bad"}, io.Discard); err == nil || !strings.Contains(err.Error(), "parse schedule interval") {
		t.Fatalf("expected interval parse error, got %v", err)
	}

	cfgCooldown := config.Config{
		Interval: "10ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "ok", Command: "true"}},
		Notify:   config.NotifyConfig{Cooldown: "bad"},
	}
	if err := Run(context.Background(), cfgCooldown, io.Discard); err == nil || !strings.Contains(err.Error(), "parse cooldown") {
		t.Fatalf("expected cooldown parse error, got %v", err)
	}

	cfgBackend := config.Config{
		Interval: "10ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "ok", Command: "true"}},
		Notify: config.NotifyConfig{Backends: []config.NotifyBackendConfig{{
			Type: "unsupported",
		}}},
	}
	if err := Run(context.Background(), cfgBackend, io.Discard); err == nil || !strings.Contains(err.Error(), "unsupported backend type") {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}

	cfgRun := config.Config{
		Interval: "20ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "failing", Command: "false"}},
		Notify: config.NotifyConfig{Backends: []config.NotifyBackendConfig{{
			Name:    "broken",
			Type:    "command",
			Command: "exit 1",
		}}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
	defer cancel()
	var out bytes.Buffer
	if err := Run(ctx, cfgRun, &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "notify dispatch error for failing") {
		t.Fatalf("expected dispatch error output, got %q", out.String())
	}

	alertsPath := filepath.Join(homeDir, ".local", "state", "healthd", "alerts.log")
	raw, err := os.ReadFile(alertsPath)
	if err != nil {
		t.Fatalf("read alerts.log: %v", err)
	}
	if !strings.Contains(string(raw), "[crit] failing") {
		t.Fatalf("expected alerts.log to contain transition for failing check, got %q", string(raw))
	}
}

func TestRunFailThenRecoverWritesBothAlerts(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	marker := filepath.Join(t.TempDir(), "state")
	// First tick fails (missing marker); later ticks pass once the marker exists.
	// A short-lived helper flips the marker after the first failure alert.
	checkCmd := fmt.Sprintf(`test -f %q`, marker)

	cfg := config.Config{
		Interval: "25ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "flip", Command: checkCmd}},
		Notify: config.NotifyConfig{Backends: []config.NotifyBackendConfig{{
			Name:    "mark",
			Type:    "command",
			Command: fmt.Sprintf("touch %q", marker),
			Timeout: "1s",
		}}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := Run(ctx, cfg, io.Discard); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	alertsPath := filepath.Join(homeDir, ".local", "state", "healthd", "alerts.log")
	raw, err := os.ReadFile(alertsPath)
	if err != nil {
		t.Fatalf("read alerts.log: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "[crit] flip") {
		t.Fatalf("expected crit alert, got %q", text)
	}
	if !strings.Contains(text, "[recovered] flip") {
		t.Fatalf("expected recovered alert, got %q", text)
	}
}

func TestRunSkipsCancelKilledChecksButKeepsEarlierFailures(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := config.Config{
		Interval: "1h",
		Timeout:  "2s",
		Checks: []config.CheckConfig{
			{Name: "fast-fail", Command: "false"},
			{Name: "slow-ok", Command: "sleep 2"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if err := Run(ctx, cfg, io.Discard); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	alertsPath := filepath.Join(homeDir, ".local", "state", "healthd", "alerts.log")
	raw, err := os.ReadFile(alertsPath)
	if err != nil {
		t.Fatalf("read alerts.log: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "[crit] fast-fail") {
		t.Fatalf("expected genuine earlier failure to alert, got %q", text)
	}
	if strings.Contains(text, "slow-ok") {
		t.Fatalf("expected cancel-killed check to produce no alert, got %q", text)
	}
}

func TestRunCancelDuringSlowPassProducesNoAlert(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	cfg := config.Config{
		Interval: "1h",
		Timeout:  "2s",
		Checks:   []config.CheckConfig{{Name: "slow-ok", Command: "sleep 2"}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	if err := Run(ctx, cfg, io.Discard); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	alertsPath := filepath.Join(homeDir, ".local", "state", "healthd", "alerts.log")
	if _, err := os.Stat(alertsPath); err == nil {
		raw, readErr := os.ReadFile(alertsPath)
		if readErr != nil {
			t.Fatalf("read alerts.log: %v", readErr)
		}
		if strings.Contains(string(raw), "slow-ok") {
			t.Fatalf("expected no cancel-induced alert, got %q", string(raw))
		}
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat alerts.log: %v", err)
	}
}

func TestRunCooldownDefersRecoveryUntilWindowElapses(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	marker := filepath.Join(t.TempDir(), "state")
	checkCmd := fmt.Sprintf(`test -f %q`, marker)

	cfg := config.Config{
		Interval: "30ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "flip", Command: checkCmd}},
		Notify: config.NotifyConfig{
			Cooldown: "100ms",
			Backends: []config.NotifyBackendConfig{{
				Name:    "mark",
				Type:    "command",
				Command: fmt.Sprintf("touch %q", marker),
				Timeout: "1s",
			}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 450*time.Millisecond)
	defer cancel()
	if err := Run(ctx, cfg, io.Discard); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	alertsPath := filepath.Join(homeDir, ".local", "state", "healthd", "alerts.log")
	raw, err := os.ReadFile(alertsPath)
	if err != nil {
		t.Fatalf("read alerts.log: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "[crit] flip") {
		t.Fatalf("expected crit alert, got %q", text)
	}
	if !strings.Contains(text, "[recovered] flip") {
		t.Fatalf("expected deferred recovered alert after cooldown, got %q", text)
	}
}
