package cmd

import (
	"github.com/spf13/cobra"
)

var (
	configPath string
	verbose    bool
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:   "devclean",
	Short: "Automated developer environment cleaner",
	Long:  "DevClean scans and removes build artifacts, caches, and unused dependencies across multiple ecosystems.",
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "alternate config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")

	rootCmd.AddCommand(newScanCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
