package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/intelligence"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	var ecosystem string
	var format string
	var all bool
	var smart bool

	cmd := &cobra.Command{
		Use:          "scan",
		Short:        "Scan for cleanable targets across ecosystems",
		Long:         "Discovers build artifacts, caches, and unused dependencies that can be safely removed.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigWithProfile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if all {
				zeroAllMaxAgeDays(cfg)
			}

			resolver := platform.New()
			eng := engine.New(cfg, resolver)

			engine.RegisterDefaultCleaners(eng, cfg, resolver)

			if ecosystem != "" {
				if err := validateEcosystem(ecosystem, eng.Ecosystems()); err != nil {
					return err
				}
			}

			ctx := context.Background()
			ch := eng.Scan(ctx)
			results := engine.Collect(ctx, ch)

			if ecosystem != "" {
				results = filterByEcosystem(results, ecosystem)
			}

			if len(results) == 0 || totalTargets(results) == 0 {
				return errors.New("no targets found")
			}

			if smart {
				scorer := intelligence.NewScorer(cfg.Intelligence)
				for i, r := range results {
					for j := range r.Targets {
						score, err := scorer.Score(ctx, r.Targets[j])
						if err != nil {
							continue
						}
						results[i].Targets[j].SmartScore = score.Total
					}
				}
			}

			return formatOutput(cmd.OutOrStdout(), results, format, verbose, smart)
		},
	}

	cmd.Flags().StringVarP(&ecosystem, "ecosystem", "e", "", "filter by ecosystem name")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table or json")
	cmd.Flags().BoolVar(&all, "all", false, "show all targets regardless of age (bypass TTL filter)")
	cmd.Flags().BoolVar(&smart, "smart", false, "show heuristic score (0.0-1.0) per target")

	return cmd
}

func filterByEcosystem(results []domain.ScanResult, eco string) []domain.ScanResult {
	var filtered []domain.ScanResult
	for _, r := range results {
		if strings.EqualFold(r.Ecosystem, eco) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func totalTargets(results []domain.ScanResult) int {
	count := 0
	for _, r := range results {
		count += len(r.Targets)
	}
	return count
}

func validateEcosystem(eco string, valid []string) error {
	for _, v := range valid {
		if strings.EqualFold(v, eco) {
			return nil
		}
	}
	return fmt.Errorf("unknown ecosystem %q; valid: %s", eco, strings.Join(valid, ", "))
}

func zeroAllMaxAgeDays(cfg *config.Config) {
	cfg.Ecosystems.Docker.MaxAgeDays = 0
	cfg.Ecosystems.Go.MaxAgeDays = 0
	cfg.Ecosystems.Node.MaxAgeDays = 0
	cfg.Ecosystems.DotNet.MaxAgeDays = 0
	cfg.Ecosystems.Python.MaxAgeDays = 0
	cfg.Ecosystems.Rust.MaxAgeDays = 0
	cfg.Ecosystems.Java.MaxAgeDays = 0
	cfg.Ecosystems.AI.MaxAgeDays = 0
	cfg.Ecosystems.Pnpm.MaxAgeDays = 0
	cfg.Ecosystems.Bun.MaxAgeDays = 0
	cfg.Ecosystems.NextJS.MaxAgeDays = 0
	cfg.Ecosystems.Xcode.MaxAgeDays = 0
	cfg.Ecosystems.Homebrew.MaxAgeDays = 0
	cfg.Ecosystems.CPP.MaxAgeDays = 0
	cfg.Ecosystems.Flutter.MaxAgeDays = 0
	cfg.Ecosystems.Terraform.MaxAgeDays = 0
	cfg.Ecosystems.CocoaPods.MaxAgeDays = 0
	cfg.Ecosystems.Elixir.MaxAgeDays = 0
}
