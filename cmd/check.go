package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
)

func newCheckCommand() *cobra.Command {
	var configPath string
	var only []string
	var groups []string

	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run health checks once",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPath, err := config.ResolvePath(configPath)
			if err != nil {
				return err
			}

			cfg, err := config.LoadFromPath(resolvedPath)
			if err != nil {
				return err
			}

			checks := runner.FilterChecks(cfg.Checks, only, groups)
			if len(checks) == 0 {
				return errors.New("no checks matched filters")
			}

			results := runner.RunChecks(cmd.Context(), checks, cfg.Timeout)
			for _, result := range results {
				cmd.Printf("%s [%s]: %s (%s)\n", result.Name, emptyIfBlank(result.Group), passFail(result.Passed), result.Reason)

				if strings.TrimSpace(result.Stdout) != "" {
					cmd.Printf("  stdout: %s\n", strings.TrimSpace(result.Stdout))
				}
				if strings.TrimSpace(result.Stderr) != "" {
					cmd.Printf("  stderr: %s\n", strings.TrimSpace(result.Stderr))
				}
			}

			if !runner.AllPassed(results) {
				return errors.New("one or more checks failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	cmd.Flags().StringSliceVar(&only, "only", nil, "only run checks by name (repeat flag or pass comma-separated names)")
	cmd.Flags().StringSliceVar(&groups, "group", nil, "only run checks by group (repeat flag or pass comma-separated groups)")
	return cmd
}

func passFail(passed bool) string {
	if passed {
		return "pass"
	}
	return "fail"
}

func emptyIfBlank(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
