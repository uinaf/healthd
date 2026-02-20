package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkCommandResult struct {
	stdout string
	stderr string
	err    error
}

func executeCheckCommand(t *testing.T, args ...string) checkCommandResult {
	t.Helper()

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs(args)

	err := root.Execute()
	return checkCommandResult{
		stdout: out.String(),
		stderr: errOut.String(),
		err:    err,
	}
}

func TestCheckJSONContractStable(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok-check"
group = "host"
command = "true"

[[check]]
name = "bad-check"
group = "service"
command = "false"
`)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "failed check still returns stable json envelope",
			args: []string{"check", "--config", configPath, "--json"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := executeCheckCommand(t, tc.args...)
			require.Error(t, result.err)
			require.Empty(t, strings.TrimSpace(result.stderr))

			output := strings.TrimSpace(result.stdout)
			require.NotEmpty(t, output)
			assert.NotContains(t, output, "Group:")

			var envelope map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(output), &envelope))

			expectedEnvelopeKeys := map[string]struct{}{
				"ok":        {},
				"timestamp": {},
				"summary":   {},
				"checks":    {},
				"error":     {},
			}
			require.Len(t, envelope, len(expectedEnvelopeKeys))
			for key := range expectedEnvelopeKeys {
				_, ok := envelope[key]
				require.Truef(t, ok, "expected key %q in envelope", key)
			}

			var summary map[string]int
			require.NoError(t, json.Unmarshal(envelope["summary"], &summary))
			require.Equal(t, map[string]int{
				"total":    2,
				"passed":   1,
				"failed":   1,
				"warning":  0,
				"critical": 0,
			}, summary)

			var checks []map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(envelope["checks"], &checks))
			require.Len(t, checks, 2)

			expectedCheckKeys := map[string]struct{}{
				"name":      {},
				"group":     {},
				"ok":        {},
				"reason":    {},
				"exit_code": {},
				"timed_out": {},
				"timestamp": {},
			}
			for i, check := range checks {
				require.Lenf(t, check, len(expectedCheckKeys), "unexpected key count in check[%d]", i)
				for key := range expectedCheckKeys {
					_, ok := check[key]
					require.Truef(t, ok, "expected key %q in check[%d]", key, i)
				}
			}
		})
	}
}

func TestCheckHumanOutputGrouped(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "cpu"
group = "host"
command = "echo ok"

[[check]]
name = "api"
group = "service"
command = "true"
`)

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "grouped human output includes summary counters",
			args: []string{"check", "--config", configPath},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := executeCheckCommand(t, tc.args...)
			require.NoError(t, result.err)
			require.Empty(t, strings.TrimSpace(result.stderr))

			output := result.stdout
			assert.Contains(t, output, "Group: host")
			assert.Contains(t, output, "Group: service")
			assert.Contains(t, output, "- cpu: PASS (ok)")
			assert.Contains(t, output, "Summary: total=2 passed=2 failed=0 warning=0 critical=0")
		})
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "healthd.toml")
	err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600)
	require.NoError(t, err)
	return path
}
