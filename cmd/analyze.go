package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/format"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var analyzeIsTerminal = term.IsTerminal
var analyzeRunProgram = func(m tui.AnalyzerModel) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newAnalyzeCmd() *cobra.Command {
	var minSizeStr string
	var depth int
	var all bool
	var formatType string

	cmd := &cobra.Command{
		Use:          "analyze [path]",
		Short:        "Interactive visual disk explorer (ncdu/Mole style)",
		Long:         "Explores disk usage interactively with concurrent size calculation, proportion bars, large files detection, and protected trash deletion.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath := "."
			if len(args) > 0 {
				targetPath = args[0]
			}

			if formatType != "" && formatType != "table" && formatType != "json" {
				return fmt.Errorf("unknown format: %q (supported: table, json)", formatType)
			}

			var minSizeBytes int64
			if minSizeStr != "" {
				var err error
				minSizeBytes, err = format.ParseBytes(minSizeStr)
				if err != nil {
					return fmt.Errorf("invalid --min-size: %w", err)
				}
			}

			cfg, err := loadConfigWithProfile()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)
			scanner := analyzer.NewScanner(bl, all, minSizeBytes, depth)

			isTTY := analyzeIsTerminal(int(os.Stdin.Fd()))

			// Non-interactive fallback (D-13)
			if !isTTY || formatType != "" {
				rootNode, err := scanner.ScanImmediate(targetPath)
				if err != nil {
					return err
				}

				ctx := context.Background()
				var totalBytes int64
				var totalItems int
				for _, child := range rootNode.Children {
					if child.IsDir() {
						_, _, _ = scanner.CalculateDirSize(ctx, child, nil)
					}
					totalBytes += child.SizeBytes
					totalItems += child.ItemCount
				}
				rootNode.SizeBytes = totalBytes
				rootNode.ItemCount = totalItems
				rootNode.Scanned = true

				if minSizeBytes > 0 {
					var filtered []*analyzer.Node
					for _, child := range rootNode.Children {
						if child.SizeBytes >= minSizeBytes {
							filtered = append(filtered, child)
						}
					}
					rootNode.Children = filtered
				}

				sort.SliceStable(rootNode.Children, func(i, j int) bool {
					return rootNode.Children[i].SizeBytes > rootNode.Children[j].SizeBytes
				})

				if formatType == "json" {
					return outputAnalyzeJSON(cmd.OutOrStdout(), rootNode)
				}
				return outputAnalyzeTable(cmd.OutOrStdout(), rootNode)
			}

			// Interactive TUI Launch (D-01, D-05)
			rootNode, err := scanner.ScanImmediate(targetPath)
			if err != nil {
				return err
			}

			resolver := platform.New()
			paths := resolver.Resolve()
			dataDir := getDataDir()
			trashMgr := trash.New(cfg.Safety)
			hookRunner := hooks.New(cfg.Hooks)
			coord := deleter.New(nil, cfg, bl, trashMgr, hookRunner, paths.ConfigDir, dataDir)

			m := tui.NewAnalyzerModel(rootNode, bl, coord, 100, 30, scanner)
			return analyzeRunProgram(m)
		},
	}

	cmd.Flags().StringVar(&minSizeStr, "min-size", "", "ignore items smaller than threshold (e.g. 10M, 500KB)")
	cmd.Flags().IntVar(&depth, "depth", 0, "initial recursion depth (0 = unlimited)")
	cmd.Flags().BoolVarP(&all, "all", "a", false, "include hidden files and directories")
	cmd.Flags().StringVar(&formatType, "format", "", "output format for non-interactive use (table, json)")

	return cmd
}

func outputAnalyzeTable(w io.Writer, root *analyzer.Node) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "DevClean Analyze: %s (Total: %s, %d itens)\n\n", root.Path, format.HumanSize(root.SizeBytes), root.ItemCount)
	fmt.Fprintln(tw, "TAMANHO\tITENS\tNOME\tPROPORÇÃO")

	var maxBytes int64
	for _, c := range root.Children {
		if c.SizeBytes > maxBytes {
			maxBytes = c.SizeBytes
		}
	}

	for _, c := range root.Children {
		name := c.Name
		if c.IsDir() {
			name += "/"
		}
		bar := tui.RenderProportionBar(c.SizeBytes, maxBytes, 10)
		fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", format.HumanSize(c.SizeBytes), c.ItemCount, name, bar)
	}

	return tw.Flush()
}

type analyzeJSONNode struct {
	Path        string             `json:"path"`
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	SizeBytes   int64              `json:"size_bytes"`
	ItemCount   int                `json:"item_count"`
	IsProtected bool               `json:"is_protected"`
	IsLargeFile bool               `json:"is_large_file"`
	Children    []*analyzeJSONNode `json:"children,omitempty"`
}

func nodeToJSON(n *analyzer.Node) *analyzeJSONNode {
	if n == nil {
		return nil
	}
	typeStr := "dir"
	if !n.IsDir() {
		typeStr = "file"
	}
	if n.IsSymlink() {
		typeStr = "symlink"
	}

	jn := &analyzeJSONNode{
		Path:        n.Path,
		Name:        n.Name,
		Type:        typeStr,
		SizeBytes:   n.SizeBytes,
		ItemCount:   n.ItemCount,
		IsProtected: n.IsProtected,
		IsLargeFile: n.IsLargeFile,
	}

	for _, c := range n.Children {
		jn.Children = append(jn.Children, nodeToJSON(c))
	}

	return jn
}

func outputAnalyzeJSON(w io.Writer, root *analyzer.Node) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(nodeToJSON(root))
}
