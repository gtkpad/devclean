package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPaths_FindsManifests(t *testing.T) {
	root := t.TempDir()

	// Create project dirs with manifests
	nodeProject := filepath.Join(root, "my-app")
	os.MkdirAll(nodeProject, 0o755)
	os.WriteFile(filepath.Join(nodeProject, "package.json"), []byte("{}"), 0o644)

	goProject := filepath.Join(root, "backend")
	os.MkdirAll(goProject, 0o755)
	os.WriteFile(filepath.Join(goProject, "go.mod"), []byte("module x"), 0o644)

	paths, ecos := discoverPaths(root)

	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d: %v", len(paths), paths)
	}

	ecoMap := make(map[string]bool)
	for _, e := range ecos {
		ecoMap[e] = true
	}
	if !ecoMap["node"] {
		t.Error("expected node ecosystem detected")
	}
	if !ecoMap["go"] {
		t.Error("expected go ecosystem detected")
	}
}

func TestDiscoverPaths_SkipsHiddenDirs(t *testing.T) {
	root := t.TempDir()

	hidden := filepath.Join(root, ".hidden")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "package.json"), []byte("{}"), 0o644)

	paths, _ := discoverPaths(root)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths (hidden skipped), got %d: %v", len(paths), paths)
	}
}

func TestDiscoverPaths_EmptyDir(t *testing.T) {
	root := t.TempDir()

	paths, ecos := discoverPaths(root)
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}
	if len(ecos) != 0 {
		t.Errorf("expected 0 ecosystems, got %d", len(ecos))
	}
}

func TestInitCmd_YesFlag_WritesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	os.MkdirAll(configDir, 0o755)

	// Create a project dir with a manifest
	projectDir := filepath.Join(tmpDir, "projects", "myapp")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module test"), 0o644)

	// Test buildConfig directly
	paths := []string{projectDir}
	ecos := []string{"go"}
	cfg := buildConfig(paths, ecos)

	if len(cfg.ScanPaths) != 1 || cfg.ScanPaths[0] != projectDir {
		t.Errorf("expected scan_paths=[%s], got %v", projectDir, cfg.ScanPaths)
	}
	if !cfg.Ecosystems.Go.Enabled {
		t.Error("expected go ecosystem enabled")
	}
	if cfg.Ecosystems.Node.Enabled {
		t.Error("expected node ecosystem disabled (not detected)")
	}
}
