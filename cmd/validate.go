package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/uinaf/healthd/internal/config"
)

func newValidateCommand() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate healthd configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolvedPath, err := config.ResolvePath(configPath)
			if err != nil {
				return err
			}

			if _, err := config.LoadFromPath(resolvedPath); err != nil {
				return err
			}

			cmd.Printf("config is valid: %s\n", resolvedPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", fmt.Sprintf("config file path (default: %s)", config.DefaultConfigPath))
	return cmd
}
