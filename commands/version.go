package commands

import (
	"fmt"
	"github.com/Ivlyth/process-bandwidth/version"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(newVersionCmd())
}

func newVersionCmd() *cobra.Command {
	var cmd = &cobra.Command{
		Use:     "version",
		Aliases: []string{"v", "V"},
		Short:   "show version info of current process-bandwidth binary",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`Version: %s
Commit: %s
Compile at: %s
Compiled by: %s
`,
				version.VERSION, version.COMMIT,
				version.COMPILE_AT, version.GOVERSION)
		},
	}
	return cmd
}
