package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/history"
	"github.com/gtkpad/devclean/internal/hooks"
	devmcp "github.com/gtkpad/devclean/internal/mcp"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// historyAdapter wraps the history package functions to satisfy HistoryReader.
type historyAdapter struct {
	dataDir string
}

func (h *historyAdapter) Read() ([]history.Entry, error) {
	return history.Read(h.dataDir)
}

func (h *historyAdapter) ReadFiltered(opts history.FilterOpts) ([]history.Entry, error) {
	return history.ReadFiltered(h.dataDir, opts)
}

// configAdapter wraps config to satisfy ConfigProvider.
type configAdapter struct {
	cfg    *config.Config
	cfgDir string
}

func (c *configAdapter) Config() *config.Config { return c.cfg }
func (c *configAdapter) ConfigDir() string      { return c.cfgDir }

func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for AI agent integration",
		Long:  "Starts a Model Context Protocol server over stdio, exposing scan, propose, execute, stats, config, explain, restore, and list_trash tools for autonomous agents.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			eng := engine.New(cfg, resolver)

			engine.RegisterDefaultCleaners(eng, cfg, resolver)

			dataDir := getDataDir()

			cfgDir := filepath.Dir(viper.ConfigFileUsed())
			if cfgDir == "." {
				cfgDir = paths.ConfigDir
			}

			ha := &historyAdapter{dataDir: dataDir}
			ca := &configAdapter{cfg: cfg, cfgDir: cfgDir}
			tm := trash.New(cfg.Safety)
			bl := safety.NewBlacklist(cfg.ProtectedPatterns)
			explainer := devmcp.NewDefaultExplainer(cfg, bl)
			hookRunner := hooks.New(cfg.Hooks)
			coord := deleter.New(eng, cfg, bl, tm, hookRunner, cfgDir, dataDir)

			srv := devmcp.New(eng, cfg,
				devmcp.WithHistory(ha),
				devmcp.WithConfigProvider(ca),
				devmcp.WithTrash(tm),
				devmcp.WithExplainer(explainer),
				devmcp.WithCoordinator(coord),
			)
			return srv.ServeStdio()
		},
	}
}
