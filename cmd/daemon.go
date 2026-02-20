package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
	"github.com/uinaf/healthd/internal/daemon"
)

var daemonRunLoop = daemon.RunLoop

type daemonController interface {
	Install(configPath string, interval time.Duration) (daemon.Paths, error)
	Uninstall() (daemon.Paths, error)
	Status() (daemon.Status, daemon.Paths, error)
	ReadLogs(lines int) (string, string, error)
}

var daemonControllerFactory = func() daemonController {
	return daemon.NewManager()
}

func newDaemonCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage healthd daemon and LaunchAgent",
	}

	cmd.AddCommand(newDaemonInstallCommand())
	cmd.AddCommand(newDaemonUninstallCommand())
	cmd.AddCommand(newDaemonStatusCommand())
	cmd.AddCommand(newDaemonLogsCommand())
	cmd.AddCommand(newDaemonRunCommand())

	return cmd
}

func newDaemonInstallCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install and start macOS LaunchAgent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPath, cfg, err := loadConfigForDaemon(configPath)
			if err != nil {
				return err
			}

			interval, err := parseInterval(cfg.Interval)
			if err != nil {
				return err
			}

			paths, err := daemonControllerFactory().Install(resolvedPath, interval)
			if err != nil {
				return err
			}

			cmd.Printf("daemon installed and started\n")
			cmd.Printf("plist: %s\n", paths.PlistPath)
			cmd.Printf("stdout: %s\n", paths.StdoutPath)
			cmd.Printf("stderr: %s\n", paths.StderrPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	return cmd
}

func newDaemonUninstallCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and uninstall macOS LaunchAgent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := daemonControllerFactory().Uninstall()
			if err != nil {
				return err
			}
			cmd.Printf("daemon uninstalled\n")
			cmd.Printf("removed plist: %s\n", paths.PlistPath)
			return nil
		},
	}
	return cmd
}

func newDaemonStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show LaunchAgent daemon status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			status, paths, err := daemonControllerFactory().Status()
			if err != nil {
				return err
			}

			if !status.Installed {
				cmd.Printf("status: stopped (not installed)\n")
				cmd.Printf("plist: %s\n", paths.PlistPath)
				return nil
			}

			if status.Running {
				cmd.Printf("status: running (pid %d)\n", status.PID)
			} else {
				cmd.Printf("status: stopped\n")
			}
			cmd.Printf("plist: %s\n", paths.PlistPath)
			cmd.Printf("stdout: %s\n", paths.StdoutPath)
			cmd.Printf("stderr: %s\n", paths.StderrPath)
			return nil
		},
	}

	return cmd
}

func newDaemonLogsCommand() *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Print daemon stdout/stderr logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			stdout, stderr, err := daemonControllerFactory().ReadLogs(lines)
			if err != nil {
				return err
			}

			cmd.Println("== stdout ==")
			if strings.TrimSpace(stdout) == "" {
				cmd.Println("(empty)")
			} else {
				cmd.Println(stdout)
			}

			cmd.Println("== stderr ==")
			if strings.TrimSpace(stderr) == "" {
				cmd.Println("(empty)")
			} else {
				cmd.Println(stderr)
			}

			return nil
		},
	}

	cmd.Flags().IntVar(&lines, "lines", 50, "number of lines to print from each log")
	return cmd
}

func newDaemonRunCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:    "run",
		Short:  "Run daemon loop (for LaunchAgent)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cfg, err := loadConfigForDaemon(configPath)
			if err != nil {
				return err
			}
			return daemonRunLoop(context.Background(), cfg, cmd.ErrOrStderr())
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

func parseInterval(raw string) (time.Duration, error) {
	interval, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse schedule interval: %w", err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("schedule interval must be greater than zero")
	}
	return interval, nil
}
