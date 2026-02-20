package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotifyTestCommandUsesBackendsWithoutRunningChecks(t *testing.T) {
	tmpDir := t.TempDir()
	notifyMarker := filepath.Join(tmpDir, "notify.txt")
	checkMarker := filepath.Join(tmpDir, "check.txt")
	configPath := writeTestConfig(t, fmt.Sprintf(`
interval = "1s"
timeout = "1s"

[[check]]
name = "would-run"
command = "printf check > %s"

[[notify.backend]]
name = "local-cmd"
type = "command"
command = "printf notify > %s"
`, checkMarker, notifyMarker))

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"notify", "test", "--config", configPath, "--backend", "local-cmd"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	notifyContent, err := os.ReadFile(notifyMarker)
	if err != nil {
		t.Fatalf("expected notify marker file, got %v", err)
	}
	if strings.TrimSpace(string(notifyContent)) != "notify" {
		t.Fatalf("unexpected notify marker content: %q", string(notifyContent))
	}

	if _, err := os.Stat(checkMarker); !os.IsNotExist(err) {
		t.Fatalf("expected check marker to be absent, stat err=%v", err)
	}

	if !strings.Contains(out.String(), "notify test sent") {
		t.Fatalf("expected confirmation output, got %q", out.String())
	}
	if strings.TrimSpace(errOut.String()) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func TestNotifyTestCommandFailsWhenBackendFilterMissesAll(t *testing.T) {
	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok"
command = "true"

[[notify.backend]]
name = "local-cmd"
type = "command"
command = "true"
`)

	root := NewRootCommand()
	root.SetArgs([]string{"notify", "test", "--config", configPath, "--backend", "does-not-exist"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "no notifier backends matched") {
		t.Fatalf("expected missing backend error, got %v", err)
	}
}
