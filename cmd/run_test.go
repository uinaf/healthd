package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunCommandStopsOnCancelAndWritesAlerts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	configPath := filepath.Join(t.TempDir(), "config.toml")
	content := `
interval = "20ms"
timeout = "1s"

[[check]]
name = "failing"
command = "false"
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0o600))

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetContext(ctx)
	root.SetArgs([]string{"run", "--config", configPath})

	err := root.Execute()
	require.NoError(t, err)

	alertsPath := filepath.Join(home, ".local", "state", "healthd", "alerts.log")
	raw, readErr := os.ReadFile(alertsPath)
	require.NoError(t, readErr)
	require.Contains(t, string(raw), "[crit] failing")
}

func TestRunCommandRejectsBadConfig(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
interval = "bad"
timeout = "1s"

[[check]]
name = "x"
command = "true"
`), 0o600))

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"run", "--config", configPath})

	err := root.Execute()
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid config")
	require.Contains(t, err.Error(), "interval")
}
