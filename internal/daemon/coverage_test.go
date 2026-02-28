package daemon

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

func TestRunLoopAdditionalBranches(t *testing.T) {
	if err := RunLoop(context.Background(), config.Config{Interval: "bad"}, io.Discard); err == nil || !strings.Contains(err.Error(), "parse schedule interval") {
		t.Fatalf("expected interval parse error, got %v", err)
	}

	cfgCooldown := config.Config{
		Interval: "10ms",
		Timeout:  "1s",
		Checks:   []config.CheckConfig{{Name: "ok", Command: "true"}},
		Notify:   config.NotifyConfig{Cooldown: "bad"},
	}
	if err := RunLoop(context.Background(), cfgCooldown, io.Discard); err == nil || !strings.Contains(err.Error(), "parse cooldown") {
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
	if err := RunLoop(context.Background(), cfgBackend, io.Discard); err == nil || !strings.Contains(err.Error(), "unsupported backend type") {
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
	if err := RunLoop(ctx, cfgRun, &out); err != nil {
		t.Fatalf("RunLoop() error = %v", err)
	}
	if !strings.Contains(out.String(), "notify dispatch error for failing") {
		t.Fatalf("expected dispatch error output, got %q", out.String())
	}
}
