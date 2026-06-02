package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gabrielvp/devclean/internal/cleaner/docker"
	"github.com/gabrielvp/devclean/internal/cleaner/dotnet"
	"github.com/gabrielvp/devclean/internal/cleaner/golang"
	"github.com/gabrielvp/devclean/internal/cleaner/node"
	"github.com/gabrielvp/devclean/internal/cleaner/python"
	"github.com/gabrielvp/devclean/internal/config"
	"github.com/gabrielvp/devclean/internal/engine"
	"github.com/gabrielvp/devclean/internal/platform"
	"github.com/gabrielvp/devclean/internal/safety"
	"github.com/gabrielvp/devclean/internal/tui"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		resolver := platform.New()
		paths := resolver.Resolve()
		eng := engine.New(cfg, resolver)

		eng.Register(docker.New())
		eng.Register(golang.New())
		eng.Register(node.New(cfg.ScanPaths))
		eng.Register(dotnet.New(paths))
		eng.Register(python.New(paths.PythonUVCache))

		bl := safety.NewBlacklist(cfg.ProtectedPatterns)

		m := tui.New(eng, bl)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "alternate config file path")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")

	rootCmd.AddCommand(newScanCmd())
	rootCmd.AddCommand(newCleanCmd())
}

func Execute() error {
	return rootCmd.Execute()
}
