package runner

import "testing"

func TestAllPassed(t *testing.T) {
	t.Parallel()

	if !AllPassed([]CheckResult{{Passed: true}, {Passed: true}}) {
		t.Fatal("expected all checks passing")
	}
	if AllPassed([]CheckResult{{Passed: true}, {Passed: false}}) {
		t.Fatal("expected failed aggregate result")
	}
}
