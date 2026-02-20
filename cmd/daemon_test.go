package cmd

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/uinaf/healthd/internal/daemon"
)

func TestDaemonInstallCommand(t *testing.T) {
	tmp := t.TempDir()
	configPath := writeTestConfig(t, `
interval = "15s"
timeout = "1s"

[[check]]
name = "ok"
command = "true"
`)

	fake := &fakeDaemonController{
		installPaths: daemon.Paths{
			PlistPath:  filepath.Join(tmp, "com.uinaf.healthd.plist"),
			StdoutPath: filepath.Join(tmp, "stdout.log"),
			StderrPath: filepath.Join(tmp, "stderr.log"),
		},
	}

	original := daemonControllerFactory
	daemonControllerFactory = func() daemonController { return fake }
	t.Cleanup(func() { daemonControllerFactory = original })

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"daemon", "install", "--config", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.installCalls != 1 {
		t.Fatalf("expected install to be called once, got %d", fake.installCalls)
	}
	if !strings.Contains(out.String(), "daemon installed and started") {
		t.Fatalf("expected install output, got %q", out.String())
	}
}

func TestDaemonStatusCommand(t *testing.T) {
	fake := &fakeDaemonController{
		status: daemon.Status{Installed: true, Running: true, PID: 3210},
		statusPaths: daemon.Paths{
			PlistPath:  "/tmp/agent.plist",
			StdoutPath: "/tmp/stdout.log",
			StderrPath: "/tmp/stderr.log",
		},
	}

	original := daemonControllerFactory
	daemonControllerFactory = func() daemonController { return fake }
	t.Cleanup(func() { daemonControllerFactory = original })

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"daemon", "status"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(out.String(), "status: running (pid 3210)") {
		t.Fatalf("expected running output, got %q", out.String())
	}
}

func TestDaemonLogsCommand(t *testing.T) {
	fake := &fakeDaemonController{
		stdout: "line1\nline2",
		stderr: "err1",
	}

	original := daemonControllerFactory
	daemonControllerFactory = func() daemonController { return fake }
	t.Cleanup(func() { daemonControllerFactory = original })

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"daemon", "logs", "--lines", "20"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.logsLines != 20 {
		t.Fatalf("expected --lines to be forwarded, got %d", fake.logsLines)
	}
	if !strings.Contains(out.String(), "== stdout ==") || !strings.Contains(out.String(), "line2") {
		t.Fatalf("expected stdout content, got %q", out.String())
	}
	if !strings.Contains(out.String(), "== stderr ==") || !strings.Contains(out.String(), "err1") {
		t.Fatalf("expected stderr content, got %q", out.String())
	}
}

func TestDaemonUninstallCommand(t *testing.T) {
	fake := &fakeDaemonController{
		uninstallPaths: daemon.Paths{PlistPath: "/tmp/agent.plist"},
	}

	original := daemonControllerFactory
	daemonControllerFactory = func() daemonController { return fake }
	t.Cleanup(func() { daemonControllerFactory = original })

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetArgs([]string{"daemon", "uninstall"})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if fake.uninstallCalls != 1 {
		t.Fatalf("expected uninstall to be called once, got %d", fake.uninstallCalls)
	}
	if !strings.Contains(out.String(), "daemon uninstalled") {
		t.Fatalf("expected uninstall output, got %q", out.String())
	}
}

type fakeDaemonController struct {
	installCalls  int
	installConfig string
	installPaths  daemon.Paths
	installErr    error

	uninstallCalls int
	uninstallPaths daemon.Paths
	uninstallErr   error

	statusCalls int
	status      daemon.Status
	statusPaths daemon.Paths
	statusErr   error

	logsCalls int
	logsLines int
	stdout    string
	stderr    string
	logsErr   error
}

func (f *fakeDaemonController) Install(configPath string) (daemon.Paths, error) {
	f.installCalls++
	f.installConfig = configPath
	if f.installErr != nil {
		return daemon.Paths{}, f.installErr
	}
	return f.installPaths, nil
}

func (f *fakeDaemonController) Uninstall() (daemon.Paths, error) {
	f.uninstallCalls++
	if f.uninstallErr != nil {
		return daemon.Paths{}, f.uninstallErr
	}
	return f.uninstallPaths, nil
}

func (f *fakeDaemonController) Status() (daemon.Status, daemon.Paths, error) {
	f.statusCalls++
	if f.statusErr != nil {
		return daemon.Status{}, daemon.Paths{}, f.statusErr
	}
	return f.status, f.statusPaths, nil
}

func (f *fakeDaemonController) ReadLogs(lines int) (string, string, error) {
	f.logsCalls++
	f.logsLines = lines
	if f.logsErr != nil {
		return "", "", f.logsErr
	}
	return f.stdout, f.stderr, nil
}

var _ daemonController = (*fakeDaemonController)(nil)

func TestDaemonCommandPropagatesErrors(t *testing.T) {
	configPath := writeTestConfig(t, `
interval = "10s"
timeout = "1s"

[[check]]
name = "ok"
command = "true"
`)

	fake := &fakeDaemonController{installErr: errors.New("install failed")}
	original := daemonControllerFactory
	daemonControllerFactory = func() daemonController { return fake }
	t.Cleanup(func() { daemonControllerFactory = original })

	root := NewRootCommand()
	root.SetArgs([]string{"daemon", "install", "--config", configPath})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "install failed") {
		t.Fatalf("expected install error, got %v", err)
	}
}
