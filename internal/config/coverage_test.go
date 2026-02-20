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
