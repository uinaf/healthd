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

func TestRunRejectsInvalidSchedulerAndNotifierConfiguration(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	valid := config.Config{
		Interval: "10ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "ok", Command: "true"}},
	}
	invalidCooldown := valid
	invalidCooldown.Notify.Cooldown = "bad"
	invalidBackend := valid
	invalidBackend.Notify.Backends = []config.NotifyBackendConfig{{
		Type: "unsupported",
	}}

	tests := []struct {
		name    string
		config  config.Config
		message string
	}{
		{
			name:    "schedule interval",
			config:  config.Config{Interval: "bad"},
			message: "parse schedule interval",
		},
		{
			name:    "notification cooldown",
			config:  invalidCooldown,
			message: "parse cooldown",
		},
		{
			name:    "notification backend",
			config:  invalidBackend,
			message: "unsupported backend type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Run(context.Background(), test.config, io.Discard)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("expected %q error, got %v", test.message, err)
			}
		})
	}
}

func TestRunReportsNotifierFailureAndPersistsAlert(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

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
