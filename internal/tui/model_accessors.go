package tui

import "github.com/uinaf/healthd/internal/runner"

func (m Model) Results() []runner.CheckResult {
	results := make([]runner.CheckResult, len(m.results))
	copy(results, m.results)
	return results
}
