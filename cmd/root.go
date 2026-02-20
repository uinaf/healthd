package cmd

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "healthd",
		Short: "Host health-check daemon",
	}

	root.AddCommand(newCheckCommand())
	root.AddCommand(newNotifyCommand())
	root.AddCommand(newValidateCommand())
	return root
}
