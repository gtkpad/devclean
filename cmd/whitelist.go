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
