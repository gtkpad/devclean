package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/spf13/cobra"
)

// targetPatterns maps directory basenames to their ecosystem.
var targetPatterns = map[string]string{
	"node_modules": "node",
	"target":       "rust",
	"build":        "java",
	"__pycache__":  "python",
	".venv":        "python",
	"bin":          "dotnet",
	"obj":          "dotnet",
}

// manifestForEcosystem maps ecosystem to the manifest file that must be a sibling.
var manifestForEcosystem = map[string][]string{
	"node":   {"package.json"},
	"rust":   {"Cargo.toml"},
	"java":   {"pom.xml", "build.gradle"},
	"python": {"requirements.txt", "pyproject.toml", "setup.py"},
	"dotnet": {"*.csproj", "*.sln"},
}

func newWhyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "why <path>",
		Short: "Explain whether a path is a cleanup target",
		Long:  "Analyzes a path and explains if it would be cleaned, excluded, or protected.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queryPath, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			cfg, err := config.Load(configPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			bl := safety.NewBlacklist(cfg.ProtectedPatterns)

			// Check blacklist
			if bl.IsBlacklisted(queryPath) {
				printStatus(queryPath, "protected", "Protected by core blacklist ("+filepath.Base(queryPath)+")", "", 0)
				return nil
			}

			// Check exclude paths
			for _, pattern := range cfg.ExcludePaths {
				if matched, _ := filepath.Match(pattern, queryPath); matched {
					printStatus(queryPath, "excluded", "Excluded by pattern: "+pattern, "", 0)
					return nil
				}
				if matched, _ := filepath.Match(pattern, filepath.Base(queryPath)); matched {
					printStatus(queryPath, "excluded", "Excluded by pattern: "+pattern, "", 0)
					return nil
				}
			}

			// Check if path is in scan_paths
			inScanPath := len(cfg.ScanPaths) == 0 // if no scan_paths, all paths are fair game
			for _, sp := range cfg.ScanPaths {
				if strings.HasPrefix(queryPath, sp) {
					inScanPath = true
					break
				}
			}

			// Check target patterns
			base := filepath.Base(queryPath)
			if eco, ok := targetPatterns[base]; ok {
				if hasSiblingManifest(queryPath, eco) {
					size := getSize(queryPath)
					age := getAge(queryPath)
					printStatus(queryPath, "target ("+eco+")", base+"/ with sibling manifest", age, size)
					return nil
				}
			}

			// Check if it's a Go module cache path
			if strings.Contains(queryPath, "go/pkg/mod") {
				size := getSize(queryPath)
				printStatus(queryPath, "target (go)", "Go module cache", "", size)
				return nil
			}

			if !inScanPath {
				printStatus(queryPath, "not a target", "Path not in scan_paths", "", 0)
				return nil
			}

			printStatus(queryPath, "not a target", "No cleaner recognizes this path", "", 0)
			return nil
		},
	}

	return cmd
}

func hasSiblingManifest(path, ecosystem string) bool {
	parent := filepath.Dir(path)
	manifests, ok := manifestForEcosystem[ecosystem]
	if !ok {
		return false
	}
	for _, m := range manifests {
		if strings.Contains(m, "*") {
			matches, _ := filepath.Glob(filepath.Join(parent, m))
			if len(matches) > 0 {
				return true
			}
		} else {
			if _, err := os.Stat(filepath.Join(parent, m)); err == nil {
				return true
			}
		}
	}
	return false
}

func getSize(path string) int64 {
	size, _ := fs.CalcDirSize(path)
	return size
}

func getAge(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	age := time.Since(info.ModTime())
	days := int(age.Hours() / 24)
	if days == 0 {
		return "today"
	}
	return fmt.Sprintf("%d days", days)
}

func printStatus(path, status, reason, age string, size int64) {
	fmt.Printf("%s\n", path)
	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Reason: %s\n", reason)
	if size > 0 || age != "" {
		parts := []string{}
		if size > 0 {
			parts = append(parts, fmt.Sprintf("Size: %s", formatBytes(size)))
		}
		if age != "" {
			parts = append(parts, fmt.Sprintf("Age: %s", age))
		}
		fmt.Printf("  %s\n", strings.Join(parts, " | "))
	}
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
