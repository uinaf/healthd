package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePathPrecedence(t *testing.T) {
	t.Setenv(EnvConfigPath, "/tmp/from-env.toml")

	path, err := ResolvePath("/tmp/from-flag.toml")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if path != "/tmp/from-flag.toml" {
		t.Fatalf("expected flag path, got %q", path)
	}

	path, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if path != "/tmp/from-env.toml" {
		t.Fatalf("expected env path, got %q", path)
	}

	t.Setenv(EnvConfigPath, "")
	path, err = ResolvePath("")
	if err != nil {
		t.Fatalf("ResolvePath() error = %v", err)
	}
	if !strings.HasSuffix(path, filepath.FromSlash(".config/healthd/config.toml")) {
		t.Fatalf("expected default path suffix, got %q", path)
	}
}

func TestLoadFromPathValid(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	path := writeTempConfig(t, `
interval = "30s"
timeout = "5s"

[[check]]
name = "disk"
command = "df -h"
`)

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}

	if cfg.Interval != "30s" || cfg.Timeout != "5s" {
		t.Fatalf("unexpected top-level values: %+v", cfg)
	}
	if len(cfg.Checks) != 1 || cfg.Checks[0].Name != "disk" {
		t.Fatalf("unexpected checks: %+v", cfg.Checks)
	}
}

func TestLoadFromPathUsesDefaults(t *testing.T) {
	path := writeTempConfig(t, `
[[check]]
name = "disk"
command = "df -h"
`)

	cfg, err := LoadFromPath(path)
	if err != nil {
		t.Fatalf("LoadFromPath() error = %v", err)
	}
	if cfg.Interval != "60s" || cfg.Timeout != "10s" {
		t.Fatalf("expected defaults, got interval=%q timeout=%q", cfg.Interval, cfg.Timeout)
	}
}

func TestLoadFromPathRejectsUnknownField(t *testing.T) {
	path := writeTempConfig(t, `
interval = "30s"
timeout = "5s"
extra = true

[[check]]
name = "disk"
command = "df -h"
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "unknown config fields") {
		t.Fatalf("expected unknown fields error, got %v", err)
	}
}

func TestLoadFromPathRejectsUnknownNestedField(t *testing.T) {
	path := writeTempConfig(t, `
interval = "30s"
timeout = "5s"

[[check]]
name = "disk"
command = "df -h"
unknown = true
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected error for unknown nested field")
	}
	if !strings.Contains(err.Error(), "unknown config fields") {
		t.Fatalf("expected unknown fields error, got %v", err)
	}
}

func TestLoadFromPathRejectsInvalidConfig(t *testing.T) {
	path := writeTempConfig(t, `
interval = "not-a-duration"
timeout = "5s"

[[check]]
name = "disk"
command = "df -h"
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
	if !strings.Contains(err.Error(), "invalid config") {
		t.Fatalf("expected invalid config error, got %v", err)
	}
}

func TestLoadFromPathRejectsInvalidExpectRegex(t *testing.T) {
	path := writeTempConfig(t, `
[[check]]
name = "disk"
command = "df -h"

[check.expect]
regex = "("
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "regex is invalid") {
		t.Fatalf("expected regex validation error, got %v", err)
	}
}

func TestLoadFromPathRejectsInvalidExpectRange(t *testing.T) {
	path := writeTempConfig(t, `
[[check]]
name = "disk"
command = "printf 42"

[check.expect]
min = 10
max = 5
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "min must be less than or equal to max") {
		t.Fatalf("expected min/max validation error, got %v", err)
	}
}

func TestLoadFromPathRejectsInvalidEnvKey(t *testing.T) {
	path := writeTempConfig(t, `
[[check]]
name = "disk"
command = "df -h"

[check.env]
"BAD=KEY" = "1"
`)

	_, err := LoadFromPath(path)
	if err == nil {
		t.Fatal("expected invalid config error")
	}
	if !strings.Contains(err.Error(), "must not contain '='") {
		t.Fatalf("expected env key validation error, got %v", err)
	}
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
