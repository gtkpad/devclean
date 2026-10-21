package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// Version exposes the build version string for package consumers.
var Version = version

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			v := version
			if v == "dev" && Version != "dev" {
				v = Version
			}
			fmt.Fprintf(cmd.OutOrStdout(), "devclean version %s\n  commit: %s\n  built:  %s\n", v, commit, date)
			return nil
		},
	}
}
