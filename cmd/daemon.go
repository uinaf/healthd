package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/daemon"
)

var daemonRunLoop = daemon.RunLoop

func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage healthd daemon",
	}

	cmd.AddCommand(newDaemonRunCommand())

	return cmd
}

func newDaemonRunCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run daemon health-check loop",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadConfigForDaemon(configPath)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return daemonRunLoop(ctx, cfg, cmd.ErrOrStderr())
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	return cmd
}

func loadConfigForDaemon(path string) (string, config.Config, error) {
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
