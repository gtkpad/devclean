package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	var ecosystem string
	var format string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan for cleanable targets across ecosystems",
		Long:  "Discovers build artifacts, caches, and unused dependencies that can be safely removed.",
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
			eng.Register(python.New(paths.PythonUVCache))

			ctx := context.Background()
			ch := eng.Scan(ctx)
			results := engine.Collect(ctx, ch)

			if ecosystem != "" {
				results = filterByEcosystem(results, ecosystem)
			}

			if len(results) == 0 || totalTargets(results) == 0 {
				fmt.Fprintln(os.Stderr, "No targets found")
				os.Exit(2)
			}

			return formatOutput(os.Stdout, results, format, verbose)
		},
	}

	cmd.Flags().StringVarP(&ecosystem, "ecosystem", "e", "", "filter by ecosystem name")
	cmd.Flags().StringVar(&format, "format", "table", "output format: table or json")

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
