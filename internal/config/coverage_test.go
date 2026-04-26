package config

import (
	"strings"
	"testing"
)

func TestExpandPathAndValidateNotifyBackendBranches(t *testing.T) {
	t.Parallel()

	if _, err := expandPath(" "); err == nil || !strings.Contains(err.Error(), "path is empty") {
		t.Fatalf("expected empty path error, got %v", err)
	}
	if expanded, err := expandPath("~"); err != nil || expanded == "~" {
		t.Fatalf("expected ~ expansion, got %q err=%v", expanded, err)
	}

	if err := validateNotifyBackend("notify.backend[0]", NotifyBackendConfig{Type: "ntfy", Topic: "   /  "}); err == nil || !strings.Contains(err.Error(), "topic is required") {
		t.Fatalf("expected ntfy topic error, got %v", err)
	}
	if err := validateNotifyBackend("notify.backend[0]", NotifyBackendConfig{Type: "command"}); err == nil || !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("expected command required error, got %v", err)
	}
	if err := validateNotifyBackend("notify.backend[0]", NotifyBackendConfig{Type: "webhook", URL: "https://example.com"}); err != nil {
		t.Fatalf("expected valid webhook backend, got %v", err)
	}
}

func TestValidateNotifyConfigDuplicateTypeFallbackAndCooldown(t *testing.T) {
	t.Parallel()

	err := validateNotifyConfig(NotifyConfig{
		Cooldown: "bad",
		Backends: []NotifyBackendConfig{{Type: "command", Command: "true"}},
	})
	if err == nil || !strings.Contains(err.Error(), "notify.cooldown") {
		t.Fatalf("expected cooldown error, got %v", err)
	}

	err = validateNotifyConfig(NotifyConfig{Backends: []NotifyBackendConfig{
		{Type: "command", Command: "echo one"},
		{Type: "command", Command: "echo two"},
	}})
	if err == nil || !strings.Contains(err.Error(), `name "command" must be unique`) {
		t.Fatalf("expected duplicate fallback name error, got %v", err)
	}
}

func TestConfigValidateMissingChecks(t *testing.T) {
	t.Parallel()

	err := DefaultConfig().Validate()
	if err == nil || !strings.Contains(err.Error(), "at least one [[check]] entry is required") {
		t.Fatalf("expected missing checks error, got %v", err)
	}
}

func TestValidateAlertSafeIdentifierRejectsParserBreakers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		field    string
		value    string
		wantSubs string
	}{
		{name: "open paren in name", field: "check[0].name", value: "weird(name", wantSubs: "must not contain '('"},
		{name: "close paren in group", field: "check[0].group", value: "svc)", wantSubs: "must not contain ')'"},
		{name: "open bracket", field: "check[0].name", value: "x[y", wantSubs: "must not contain '['"},
		{name: "close bracket", field: "check[0].group", value: "x]y", wantSubs: "must not contain ']'"},
		{name: "newline", field: "check[0].name", value: "line\nbreak", wantSubs: "must not contain newlines"},
		{name: "carriage return", field: "check[0].name", value: "line\rbreak", wantSubs: "must not contain newlines"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateAlertSafeIdentifier(tc.field, tc.value)
			if err == nil || !strings.Contains(err.Error(), tc.wantSubs) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSubs, err)
			}
		})
	}

	if err := validateAlertSafeIdentifier("check[0].name", "disk-root"); err != nil {
		t.Fatalf("unexpected error for safe identifier: %v", err)
	}
	if err := validateAlertSafeIdentifier("check[0].group", "host services"); err != nil {
		t.Fatalf("unexpected error for safe identifier with space: %v", err)
	}
}
