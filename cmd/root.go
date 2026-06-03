package cmd

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/tui"
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
		eng.Register(dotnet.New(paths, cfg.ScanPaths))
		eng.Register(python.New(paths.PythonUVCache, cfg.ScanPaths))
		eng.Register(rust.New(cfg.ScanPaths))
		eng.Register(java.New(cfg.ScanPaths))

		bl := safety.NewBlacklist(cfg.ProtectedPatterns)

		m := tui.New(eng, bl, cfg)
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
	rootCmd.AddCommand(newTrashCmd())
	rootCmd.AddCommand(newScheduleCmd())
	rootCmd.AddCommand(newUnscheduleCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newMCPCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.Version = version
}

func Execute() error {
	return rootCmd.Execute()
}
