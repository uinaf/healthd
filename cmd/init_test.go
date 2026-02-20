package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/healthd/internal/config"
)

func TestInitCommandCreatesStarterConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "nested", "healthd.toml")

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"init", "--config", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected init to succeed, got %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file to exist, got %v", err)
	}
	if !strings.Contains(string(content), "[[check]]") {
		t.Fatalf("expected starter config content, got %q", string(content))
	}
	if !strings.Contains(out.String(), "wrote starter config") {
		t.Fatalf("expected success output, got %q", out.String())
	}
	if _, err := config.LoadFromPath(configPath); err != nil {
		t.Fatalf("expected starter config to validate, got %v", err)
	}
}

func TestInitCommandRefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "healthd.toml")
	initial := "interval = \"1s\"\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"init", "--config", configPath})

	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already exists error, got %v", err)
	}

	content, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if string(content) != initial {
		t.Fatalf("expected existing config to remain unchanged, got %q", string(content))
	}
}

func TestInitCommandForceOverwritesExistingConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "healthd.toml")
	if err := os.WriteFile(configPath, []byte("old-content\n"), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}

	root := NewRootCommand()
	root.SetArgs([]string{"init", "--config", configPath, "--force"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected force init to succeed, got %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(content), "old-content") {
		t.Fatalf("expected existing content to be replaced, got %q", string(content))
	}
	if _, err := config.LoadFromPath(configPath); err != nil {
		t.Fatalf("expected overwritten config to validate, got %v", err)
	}
}
