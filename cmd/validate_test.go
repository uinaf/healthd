package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestValidateCommandSuccessAndError(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok"
command = "true"
`)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"validate", "--config", configPath})
	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out.String(), "config is valid") {
		t.Fatalf("expected success output, got %q", out.String())
	}

	root = NewRootCommand()
	root.SetArgs([]string{"validate", "--config", "/definitely/missing/healthd.toml"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "failed to decode config") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestCheckJSONErrorWhenFiltersMatchNothing(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "cpu"
group = "host"
command = "true"
`)

	result := executeCheckCommand(t, "check", "--config", configPath, "--json", "--only", "missing")
	if result.err == nil {
		t.Fatal("expected non-nil error")
	}
	if strings.TrimSpace(result.stderr) != "" {
		t.Fatalf("expected empty stderr, got %q", result.stderr)
	}

	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(strings.TrimSpace(result.stdout)), &report); err != nil {
		t.Fatalf("failed to decode json output: %v", err)
	}
	var ok bool
	if err := json.Unmarshal(report["ok"], &ok); err != nil {
		t.Fatalf("failed to decode ok field: %v", err)
	}
	if ok {
		t.Fatalf("expected ok=false report, got %q", result.stdout)
	}
	var message string
	if err := json.Unmarshal(report["error"], &message); err != nil {
		t.Fatalf("failed to decode error field: %v", err)
	}
	if !strings.Contains(message, "no checks matched filters") {
		t.Fatalf("unexpected error message %q", message)
	}
}
