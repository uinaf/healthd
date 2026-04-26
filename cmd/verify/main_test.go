package main

import (
	"strings"
	"testing"
)

func TestParsePackageCoverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		output        string
		expected      int
		wantCovered   []packageCoverage
		wantErrSubstr string
	}{
		{
			name:        "single package with statements",
			output:      `ok  	github.com/uinaf/healthd/internal/runner	2.868s	coverage: 86.4% of statements`,
			expected:    1,
			wantCovered: []packageCoverage{{Name: "github.com/uinaf/healthd/internal/runner", Coverage: 86.4}},
		},
		{
			name:        "cached result is matched",
			output:      `ok  	github.com/uinaf/healthd/internal/notify	(cached)	coverage: 91.1% of statements`,
			expected:    1,
			wantCovered: []packageCoverage{{Name: "github.com/uinaf/healthd/internal/notify", Coverage: 91.1}},
		},
		{
			name: "no-statements packages count toward expected",
			output: strings.Join([]string{
				"ok  	github.com/uinaf/healthd/e2e/cli	3.081s	coverage: [no statements]",
				"ok  	github.com/uinaf/healthd/integration	2.051s	coverage: [no statements]",
			}, "\n"),
			expected:    2,
			wantCovered: nil,
		},
		{
			name: "mixed normal + cached + no-statements",
			output: strings.Join([]string{
				"ok  	github.com/uinaf/healthd/cmd	0.450s	coverage: 82.1% of statements",
				"ok  	github.com/uinaf/healthd/e2e/cli	(cached)	coverage: [no statements]",
				"ok  	github.com/uinaf/healthd/internal/loop	1.512s	coverage: 91.9% of statements",
			}, "\n"),
			expected: 3,
			wantCovered: []packageCoverage{
				{Name: "github.com/uinaf/healthd/cmd", Coverage: 82.1},
				{Name: "github.com/uinaf/healthd/internal/loop", Coverage: 91.9},
			},
		},
		{
			name: "partial match (e.g. root pkg has no _test.go) is allowed",
			output: strings.Join([]string{
				"\tgithub.com/uinaf/healthd\t\tcoverage: 0.0% of statements",
				"ok  \tgithub.com/uinaf/healthd/cmd\t0.450s\tcoverage: 82.1% of statements",
			}, "\n"),
			expected:    2,
			wantCovered: []packageCoverage{{Name: "github.com/uinaf/healthd/cmd", Coverage: 82.1}},
		},
		{
			name:          "zero matches when expected non-zero fails",
			output:        "no ok lines here\n--- PASS: TestFoo (0.00s)",
			expected:      2,
			wantErrSubstr: "parsed coverage for 0 of 2 packages",
		},
		{
			name:        "expected=0 short-circuits the sanity check",
			output:      "",
			expected:    0,
			wantCovered: nil,
		},
		{
			name:        "trailing whitespace is tolerated",
			output:      `ok  	github.com/uinaf/healthd/internal/config	1.011s	coverage: 84.3% of statements   `,
			expected:    1,
			wantCovered: []packageCoverage{{Name: "github.com/uinaf/healthd/internal/config", Coverage: 84.3}},
		},
		{
			name: "non-ok lines are ignored",
			output: strings.Join([]string{
				"=== RUN   TestFoo",
				"--- PASS: TestFoo (0.01s)",
				"PASS",
				"ok  	github.com/uinaf/healthd/internal/alertlog	1.044s	coverage: 77.8% of statements",
				"FAIL",
			}, "\n"),
			expected:    1,
			wantCovered: []packageCoverage{{Name: "github.com/uinaf/healthd/internal/alertlog", Coverage: 77.8}},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parsePackageCoverage(tc.output, tc.expected)
			if tc.wantErrSubstr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.wantCovered) {
				t.Fatalf("got %d covered, want %d: %+v", len(got), len(tc.wantCovered), got)
			}
			for i, want := range tc.wantCovered {
				if got[i].Name != want.Name || got[i].Coverage != want.Coverage {
					t.Fatalf("covered[%d] = %+v, want %+v", i, got[i], want)
				}
			}
		})
	}
}
