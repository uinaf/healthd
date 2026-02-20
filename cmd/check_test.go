package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckJSONContractStable(t *testing.T) {
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

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"check", "--config", configPath, "--json"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected non-nil error when a check fails")
	}

	if strings.TrimSpace(errOut.String()) != "" {
		t.Fatalf("expected no stderr output in json mode, got %q", errOut.String())
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		t.Fatal("expected json output")
	}
	if strings.Contains(output, "Group:") {
		t.Fatalf("expected no human output in json mode, got %q", output)
	}

	var envelope map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal([]byte(output), &envelope); unmarshalErr != nil {
		t.Fatalf("expected valid json output, got error: %v", unmarshalErr)
	}

	expectedEnvelopeKeys := map[string]struct{}{
		"ok":        {},
		"timestamp": {},
		"summary":   {},
		"checks":    {},
		"error":     {},
	}

	if len(envelope) != len(expectedEnvelopeKeys) {
		t.Fatalf("expected exactly %d envelope keys, got %d", len(expectedEnvelopeKeys), len(envelope))
	}
	for key := range expectedEnvelopeKeys {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("expected key %q in envelope", key)
		}
	}

	var checks []map[string]json.RawMessage
	if unmarshalErr := json.Unmarshal(envelope["checks"], &checks); unmarshalErr != nil {
		t.Fatalf("expected checks array, got error: %v", unmarshalErr)
	}
	if len(checks) != 2 {
		t.Fatalf("expected 2 checks, got %d", len(checks))
	}

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
		if len(check) != len(expectedCheckKeys) {
			t.Fatalf("expected check[%d] to have %d keys, got %d", i, len(expectedCheckKeys), len(check))
		}
		for key := range expectedCheckKeys {
			if _, ok := check[key]; !ok {
				t.Fatalf("expected key %q in check[%d]", key, i)
			}
		}
	}
}

func TestCheckHumanOutputGrouped(t *testing.T) {
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

	var out bytes.Buffer
	var errOut bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&errOut)
	root.SetArgs([]string{"check", "--config", configPath})

	if err := root.Execute(); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "Group: host") {
		t.Fatalf("expected host group header, got %q", output)
	}
	if !strings.Contains(output, "Group: service") {
		t.Fatalf("expected service group header, got %q", output)
	}
	if !strings.Contains(output, "Summary: 2/2 checks passed") {
		t.Fatalf("expected summary line, got %q", output)
	}
	if !strings.Contains(output, "- cpu: PASS (ok)") {
		t.Fatalf("expected check line with pass status, got %q", output)
	}
	if strings.TrimSpace(errOut.String()) != "" {
		t.Fatalf("expected empty stderr, got %q", errOut.String())
	}
}

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "healthd.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}
