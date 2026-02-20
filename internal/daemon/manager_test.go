package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderPlistIncludesRequiredFields(t *testing.T) {
	plist, err := RenderPlist(PlistSpec{
		Label:      LaunchAgentLabel,
		Executable: "/usr/local/bin/healthd",
		ConfigPath: "/tmp/healthd.toml",
		StdoutPath: "/tmp/stdout.log",
		StderrPath: "/tmp/stderr.log",
	})
	if err != nil {
		t.Fatalf("RenderPlist() error = %v", err)
	}
	for _, expected := range []string{
		"<string>com.uinaf.healthd</string>",
		"<string>/usr/local/bin/healthd</string>",
		"<string>daemon</string>",
		"<string>run</string>",
		"<string>/tmp/healthd.toml</string>",
		"<true/>",
	} {
		if !strings.Contains(plist, expected) {
			t.Fatalf("expected plist to contain %q", expected)
		}
	}
	if strings.Contains(plist, "<key>StartInterval</key>") {
		t.Fatalf("expected plist to omit StartInterval when KeepAlive is enabled")
	}
}

func TestManagerInstallWritesPlistAndRestartsService(t *testing.T) {
	tmp := t.TempDir()
	fake := &fakeLaunchCtl{}

	manager := NewManager()
	manager.launchctl = fake
	manager.homeDir = func() (string, error) { return tmp, nil }
	manager.execPath = func() (string, error) { return "/usr/local/bin/healthd", nil }

	paths, err := manager.Install("/tmp/healthd.toml")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if fake.unloadCalls != 1 || fake.loadCalls != 1 {
		t.Fatalf("expected unload/load for safe restart, got unload=%d load=%d", fake.unloadCalls, fake.loadCalls)
	}
	if _, err := os.Stat(paths.PlistPath); err != nil {
		t.Fatalf("expected plist to exist, got %v", err)
	}
	content, err := os.ReadFile(paths.PlistPath)
	if err != nil {
		t.Fatalf("failed reading plist: %v", err)
	}
	if strings.Contains(string(content), "<key>StartInterval</key>") {
		t.Fatalf("expected StartInterval to be omitted, got %q", string(content))
	}
}

func TestManagerStatusParsesRunningPID(t *testing.T) {
	tmp := t.TempDir()
	paths := DefaultPaths(tmp)
	if err := os.MkdirAll(filepath.Dir(paths.PlistPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(paths.PlistPath, []byte("plist"), 0o644); err != nil {
		t.Fatalf("write plist: %v", err)
	}

	manager := NewManager()
	manager.homeDir = func() (string, error) { return tmp, nil }
	manager.launchctl = &fakeLaunchCtl{listOutput: `{ "PID" = 4242; }`}

	status, _, err := manager.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.Installed || !status.Running || status.PID != 4242 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestTailLines(t *testing.T) {
	input := []byte("a\nb\nc\nd\n")
	if got := TailLines(input, 2); got != "c\nd" {
		t.Fatalf("expected tail to return last 2 lines, got %q", got)
	}
	if got := TailLines(input, 10); got != "a\nb\nc\nd" {
		t.Fatalf("expected full output when lines exceed content, got %q", got)
	}
}

func TestParsePID(t *testing.T) {
	if pid := ParsePID("{ \"PID\" = 777; }"); pid != 777 {
		t.Fatalf("expected pid 777, got %d", pid)
	}
	if pid := ParsePID("pid = 55"); pid != 55 {
		t.Fatalf("expected pid 55, got %d", pid)
	}
	if pid := ParsePID("no pid"); pid != 0 {
		t.Fatalf("expected pid 0, got %d", pid)
	}
}

type fakeLaunchCtl struct {
	loadCalls   int
	unloadCalls int
	listOutput  string
	listErr     error
}

func (f *fakeLaunchCtl) Load(plistPath string) error {
	f.loadCalls++
	return nil
}

func (f *fakeLaunchCtl) Unload(plistPath string) error {
	f.unloadCalls++
	return errors.New("missing service")
}

func (f *fakeLaunchCtl) List(label string) (string, error) {
	if f.listErr != nil {
		return "", f.listErr
	}
	return f.listOutput, nil
}
