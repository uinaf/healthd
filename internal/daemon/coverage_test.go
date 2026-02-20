package daemon

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

func TestManagerUninstallAndStatusBranches(t *testing.T) {
	tmp := t.TempDir()
	paths := DefaultPaths(tmp)
	if err := os.MkdirAll(filepath.Dir(paths.PlistPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(paths.PlistPath, []byte("plist"), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}

	m := NewManager()
	m.homeDir = func() (string, error) { return tmp, nil }
	m.launchctl = &fakeLaunchCtl{listErr: errors.New("not running")}

	if _, err := m.Uninstall(); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if _, err := os.Stat(paths.PlistPath); !os.IsNotExist(err) {
		t.Fatalf("expected removed plist, got %v", err)
	}

	status, _, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.Installed || status.Running || status.PID != 0 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestManagerErrorBranches(t *testing.T) {
	m := NewManager()
	m.homeDir = func() (string, error) { return "", errors.New("home failed") }
	if _, err := m.Uninstall(); err == nil || !strings.Contains(err.Error(), "determine user home") {
		t.Fatalf("expected homeDir error, got %v", err)
	}

	m = NewManager()
	m.homeDir = func() (string, error) { return "/tmp/home", nil }
	m.remove = func(string) error { return errors.New("remove failed") }
	if _, err := m.Uninstall(); err == nil || !strings.Contains(err.Error(), "remove launchagent plist") {
		t.Fatalf("expected remove error, got %v", err)
	}

	m = NewManager()
	m.homeDir = func() (string, error) { return "/tmp/home", nil }
	m.stat = func(string) (os.FileInfo, error) { return nil, errors.New("stat failed") }
	if _, _, err := m.Status(); err == nil || !strings.Contains(err.Error(), "read launchagent status") {
		t.Fatalf("expected status stat error, got %v", err)
	}
}

func TestManagerReadLogsAndTailFileBranches(t *testing.T) {
	m := NewManager()
	m.homeDir = func() (string, error) { return "/tmp/home", nil }
	m.readFile = func(path string) ([]byte, error) {
		switch filepath.Base(path) {
		case "stdout.log":
			return []byte("a\nb\nc\n"), nil
		case "stderr.log":
			return nil, os.ErrNotExist
		default:
			return nil, errors.New("unexpected path")
		}
	}

	stdout, stderr, err := m.ReadLogs(2)
	if err != nil {
		t.Fatalf("ReadLogs() error = %v", err)
	}
	if stdout != "b\nc" {
		t.Fatalf("expected tailed stdout, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected missing stderr to be empty, got %q", stderr)
	}

	m.readFile = func(string) ([]byte, error) { return nil, errors.New("boom") }
	if _, err := m.tailFile("/tmp/stdout.log", 5); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected tailFile error, got %v", err)
	}
}

func TestTailLinesAndParsePIDBranches(t *testing.T) {
	// Exercise parse failure path for overflow value.
	if pid := ParsePID(`{"PID" = 999999999999999999999999999999999999;}`); pid != 0 {
		t.Fatalf("expected overflow pid to parse as 0, got %d", pid)
	}
	if got := TailLines([]byte(""), 3); got != "" {
		t.Fatalf("expected empty output, got %q", got)
	}
	if got := TailLines([]byte("x\ny\n"), 0); got != "" {
		t.Fatalf("expected empty output for zero lines, got %q", got)
	}
	if got := TailLines(bytes.TrimSpace([]byte("x\ny")), 1); got != "y" {
		t.Fatalf("expected last line, got %q", got)
	}
}

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

func TestExecLaunchCtlSuccessAndFailure(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "launchctl")
	successScript := "#!/bin/sh\ncase \"$1\" in\n  load) exit 0 ;;\n  unload) exit 0 ;;\n  list) echo '\"PID\" = 123;'; exit 0 ;;\nesac\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(successScript), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	ctl := execLaunchCtl{}
	if err := ctl.Load("/tmp/test.plist"); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if err := ctl.Unload("/tmp/test.plist"); err != nil {
		t.Fatalf("Unload() error = %v", err)
	}
	out, err := ctl.List(LaunchAgentLabel)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if ParsePID(out) != 123 {
		t.Fatalf("expected PID 123 from list output, got %q", out)
	}

	failScript := "#!/bin/sh\necho 'failed' >&2\nexit 1\n"
	if err := os.WriteFile(scriptPath, []byte(failScript), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	if err := ctl.Load("/tmp/test.plist"); err == nil || !strings.Contains(err.Error(), "launchctl load") {
		t.Fatalf("expected load failure, got %v", err)
	}
	if _, err := ctl.List(LaunchAgentLabel); err == nil || !strings.Contains(err.Error(), "launchctl list") {
		t.Fatalf("expected list failure, got %v", err)
	}
}
