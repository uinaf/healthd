package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var builtBinaryPath string

func TestMain(m *testing.M) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	buildDir, err := os.MkdirTemp("", "healthd-e2e-build-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create build dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(buildDir)

	binaryName := "healthd"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	builtBinaryPath = filepath.Join(buildDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", builtBinaryPath, "./")
	buildCmd.Dir = repoRoot
	if output, err := buildCmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build healthd binary: %v\n%s\n", err, string(output))
		os.Exit(1)
	}

	os.Exit(m.Run())
}

type e2eCheckReport struct {
	OK      bool           `json:"ok"`
	Summary e2eSummary     `json:"summary"`
	Checks  []e2eCheckItem `json:"checks"`
	Error   *string        `json:"error"`
}

type e2eSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type e2eCheckItem struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	TimedOut bool   `json:"timed_out"`
	ExitCode int    `json:"exit_code"`
	Reason   string `json:"reason"`
}

type e2eResult struct {
	stdout string
	stderr string
	err    error
}

func findRepoRoot() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolve test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", "..")), nil
}

func runCLI(t *testing.T, workingDir string, args ...string) e2eResult {
	t.Helper()
	cmd := exec.Command(builtBinaryPath, args...)
	cmd.Dir = workingDir
	cmd.Env = append(os.Environ(), "HOME="+workingDir)
	output, err := cmd.CombinedOutput()
	return e2eResult{
		stdout: string(output),
		stderr: "",
		err:    err,
	}
}

func writeConfig(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "healthd.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func parseReport(t *testing.T, raw string) e2eCheckReport {
	t.Helper()
	var report e2eCheckReport
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &report); err != nil {
		t.Fatalf("parse json report: %v\noutput=%s", err, raw)
	}
	return report
}

func TestCLICheckPassAndFailWithBuiltBinary(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	configPath := writeConfig(t, workDir, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok"
command = "true"

[[check]]
name = "bad"
command = "false"
`)

	result := runCLI(t, workDir, "check", "--config", configPath, "--json")
	if result.err == nil {
		t.Fatalf("expected non-zero exit for failing checks")
	}

	report := parseReport(t, result.stdout)
	if report.OK {
		t.Fatalf("expected report ok=false: %+v", report)
	}
	if report.Summary.Total != 2 || report.Summary.Passed != 1 || report.Summary.Failed != 1 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if report.Error == nil || !strings.Contains(*report.Error, "one or more checks failed") {
		t.Fatalf("expected failing error in report, got %+v", report.Error)
	}
}

func TestCLINotifyTestWritesNotifierSideEffect(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	markerPath := filepath.Join(workDir, "notify-marker.txt")
	configPath := writeConfig(t, workDir, fmt.Sprintf(`
interval = "1s"
timeout = "1s"

[[check]]
name = "noop"
command = "true"

[notify]

[[notify.backend]]
name = "local-cmd"
type = "command"
command = "printf \"%%s\" \"$HEALTHD_EVENT_CHECK:$HEALTHD_EVENT_STATE\" > %s"
timeout = "1s"
`, markerPath))

	result := runCLI(t, workDir, "notify", "test", "--config", configPath, "--backend", "local-cmd")
	if result.err != nil {
		t.Fatalf("notify test failed: %v\noutput=%s", result.err, result.stdout)
	}

	marker, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}
	if strings.TrimSpace(string(marker)) != "notify-test:warn" {
		t.Fatalf("unexpected notifier marker content: %q", string(marker))
	}
}

func TestCLITimeoutBehaviorWithBuiltBinary(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	configPath := writeConfig(t, workDir, `
interval = "1s"
timeout = "50ms"

[[check]]
name = "slow"
command = "sleep 1"
`)

	result := runCLI(t, workDir, "check", "--config", configPath, "--json")
	if result.err == nil {
		t.Fatalf("expected non-zero exit for timed out check")
	}

	report := parseReport(t, result.stdout)
	if len(report.Checks) != 1 {
		t.Fatalf("expected one check result, got %d", len(report.Checks))
	}
	item := report.Checks[0]
	if !item.TimedOut {
		t.Fatalf("expected timed_out=true, got %+v", item)
	}
	if item.ExitCode != -1 {
		t.Fatalf("expected timeout exit_code=-1, got %d", item.ExitCode)
	}
	if !strings.Contains(item.Reason, "timed out") {
		t.Fatalf("expected timeout reason, got %q", item.Reason)
	}
}
