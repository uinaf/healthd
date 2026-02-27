package cmd

import (
	"strings"
	"testing"
)

func TestStatusRendersAndFailsOnCheckFailure(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok-check"
group = "services"
command = "true"

[[check]]
name = "bad-check"
group = "services"
command = "false"
`)

	result := executeCheckCommand(t, "status", "--config", configPath)
	if result.err == nil {
		t.Fatalf("expected non-nil error when checks fail")
	}
	if !strings.Contains(result.stdout, "healthd - 2 checks - 1 ok - 1 fail") {
		t.Fatalf("expected status header in output, got: %q", result.stdout)
	}
	if !strings.Contains(result.stdout, "services") {
		t.Fatalf("expected group heading in output, got: %q", result.stdout)
	}
}

func TestStatusAllPassingExitsClean(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok-check"
group = "services"
command = "true"
`)

	result := executeCheckCommand(t, "status", "--config", configPath)
	if result.err != nil {
		t.Fatalf("expected clean exit, got: %v", result.err)
	}
	if !strings.Contains(result.stdout, "1 ok - 0 fail") {
		t.Fatalf("expected all passing in output, got: %q", result.stdout)
	}
}

func TestStatusReturnsErrorWhenFiltersMatchNothing(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "ok-check"
group = "services"
command = "true"
`)

	result := executeCheckCommand(t, "status", "--config", configPath, "--only", "missing")
	if result.err == nil {
		t.Fatal("expected filter miss to error")
	}
	if !strings.Contains(result.err.Error(), "no checks matched filters") {
		t.Fatalf("unexpected error: %v", result.err)
	}
}

func TestStatusGroupFilter(t *testing.T) {
	t.Parallel()

	configPath := writeTestConfig(t, `
interval = "1s"
timeout = "1s"

[[check]]
name = "a"
group = "net"
command = "true"

[[check]]
name = "b"
group = "auth"
command = "true"
`)

	result := executeCheckCommand(t, "status", "--config", configPath, "--group", "net")
	if result.err != nil {
		t.Fatalf("expected clean exit, got: %v", result.err)
	}
	if !strings.Contains(result.stdout, "1 checks") {
		t.Fatalf("expected 1 check after group filter, got: %q", result.stdout)
	}
}
