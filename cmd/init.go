package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var manifestFiles = map[string]string{
	"go.mod":           "go",
	"package.json":     "node",
	"Cargo.toml":       "rust",
	"pom.xml":          "java",
	"build.gradle":     "java",
	"requirements.txt": "python",
	"pyproject.toml":   "python",
}

func newInitCmd() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize devclean configuration",
		Long:  "Discover development paths and generate a config file. Use --yes to skip prompts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver := platform.New()
			paths := resolver.Resolve()

			configFile := filepath.Join(paths.ConfigDir, "config.yaml")

			if !yes {
				if _, err := os.Stat(configFile); err == nil {
					fmt.Printf("Config already exists: %s\n", configFile)
					fmt.Println("Use --yes to overwrite or delete the file manually.")
					return nil
				}
			}

			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("getting home directory: %w", err)
			}

			scanPaths, ecosystems := discoverPaths(home)

			cfg := buildConfig(scanPaths, ecosystems)

			if err := os.MkdirAll(paths.ConfigDir, 0o755); err != nil {
				return fmt.Errorf("creating config dir: %w", err)
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				return fmt.Errorf("marshaling config: %w", err)
			}

			if err := os.WriteFile(configFile, data, 0o644); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}

			fmt.Printf("Config written to: %s\n\n", configFile)
			printDiscoverySummary(scanPaths, ecosystems)

			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "skip prompts and use discovered defaults")

	return cmd
}

// discoverPaths walks first-level subdirectories of root looking for project manifests.
// Returns discovered scan paths (parent directories) and detected ecosystems.
func discoverPaths(root string) ([]string, []string) {
	pathSet := make(map[string]struct{})
	ecoSet := make(map[string]struct{})

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		dirPath := filepath.Join(root, entry.Name())

		// Walk up to 2 levels deep to find manifests
		filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return fs.SkipDir
			}
			if d.IsDir() {
				// Skip hidden dirs and limit depth
				if strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "node_modules") {
					return fs.SkipDir
				}
				rel, _ := filepath.Rel(dirPath, path)
				if strings.Count(rel, string(filepath.Separator)) >= 2 {
					return fs.SkipDir
				}
				return nil
			}

			if eco, ok := manifestFiles[d.Name()]; ok {
				parent := filepath.Dir(path)
				pathSet[parent] = struct{}{}
				ecoSet[eco] = struct{}{}
			}
			// Check for .sln files
			if strings.HasSuffix(d.Name(), ".sln") {
				parent := filepath.Dir(path)
				pathSet[parent] = struct{}{}
				ecoSet["dotnet"] = struct{}{}
			}
			return nil
		})
	}

	paths := make([]string, 0, len(pathSet))
	for p := range pathSet {
		paths = append(paths, p)
	}

	ecos := make([]string, 0, len(ecoSet))
	for e := range ecoSet {
		ecos = append(ecos, e)
	}

	return paths, ecos
}

func buildConfig(scanPaths, ecosystems []string) *config.Config {
	cfg := config.Defaults()
	cfg.ScanPaths = scanPaths

	ecoSet := make(map[string]struct{})
	for _, e := range ecosystems {
		ecoSet[e] = struct{}{}
	}

	if _, ok := ecoSet["node"]; !ok {
		cfg.Ecosystems.Node.Enabled = false
	}
	if _, ok := ecoSet["go"]; !ok {
		cfg.Ecosystems.Go.Enabled = false
	}
	if _, ok := ecoSet["rust"]; !ok {
		cfg.Ecosystems.Rust.Enabled = false
	}
	if _, ok := ecoSet["java"]; !ok {
		cfg.Ecosystems.Java.Enabled = false
	}
	if _, ok := ecoSet["python"]; !ok {
		cfg.Ecosystems.Python.Enabled = false
	}
	if _, ok := ecoSet["dotnet"]; !ok {
		cfg.Ecosystems.DotNet.Enabled = false
	}

	return &cfg
}

func printDiscoverySummary(paths []string, ecosystems []string) {
	fmt.Printf("Discovered %d project path(s) with %d ecosystem(s):\n", len(paths), len(ecosystems))
	for _, e := range ecosystems {
		fmt.Printf("  • %s\n", e)
	}
	fmt.Println("\nRun `devclean` to start the interactive cleaner.")
}
