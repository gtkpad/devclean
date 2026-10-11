package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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

			root, err := loadOrCreateConfigNode(configFile)
			if err != nil {
				return err
			}
			mapNode := getMappingNode(root)
			if mapNode == nil {
				return fmt.Errorf("invalid config root structure")
			}

			w := cmd.OutOrStdout()

			if pattern != "" {
				if _, err := filepath.Match(pattern, ""); err != nil {
					return fmt.Errorf("invalid glob pattern %q: %w", pattern, err)
				}
				seq := findOrCreateSequence(mapNode, "protected_patterns")
				if appendUnique(seq, pattern) {
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
				seq := findOrCreateSequence(mapNode, "exclude_paths")
				if appendUnique(seq, normPath) {
					fmt.Fprintf(w, "Added path %q to exclude_paths\n", normPath)
				} else {
					fmt.Fprintf(w, "Path %q is already in exclude_paths\n", normPath)
				}
			}

			return saveConfigNode(configFile, root)
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

			root, err := loadOrCreateConfigNode(configFile)
			if err != nil {
				return err
			}
			mapNode := getMappingNode(root)
			if mapNode == nil {
				return fmt.Errorf("invalid config root structure")
			}

			w := cmd.OutOrStdout()
			totalRemoved := 0

			if pattern != "" {
				seq := findSequence(mapNode, "protected_patterns")
				if removed := removeFromSequence(seq, pattern); removed > 0 {
					totalRemoved += removed
					fmt.Fprintf(w, "Removed pattern %q from protected_patterns\n", pattern)
				}
			}

			if len(args) > 0 {
				rawTarget := args[0]
				normTarget, _ := normalizePath(rawTarget)

				excludeSeq := findSequence(mapNode, "exclude_paths")
				if removed := removeFromSequence(excludeSeq, normTarget, rawTarget); removed > 0 {
					totalRemoved += removed
					fmt.Fprintf(w, "Removed path %q from exclude_paths\n", normTarget)
				}

				if pattern == "" {
					protectSeq := findSequence(mapNode, "protected_patterns")
					if removed := removeFromSequence(protectSeq, rawTarget); removed > 0 {
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

			return saveConfigNode(configFile, root)
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

func loadOrCreateConfigNode(configFile string) (*yaml.Node, error) {
	var root yaml.Node
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			defaults := config.Defaults()
			defaultBytes, err := yaml.Marshal(&defaults)
			if err != nil {
				return nil, fmt.Errorf("marshaling defaults: %w", err)
			}
			if err := yaml.Unmarshal(defaultBytes, &root); err != nil {
				return nil, fmt.Errorf("unmarshaling defaults: %w", err)
			}
			return &root, nil
		}
		return nil, fmt.Errorf("reading config file %s: %w", configFile, err)
	}

	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing config yaml: %w", err)
	}

	if len(root.Content) == 0 {
		root = yaml.Node{
			Kind: yaml.DocumentNode,
			Content: []*yaml.Node{
				{
					Kind: yaml.MappingNode,
					Tag:  "!!map",
				},
			},
		}
	}
	return &root, nil
}

func getMappingNode(root *yaml.Node) *yaml.Node {
	if root.Kind == yaml.DocumentNode {
		if len(root.Content) == 0 {
			mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			root.Content = []*yaml.Node{mapNode}
			return mapNode
		}
		return root.Content[0]
	}
	if root.Kind == yaml.MappingNode {
		return root
	}
	return nil
}

func findSequence(mapNode *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			val := mapNode.Content[i+1]
			if val.Kind == yaml.SequenceNode {
				return val
			}
			return nil
		}
	}
	return nil
}

func findOrCreateSequence(mapNode *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		if k.Value == key {
			val := mapNode.Content[i+1]
			if val.Kind != yaml.SequenceNode {
				val.Kind = yaml.SequenceNode
				val.Tag = "!!seq"
				val.Content = nil
			}
			val.Style = 0
			return val
		}
	}
	keyNode := &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: key,
	}
	seqNode := &yaml.Node{
		Kind:  yaml.SequenceNode,
		Tag:   "!!seq",
		Style: 0,
	}
	mapNode.Content = append(mapNode.Content, keyNode, seqNode)
	return seqNode
}

func appendUnique(seqNode *yaml.Node, val string) bool {
	for _, item := range seqNode.Content {
		if item.Value == val {
			return false
		}
	}
	seqNode.Content = append(seqNode.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Value: val,
	})
	return true
}

func removeFromSequence(seqNode *yaml.Node, targets ...string) int {
	if seqNode == nil {
		return 0
	}
	targetSet := make(map[string]struct{}, len(targets))
	for _, t := range targets {
		if t != "" {
			targetSet[t] = struct{}{}
		}
	}

	removed := 0
	newContent := make([]*yaml.Node, 0, len(seqNode.Content))
	for _, item := range seqNode.Content {
		if _, match := targetSet[item.Value]; match {
			removed++
		} else {
			newContent = append(newContent, item)
		}
	}
	seqNode.Content = newContent
	return removed
}

func saveConfigNode(configFile string, root *yaml.Node) error {
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %s: %w", dir, err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(root); err != nil {
		return fmt.Errorf("encoding yaml: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing yaml encoder: %w", err)
	}

	if err := os.WriteFile(configFile, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}
