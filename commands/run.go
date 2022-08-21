package commands

import (
	"github.com/Ivlyth/process-bandwidth/config"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newRunCmd())
}

func newRunCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "run",
		Aliases: []string{"r", "t"},
		Short:   "for long time running, usually managed by process manager",
		RunE: func(cmd *cobra.Command, args []string) error {
			if config.GlobalConfig.WebServerPort == 0 {
				return errors.New("web server port must be provide in `run` mode")
			}
			return <-errChan
		},
	}
	return cmd
}
