package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/loop"
)

var runLoop = loop.Run

func newRunCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the health-check loop in the foreground",
		Long: "Run the health-check loop in the foreground. Use a process supervisor " +
			"(process-compose, systemd, launchd) to manage start/stop/restart in production.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadConfigForRun(configPath)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runLoop(ctx, cfg, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	return cmd
}

func loadConfigForRun(path string) (string, config.Config, error) {
	resolvedPath, err := config.ResolvePath(path)
	if err != nil {
		return "", config.Config{}, err
	}
	cfg, err := config.LoadFromPath(resolvedPath)
	if err != nil {
		return "", config.Config{}, err
	}
	return resolvedPath, cfg, nil
}
