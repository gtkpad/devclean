package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/tui"
	"github.com/gtkpad/devclean/internal/watch"
	"github.com/spf13/cobra"
)

var (
	configPath  string
	profileName string
	verbose     bool
	quiet       bool
	watchDaemon bool
)

var rootCmd = &cobra.Command{
	Use:   "devclean",
	Short: "Automated developer environment cleaner",
	Long:  "DevClean scans and removes build artifacts, caches, and unused dependencies across multiple ecosystems.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !watchDaemon {
			return nil
		}
		// --watch-daemon: run persistent watch loop then exit
		cfg, err := loadConfigWithProfile()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		interval, err := time.ParseDuration(cfg.Watch.Interval)
		if err != nil {
			interval = 6 * time.Hour
		}

		resolver := platform.New()
		eng := engine.New(cfg, resolver)

		engine.RegisterDefaultCleaners(eng, cfg, resolver)

		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
		defer stop()

		w := &watch.Watcher{
			ScanFn:         eng.Scan,
			Interval:       interval,
			ThresholdBytes: cfg.Watch.ThresholdMB * 1024 * 1024,
			Notifier:       watch.NewBeeepNotifier(),
			Logger:         slog.Default(),
		}

		_ = w.Run(ctx)
		os.Exit(0)
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfigWithProfile()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		resolver := platform.New()
		eng := engine.New(cfg, resolver)

		engine.RegisterDefaultCleaners(eng, cfg, resolver)

		bl := safety.NewBlacklist(cfg.ProtectedPatterns)

		m := tui.New(eng, bl, cfg)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err = p.Run()
		return err
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "alternate config file path")
	rootCmd.PersistentFlags().StringVar(&profileName, "profile", "", "apply named config profile")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&quiet, "quiet", false, "suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&watchDaemon, "watch-daemon", false, "run watch daemon loop (internal)")
	_ = rootCmd.PersistentFlags().MarkHidden("watch-daemon")

	rootCmd.AddCommand(newScanCmd())
	rootCmd.AddCommand(newAnalyzeCmd())
	rootCmd.AddCommand(newCleanCmd())
	rootCmd.AddCommand(newPurgeCmd())
	rootCmd.AddCommand(newConfigCmd())
	rootCmd.AddCommand(newInstallerCmd())
	rootCmd.AddCommand(newTrashCmd())
	rootCmd.AddCommand(newWhitelistCmd())
	rootCmd.AddCommand(newScheduleCmd())
	rootCmd.AddCommand(newUnscheduleCmd())
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(newMCPCmd())
	rootCmd.AddCommand(newInitCmd())
	rootCmd.AddCommand(newWhyCmd())
	rootCmd.AddCommand(newHistoryCmd())
	rootCmd.AddCommand(newStatsCmd())
	rootCmd.AddCommand(newWatchCmd())
	rootCmd.Version = version
}

func Execute() error {
	return rootCmd.Execute()
}

// loadConfigWithProfile loads config and applies --profile if set.
func loadConfigWithProfile() (*config.Config, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}
	if profileName != "" {
		resolver := platform.New()
		paths := resolver.Resolve()
		cfgDir := paths.ConfigDir
		if configPath != "" {
			profileCandidate := filepath.Join(filepath.Dir(configPath), "profiles", profileName+".yaml")
			if _, err := os.Stat(profileCandidate); err == nil {
				cfgDir = filepath.Dir(configPath)
			}
		}
		profile, err := config.LoadProfile(cfgDir, profileName)
		if err != nil {
			return nil, fmt.Errorf("loading profile %q: %w", profileName, err)
		}
		cfg = config.MergeProfile(cfg, profile)
	}
	return cfg, nil
}
