package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
)

const starterConfigTemplate = `interval = "60s"
timeout = "10s"

[[check]]
name = "self-check"
group = "host"
command = "echo ok"

[check.expect]
equals = "ok"
`

func newInitCommand() *cobra.Command {
	var configPath string
	var force bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write starter configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPath, err := config.ResolvePath(configPath)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(resolvedPath), 0o755); err != nil {
				return fmt.Errorf("create config directory: %w", err)
			}

			if err := writeStarterConfig(resolvedPath, force); err != nil {
				if errors.Is(err, os.ErrExist) {
					return fmt.Errorf("config already exists at %s (use --force to overwrite)", resolvedPath)
				}
				return err
			}

			cmd.Printf("wrote starter config: %s\n", resolvedPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config file")
	return cmd
}

func writeStarterConfig(path string, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}

	file, err := os.OpenFile(path, flags, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.WriteString(starterConfigTemplate); err != nil {
		return fmt.Errorf("write starter config: %w", err)
	}

	return nil
}
