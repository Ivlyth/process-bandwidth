package commands

import (
	"github.com/Ivlyth/process-bandwidth/top"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newTopCommand())
}

func newTopCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "top",
		Short: "like htop but focus on process's bandwidth. it's the default sub command",
		RunE: func(cmd *cobra.Command, args []string) error {
			return top.StartTop()
		},
	}
	return cmd
}
