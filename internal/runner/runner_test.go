package runner

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/uinaf/healthd/internal/config"
)

func TestRunChecksExpectationANDCombinations(t *testing.T) {
	okContains := "health"
	okRegex := `^health:[0-9]+$`
	notValue := "bad"
	minValue := 10.0
	maxValue := 20.0

	checks := []config.CheckConfig{
		{
			Name:    "expect-all-pass",
			Command: `printf "health:12"`,
			Expect: config.ExpectConfig{
				ExitCode:    intPtr(0),
				Contains:    &okContains,
				Not:         &notValue,
				Regex:       &okRegex,
				NotContains: strPtr("panic"),
			},
		},
		{
			Name:    "expect-fail-combo",
			Command: `printf "bad"`,
			Expect: config.ExpectConfig{
				ExitCode: intPtr(0),
				Contains: &okContains,
			},
		},
		{
			Name:    "expect-numeric-range-pass",
			Command: `printf "15"`,
			Expect: config.ExpectConfig{
				Min: &minValue,
				Max: &maxValue,
			},
		},
		{
			Name:    "expect-numeric-range-fail",
			Command: `printf "21"`,
			Expect: config.ExpectConfig{
				Min: &minValue,
				Max: &maxValue,
			},
		},
	}

	results := RunChecks(context.Background(), checks, "1s")
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	if !results[0].Passed {
		t.Fatalf("expected first check to pass, got %+v", results[0])
	}

	if results[1].Passed {
		t.Fatalf("expected second check to fail, got %+v", results[1])
	}
	if !strings.Contains(results[1].Reason, "contain") {
		t.Fatalf("expected contains failure reason, got %q", results[1].Reason)
	}

	if !results[2].Passed {
		t.Fatalf("expected numeric range pass, got %+v", results[2])
	}

	if results[3].Passed {
		t.Fatalf("expected numeric range failure, got %+v", results[3])
	}
	if !strings.Contains(results[3].Reason, "<=") {
		t.Fatalf("expected max failure reason, got %q", results[3].Reason)
	}
}

func TestRunChecksTimeoutBehavior(t *testing.T) {
	check := config.CheckConfig{
		Name:    "timeout-check",
		Command: `sleep 0.2`,
		Timeout: "50ms",
	}

	results := RunChecks(context.Background(), []config.CheckConfig{check}, "1s")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if !result.TimedOut {
		t.Fatalf("expected timeout, got %+v", result)
	}
	if result.Passed {
		t.Fatalf("expected timeout to fail, got %+v", result)
	}
	if result.Reason != "timed out" {
		t.Fatalf("expected timeout reason, got %q", result.Reason)
	}
	if result.ExitCode != -1 {
		t.Fatalf("expected exitCode -1 for timeout, got %d", result.ExitCode)
	}
}

func TestRunChecksUsesEnvOverrides(t *testing.T) {
	check := config.CheckConfig{
		Name:    "env-check",
		Command: `printf "$HEALTHD_TEST_VAR"`,
		Env: map[string]string{
			"HEALTHD_TEST_VAR": "override",
		},
	}

	results := RunChecks(context.Background(), []config.CheckConfig{check}, "1s")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	result := results[0]
	if strings.TrimSpace(result.Stdout) != "override" {
		t.Fatalf("expected env override output, got %q", result.Stdout)
	}
}

func TestFilterChecks(t *testing.T) {
	checks := []config.CheckConfig{
		{Name: "cpu", Group: "host"},
		{Name: "disk", Group: "host"},
		{Name: "api", Group: "service"},
	}

	onlyFiltered := FilterChecks(checks, []string{"cpu,api"}, nil)
	if len(onlyFiltered) != 2 {
		t.Fatalf("expected 2 checks for --only filter, got %d", len(onlyFiltered))
	}

	groupFiltered := FilterChecks(checks, nil, []string{"host"})
	if len(groupFiltered) != 2 {
		t.Fatalf("expected 2 checks for --group filter, got %d", len(groupFiltered))
	}

	combined := FilterChecks(checks, []string{"api"}, []string{"host"})
	if len(combined) != 0 {
		t.Fatalf("expected 0 checks for combined non-matching filters, got %d", len(combined))
	}
}

func TestRunChecksDefaultExitCodeExpectation(t *testing.T) {
	results := RunChecks(context.Background(), []config.CheckConfig{
		{Name: "ok-default", Command: "true"},
		{Name: "fail-default", Command: "false"},
	}, "1s")

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("expected zero exit code to pass by default, got %+v", results[0])
	}
	if results[1].Passed {
		t.Fatalf("expected non-zero exit code to fail by default, got %+v", results[1])
	}
}

func intPtr(v int) *int {
	return &v
}

func strPtr(v string) *string {
	return &v
}

func TestRunChecksPerCheckTimeoutOverride(t *testing.T) {
	check := config.CheckConfig{
		Name:    "override-timeout",
		Command: "sleep 0.1",
		Timeout: "300ms",
	}

	start := time.Now()
	results := RunChecks(context.Background(), []config.CheckConfig{check}, "50ms")
	elapsed := time.Since(start)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TimedOut {
		t.Fatalf("expected per-check timeout override to prevent timeout, got %+v", results[0])
	}
	if elapsed < 80*time.Millisecond {
		t.Fatalf("check finished too quickly, timeout override likely ignored: %v", elapsed)
	}
}
