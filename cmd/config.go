package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	configIsTerminal = term.IsTerminal
	configRunProgram = func(m tui.ConfigModel) error {
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err := p.Run()
		return err
	}
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "config",
		Short:        "Interactive TUI configuration manager",
		Long:         "Opens a full-screen interactive Bubble Tea interface to view and modify all DevClean settings, ecosystems, safety parameters, watch daemon, and scheduler.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !configIsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("interactive config requires a terminal TTY")
			}

			configFile, err := getConfigFile()
			if err != nil {
				return err
			}

			// Diagnostic early check for YAML corruption (D-12)
			node, cfg, err := config.LoadWithAST(configFile)
			if err != nil {
				return err
			}

			resolver := platform.New()
			paths := resolver.Resolve()

			sched := scheduler.New(paths.ConfigDir)
			bl := safety.NewBlacklist(cfg.ProtectedPatterns)

			m := tui.NewConfigModel(cfg, node, configFile, sched, bl)
			return configRunProgram(m)
		},
	}
	return cmd
}
