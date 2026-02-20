package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

type checkReport struct {
	OK        bool              `json:"ok"`
	Timestamp string            `json:"timestamp"`
	Summary   string            `json:"summary"`
	Checks    []checkReportItem `json:"checks"`
	Error     *string           `json:"error"`
}

type checkReportItem struct {
	Name      string `json:"name"`
	Group     string `json:"group"`
	OK        bool   `json:"ok"`
	Reason    string `json:"reason"`
	ExitCode  int    `json:"exit_code"`
	TimedOut  bool   `json:"timed_out"`
	Timestamp string `json:"timestamp"`
}

func newCheckCommand() *cobra.Command {
	var configPath string
	var only []string
	var groups []string
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run health checks once",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = jsonOutput

			resolvedPath, err := config.ResolvePath(configPath)
			if err != nil {
				return writeJSONError(cmd, jsonOutput, err)
			}

			cfg, err := config.LoadFromPath(resolvedPath)
			if err != nil {
				return writeJSONError(cmd, jsonOutput, err)
			}

			checks := runner.FilterChecks(cfg.Checks, only, groups)
			if len(checks) == 0 {
				return writeJSONError(cmd, jsonOutput, errors.New("no checks matched filters"))
			}

			results := runner.RunChecks(cmd.Context(), checks, cfg.Timeout)
			if jsonOutput {
				return writeJSONReport(cmd, results)
			}

			writeHumanReport(cmd, results)

			if !runner.AllPassed(results) {
				return errors.New("one or more checks failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	cmd.Flags().StringSliceVar(&only, "only", nil, "only run checks by name (repeat flag or pass comma-separated names)")
	cmd.Flags().StringSliceVar(&groups, "group", nil, "only run checks by group (repeat flag or pass comma-separated groups)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable JSON report")
	return cmd
}

func passFail(passed bool) string {
	if passed {
		return "PASS"
	}
	return "FAIL"
}

func emptyIfBlank(v string) string {
	if strings.TrimSpace(v) == "" {
		return "ungrouped"
	}
	return v
}

func writeHumanReport(cmd *cobra.Command, results []runner.CheckResult) {
	grouped := map[string][]runner.CheckResult{}
	for _, result := range results {
		group := emptyIfBlank(result.Group)
		grouped[group] = append(grouped[group], result)
	}

	groupNames := make([]string, 0, len(grouped))
	for group := range grouped {
		groupNames = append(groupNames, group)
	}
	sort.Strings(groupNames)

	for i, group := range groupNames {
		cmd.Printf("Group: %s\n", group)
		for _, result := range grouped[group] {
			cmd.Printf("  - %s: %s (%s)\n", result.Name, passFail(result.Passed), result.Reason)
			if strings.TrimSpace(result.Stdout) != "" {
				cmd.Printf("    stdout: %s\n", strings.TrimSpace(result.Stdout))
			}
			if strings.TrimSpace(result.Stderr) != "" {
				cmd.Printf("    stderr: %s\n", strings.TrimSpace(result.Stderr))
			}
		}
		if i < len(groupNames)-1 {
			cmd.Println()
		}
	}

	passedCount := 0
	for _, result := range results {
		if result.Passed {
			passedCount++
		}
	}
	cmd.Printf("\nSummary: %d/%d checks passed\n", passedCount, len(results))
}

func writeJSONError(cmd *cobra.Command, jsonOutput bool, err error) error {
	if !jsonOutput {
		return err
	}

	message := err.Error()
	report := checkReport{
		OK:        false,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Summary:   "0/0 checks passed",
		Checks:    []checkReportItem{},
		Error:     &message,
	}

	if encodeErr := json.NewEncoder(cmd.OutOrStdout()).Encode(report); encodeErr != nil {
		return encodeErr
	}
	return err
}

func writeJSONReport(cmd *cobra.Command, results []runner.CheckResult) error {
	passedCount := 0
	items := make([]checkReportItem, 0, len(results))
	for _, result := range results {
		if result.Passed {
			passedCount++
		}

		items = append(items, checkReportItem{
			Name:      result.Name,
			Group:     result.Group,
			OK:        result.Passed,
			Reason:    result.Reason,
			ExitCode:  result.ExitCode,
			TimedOut:  result.TimedOut,
			Timestamp: result.Timestamp.UTC().Format(time.RFC3339Nano),
		})
	}

	report := checkReport{
		OK:        runner.AllPassed(results),
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Summary:   fmt.Sprintf("%d/%d checks passed", passedCount, len(results)),
		Checks:    items,
		Error:     nil,
	}

	if !report.OK {
		message := "one or more checks failed"
		report.Error = &message
	}

	if err := json.NewEncoder(cmd.OutOrStdout()).Encode(report); err != nil {
		return err
	}

	if !report.OK {
		return errors.New("one or more checks failed")
	}
	return nil
}
