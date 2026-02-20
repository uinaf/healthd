package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/notify"
)

func newNotifyCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notify",
		Short: "Notification backend operations",
	}

	cmd.AddCommand(newNotifyTestCommand())
	return cmd
}

func newNotifyTestCommand() *cobra.Command {
	var configPath string
	var backends []string

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Send a test event to configured notifier backends",
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

			notifiers, err := notify.BuildNotifiers(cfg.Notify, backends)
			if err != nil {
				return err
			}

			event := notify.Event{
				CheckName: "notify-test",
				Group:     "healthd",
				State:     notify.StateWarn,
				Previous:  notify.StateOK,
				Reason:    "manual notify test",
				ExitCode:  1,
				Timestamp: time.Now().UTC(),
			}

			if err := notify.Dispatch(cmd.Context(), event, notifiers); err != nil {
				return err
			}

			selected := "all"
			if len(backends) > 0 {
				selected = strings.Join(backends, ",")
			}
			cmd.Printf("notify test sent via %s backends from %s\n", selected, resolvedPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	cmd.Flags().StringSliceVar(&backends, "backend", nil, "send test via specific backend names or backend types")
	cmd.PreRunE = func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("backend") && len(backends) == 0 {
			return errors.New("at least one --backend value is required when flag is provided")
		}
		return nil
	}

	return cmd
}
