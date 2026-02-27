package cmd

import (
	"errors"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/runner"
	"github.com/uinaf/healthd/internal/tui"
)

func newStatusCommand() *cobra.Command {
	var configPath string
	var only []string
	var groups []string
	var watch bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Render health check status in a terminal UI",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true

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

			model := tui.NewModel(cfg, checks, watch)

			if watch {
				program := tea.NewProgram(model, tea.WithOutput(cmd.OutOrStdout()), tea.WithInput(cmd.InOrStdin()))
				_, err := program.Run()
				return err
			}

			// Non-watch: run checks and render directly (no TTY needed).
			initCmd := model.Init()
			msg := initCmd()
			updated, _ := model.Update(msg)
			tuiModel := updated.(tui.Model)
			fmt.Fprint(cmd.OutOrStdout(), tuiModel.View())

			if !runner.AllPassed(tuiModel.Results()) {
				return errors.New("one or more checks failed")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	cmd.Flags().StringSliceVar(&only, "only", nil, "only run checks by name (repeat flag or pass comma-separated names)")
	cmd.Flags().StringSliceVar(&groups, "group", nil, "only run checks by group (repeat flag or pass comma-separated groups)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "watch checks on the configured interval")
	return cmd
}
