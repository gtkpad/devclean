package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/spf13/cobra"
)

func newWhitelistCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "whitelist",
		Short:        "Manage excluded paths and protected patterns",
		Long:         "View, add, or remove paths and glob patterns protected from deletion.",
		SilenceUsage: true,
	}
	cmd.AddCommand(newWhitelistListCmd())
	cmd.AddCommand(newWhitelistAddCmd())
	cmd.AddCommand(newWhitelistRemoveCmd())
	return cmd
}

func newWhitelistListCmd() *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List all excluded paths, protected patterns, and immutable blacklist",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfigWithProfile()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()

			fmt.Fprintln(w, "Caminhos Excluídos (exclude_paths):")
			if len(cfg.ExcludePaths) == 0 {
				fmt.Fprintln(w, "  (nenhum)")
			} else {
				for _, p := range cfg.ExcludePaths {
					fmt.Fprintf(w, "  • %s\n", p)
				}
			}
			fmt.Fprintln(w)

			fmt.Fprintln(w, "Padrões Protegidos (protected_patterns):")
			if len(cfg.ProtectedPatterns) == 0 {
				fmt.Fprintln(w, "  (nenhum)")
			} else {
				for _, pat := range cfg.ProtectedPatterns {
					fmt.Fprintf(w, "  • %s\n", pat)
				}
			}
			fmt.Fprintln(w)

			fmt.Fprintln(w, "Blacklist Imutável do Sistema:")
			for _, item := range safety.CoreBlacklist() {
				fmt.Fprintf(w, "  • %s [imutável, somente-leitura]\n", item)
			}

			return nil
		},
	}
}

func newWhitelistAddCmd() *cobra.Command {
	var pattern string

	cmd := &cobra.Command{
		Use:          "add [path]",
		Short:        "Add a path or glob pattern to the whitelist",
		Long:         "Adds a canonical path to exclude_paths or a glob pattern to protected_patterns in config.yaml.",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" && len(args) == 0 {
				return fmt.Errorf("specify a path to exclude or --pattern <glob>")
			}

			configFile, err := getConfigFile()
			if err != nil {
				return err
			}

			root, _, err := config.LoadWithAST(configFile)
			if err != nil {
				return err
			}
			mapNode := config.GetMappingNode(root)
			if mapNode == nil {
				return fmt.Errorf("invalid config root structure")
			}

			w := cmd.OutOrStdout()

			if pattern != "" {
				if _, err := filepath.Match(pattern, ""); err != nil {
					return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
				}
				seq := config.FindOrCreateSequence(mapNode, "protected_patterns")
				if config.AppendUniqueToSequence(seq, pattern) {
					fmt.Fprintf(w, "Added pattern %q to protected_patterns\n", pattern)
				} else {
					fmt.Fprintf(w, "Pattern %q is already in protected_patterns\n", pattern)
				}
			}

			if len(args) > 0 {
				normPath, err := normalizePath(args[0])
				if err != nil {
					return err
				}
				seq := config.FindOrCreateSequence(mapNode, "exclude_paths")
				if config.AppendUniqueToSequence(seq, normPath) {
					fmt.Fprintf(w, "Added path %q to exclude_paths\n", normPath)
				} else {
					fmt.Fprintf(w, "Path %q is already in exclude_paths\n", normPath)
				}
			}

			return config.SaveConfigNode(configFile, root)
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "glob pattern to add to protected_patterns")
	return cmd
}

func newWhitelistRemoveCmd() *cobra.Command {
	var pattern string

	cmd := &cobra.Command{
		Use:          "remove [path]",
		Short:        "Remove a path or glob pattern from the whitelist",
		Long:         "Removes an entry from exclude_paths or protected_patterns in config.yaml. System blacklist items cannot be removed.",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" && len(args) == 0 {
				return fmt.Errorf("specify a path to remove or --pattern <glob>")
			}

			// Inviolability check FIRST (D-07, NFR-03)
			if pattern != "" {
				if err := checkImmutableBlock(pattern); err != nil {
					return err
				}
			}
			if len(args) > 0 {
				if err := checkImmutableBlock(args[0]); err != nil {
					return err
				}
			}

			configFile, err := getConfigFile()
			if err != nil {
				return err
			}

			root, _, err := config.LoadWithAST(configFile)
			if err != nil {
				return err
			}
			mapNode := config.GetMappingNode(root)
			if mapNode == nil {
				return fmt.Errorf("invalid config root structure")
			}

			w := cmd.OutOrStdout()
			totalRemoved := 0

			if pattern != "" {
				seq := config.FindSequence(mapNode, "protected_patterns")
				if removed := config.RemoveFromSequence(seq, pattern); removed > 0 {
					totalRemoved += removed
					fmt.Fprintf(w, "Removed pattern %q from protected_patterns\n", pattern)
				}
			}

			if len(args) > 0 {
				rawTarget := args[0]
				normTarget, _ := normalizePath(rawTarget)

				excludeSeq := config.FindSequence(mapNode, "exclude_paths")
				if removed := config.RemoveFromSequence(excludeSeq, normTarget, rawTarget); removed > 0 {
					totalRemoved += removed
					fmt.Fprintf(w, "Removed path %q from exclude_paths\n", normTarget)
				}

				if pattern == "" {
					protectSeq := config.FindSequence(mapNode, "protected_patterns")
					if removed := config.RemoveFromSequence(protectSeq, rawTarget); removed > 0 {
						totalRemoved += removed
						fmt.Fprintf(w, "Removed pattern %q from protected_patterns\n", rawTarget)
					}
				}
			}

			if totalRemoved == 0 {
				targetName := pattern
				if targetName == "" && len(args) > 0 {
					targetName = args[0]
				}
				fmt.Fprintf(w, "Item %q not found in whitelist\n", targetName)
				return nil
			}

			return config.SaveConfigNode(configFile, root)
		},
	}

	cmd.Flags().StringVar(&pattern, "pattern", "", "glob pattern to remove from protected_patterns")
	return cmd
}

func checkImmutableBlock(target string) error {
	for _, blocked := range safety.CoreBlacklist() {
		if target == blocked || filepath.Base(target) == blocked {
			return fmt.Errorf("cannot remove %q: system blacklist is immutable (NFR-03)", blocked)
		}
	}

	clean := filepath.Clean(target)
	if base := filepath.Base(clean); base != "" {
		for _, blocked := range safety.CoreBlacklist() {
			if clean == blocked || base == blocked {
				return fmt.Errorf("cannot remove %q: system blacklist is immutable (NFR-03)", blocked)
			}
		}
	}

	if norm, err := normalizePath(target); err == nil {
		for _, blocked := range safety.CoreBlacklist() {
			if norm == blocked || filepath.Base(norm) == blocked {
				return fmt.Errorf("cannot remove %q: system blacklist is immutable (NFR-03)", blocked)
			}
		}
	}

	if safety.IsCoreBlacklisted(target) {
		return fmt.Errorf("cannot remove %q: system blacklist is immutable (NFR-03)", filepath.Base(target))
	}

	return nil
}

func getConfigFile() (string, error) {
	if configPath != "" {
		return configPath, nil
	}
	resolver := platform.New()
	paths := resolver.Resolve()
	return filepath.Join(paths.ConfigDir, "config.yaml"), nil
}

func normalizePath(target string) (string, error) {
	if strings.HasPrefix(target, "~/") || target == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolving user home directory: %w", err)
		}
		if target == "~" {
			target = home
		} else {
			target = filepath.Join(home, target[2:])
		}
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	return filepath.Clean(abs), nil
}

