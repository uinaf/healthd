//go:build hoste2e && darwin

package host_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDaemonLifecycleScaffold(t *testing.T) {
	if os.Getenv("HEALTHD_HOST_E2E") != "1" {
		t.Skip("set HEALTHD_HOST_E2E=1 to enable host daemon e2e scaffold")
	}

	binPath := buildHostBinary(t)
	workDir := t.TempDir()
	configPath := writeHostConfig(t, workDir)

	statusResult := runHostCLI(t, binPath, workDir, "daemon", "status")
	if statusResult.err != nil {
		t.Fatalf("daemon status failed: %v\noutput=%s", statusResult.err, statusResult.output)
	}
	if !strings.Contains(statusResult.output, "status:") {
		t.Fatalf("expected daemon status output, got %q", statusResult.output)
	}

	if os.Getenv("HEALTHD_HOST_E2E_ALLOW_INSTALL") != "1" {
		t.Skip("set HEALTHD_HOST_E2E_ALLOW_INSTALL=1 to run install/uninstall lifecycle")
	}

	installResult := runHostCLI(t, binPath, workDir, "daemon", "install", "--config", configPath)
	if installResult.err != nil {
		t.Fatalf("daemon install failed: %v\noutput=%s", installResult.err, installResult.output)
	}

	uninstallResult := runHostCLI(t, binPath, workDir, "daemon", "uninstall")
	if uninstallResult.err != nil {
		t.Fatalf("daemon uninstall failed: %v\noutput=%s", uninstallResult.err, uninstallResult.output)
	}
}

func buildHostBinary(t *testing.T) string {
	t.Helper()

	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))

	buildDir := t.TempDir()
	binaryName := "healthd"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binPath := filepath.Join(buildDir, binaryName)

	cmd := exec.Command("go", "build", "-o", binPath, "./")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build binary: %v\n%s", err, string(output))
	}
	return binPath
}

type hostResult struct {
	output string
	err    error
}

func runHostCLI(t *testing.T, binPath, dir string, args ...string) hostResult {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+dir)
	output, err := cmd.CombinedOutput()
	return hostResult{output: string(output), err: err}
}

func writeHostConfig(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "healthd.toml")
	content := strings.TrimSpace(fmt.Sprintf(`
interval = "1s"
timeout = "1s"

[[check]]
name = "daemon-host-noop"
command = "echo host"
`)) + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write host config: %v", err)
	}
	return path
}
