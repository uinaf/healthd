package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uinaf/healthd/cmd"
)

type commandResult struct {
	stdout string
	stderr string
	err    error
}

type checkJSONReport struct {
	OK      bool             `json:"ok"`
	Summary checkJSONSummary `json:"summary"`
	Checks  []checkJSONItem  `json:"checks"`
	Error   *string          `json:"error"`
}

type checkJSONSummary struct {
	Total  int `json:"total"`
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type checkJSONItem struct {
	Name     string `json:"name"`
	Group    string `json:"group"`
	OK       bool   `json:"ok"`
	TimedOut bool   `json:"timed_out"`
}

func runRootCommand(t *testing.T, args ...string) commandResult {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer

	root := cmd.NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)

	err := root.Execute()
	return commandResult{
		stdout: out.String(),
		stderr: errOut.String(),
		err:    err,
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "healthd.toml")
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600))
	return path
}

func decodeCheckReport(t *testing.T, raw string) checkJSONReport {
	t.Helper()
	var report checkJSONReport
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(raw)), &report))
	return report
}

func TestInitValidateAndCheckJSONIntegration(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "nested", "healthd.toml")

	initResult := runRootCommand(t, "init", "--config", configPath)
	require.NoError(t, initResult.err)
	require.Contains(t, initResult.stdout, "wrote starter config")

	validateResult := runRootCommand(t, "validate", "--config", configPath)
	require.NoError(t, validateResult.err)
	require.Contains(t, validateResult.stdout, "config is valid")

	checkResult := runRootCommand(t, "check", "--config", configPath, "--json")
	require.NoError(t, checkResult.err)
	report := decodeCheckReport(t, checkResult.stdout)
	require.True(t, report.OK)
	require.Equal(t, 1, report.Summary.Total)
	require.Equal(t, 1, report.Summary.Passed)
	require.Equal(t, 0, report.Summary.Failed)
	require.Len(t, report.Checks, 1)
	require.Nil(t, report.Error)
}

func TestValidateAndCheckJSONFailuresIntegration(t *testing.T) {
	t.Parallel()

	invalidConfigPath := writeConfig(t, `
interval = "1s"
timeout = "1s"
unknown_field = "boom"

[[check]]
name = "ok"
command = "true"
`)

	validateResult := runRootCommand(t, "validate", "--config", invalidConfigPath)
	require.Error(t, validateResult.err)
	require.Contains(t, validateResult.err.Error(), "unknown config fields")

	failingConfigPath := writeConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "failing"
group = "host"
command = "false"
`)

	checkResult := runRootCommand(t, "check", "--config", failingConfigPath, "--json")
	require.Error(t, checkResult.err)
	require.Empty(t, strings.TrimSpace(checkResult.stderr))

	report := decodeCheckReport(t, checkResult.stdout)
	require.False(t, report.OK)
	require.Equal(t, 1, report.Summary.Total)
	require.Equal(t, 0, report.Summary.Passed)
	require.Equal(t, 1, report.Summary.Failed)
	require.NotNil(t, report.Error)
	require.Contains(t, *report.Error, "one or more checks failed")
}

func TestCheckFilteringWithOnlyAndGroupIntegration(t *testing.T) {
	t.Parallel()

	configPath := writeConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "cpu"
group = "host"
command = "true"

[[check]]
name = "disk"
group = "host"
command = "true"

[[check]]
name = "api"
group = "service"
command = "true"
`)

	onlyResult := runRootCommand(t, "check", "--config", configPath, "--json", "--only", "cpu")
	require.NoError(t, onlyResult.err)
	onlyReport := decodeCheckReport(t, onlyResult.stdout)
	require.Equal(t, 1, onlyReport.Summary.Total)
	require.Len(t, onlyReport.Checks, 1)
	require.Equal(t, "cpu", onlyReport.Checks[0].Name)

	groupResult := runRootCommand(t, "check", "--config", configPath, "--json", "--group", "host")
	require.NoError(t, groupResult.err)
	groupReport := decodeCheckReport(t, groupResult.stdout)
	require.Equal(t, 2, groupReport.Summary.Total)
	require.Len(t, groupReport.Checks, 2)

	noneResult := runRootCommand(t, "check", "--config", configPath, "--json", "--only", "missing")
	require.Error(t, noneResult.err)
	noneReport := decodeCheckReport(t, noneResult.stdout)
	require.False(t, noneReport.OK)
	require.NotNil(t, noneReport.Error)
	require.Contains(t, *noneReport.Error, "no checks matched filters")
}
