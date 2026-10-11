package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHasSiblingManifest_Node(t *testing.T) {
	dir := t.TempDir()
	nodeModules := filepath.Join(dir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	if !hasSiblingManifest(nodeModules, "node") {
		t.Error("expected sibling manifest found for node")
	}
}

func TestHasSiblingManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()
	nodeModules := filepath.Join(dir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0o755)

	if hasSiblingManifest(nodeModules, "node") {
		t.Error("expected no sibling manifest without package.json")
	}
}

func TestHasSiblingManifest_Rust(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0o644)

	if !hasSiblingManifest(target, "rust") {
		t.Error("expected sibling manifest found for rust")
	}
}

func TestHasSiblingManifest_DotNet(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	_ = os.MkdirAll(binDir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "MyApp.csproj"), []byte("<Project></Project>"), 0o644)

	if !hasSiblingManifest(binDir, "dotnet") {
		t.Error("expected sibling manifest found for dotnet (*.csproj)")
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024 * 3, "3.0 MB"},
		{1024 * 1024 * 1024 * 2, "2.0 GB"},
	}
	for _, tc := range cases {
		got := formatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestWhyCmd_ExecutionBranches(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	_ = os.MkdirAll(workspace, 0755)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
exclude_paths:
  - "custom-excluded"
`, workspace)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// 1. Blacklist (.git)
	gitDir := filepath.Join(tmpDir, ".git")
	_ = os.MkdirAll(gitDir, 0755)
	cmd1 := newWhyCmd()
	cmd1.SetArgs([]string{gitDir})
	if err := cmd1.Execute(); err != nil {
		t.Fatalf("why .git failed: %v", err)
	}

	// 2. Excluded pattern (custom-excluded)
	excludedDir := filepath.Join(workspace, "custom-excluded")
	_ = os.MkdirAll(excludedDir, 0755)
	cmd2 := newWhyCmd()
	cmd2.SetArgs([]string{excludedDir})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("why custom-excluded failed: %v", err)
	}

	// 3. Target com sibling manifest (node_modules + package.json)
	nodeApp := filepath.Join(workspace, "app")
	nodeModules := filepath.Join(nodeApp, "node_modules")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(nodeApp, "package.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(nodeModules, "index.js"), []byte("payload"), 0644)
	cmd3 := newWhyCmd()
	cmd3.SetArgs([]string{nodeModules})
	if err := cmd3.Execute(); err != nil {
		t.Fatalf("why node_modules with manifest failed: %v", err)
	}

	// 4. Target sem sibling manifest (target sem Cargo.toml)
	orphanTarget := filepath.Join(workspace, "orphan", "target")
	_ = os.MkdirAll(orphanTarget, 0755)
	cmd4 := newWhyCmd()
	cmd4.SetArgs([]string{orphanTarget})
	if err := cmd4.Execute(); err != nil {
		t.Fatalf("why target without manifest failed: %v", err)
	}

	// 5. Go module cache (contendo go/pkg/mod)
	goModCache := filepath.Join(tmpDir, "go", "pkg", "mod", "cache")
	_ = os.MkdirAll(goModCache, 0755)
	cmd5 := newWhyCmd()
	cmd5.SetArgs([]string{goModCache})
	if err := cmd5.Execute(); err != nil {
		t.Fatalf("why go/pkg/mod failed: %v", err)
	}

	// 6. Path fora de scan_paths
	outsideDir := filepath.Join(tmpDir, "outside")
	_ = os.MkdirAll(outsideDir, 0755)
	cmd6 := newWhyCmd()
	cmd6.SetArgs([]string{outsideDir})
	if err := cmd6.Execute(); err != nil {
		t.Fatalf("why outside scan_paths failed: %v", err)
	}

	// 7. Path em scan_paths mas desconhecido por cleaners
	unknownDir := filepath.Join(workspace, "random-folder")
	_ = os.MkdirAll(unknownDir, 0755)
	cmd7 := newWhyCmd()
	cmd7.SetArgs([]string{unknownDir})
	if err := cmd7.Execute(); err != nil {
		t.Fatalf("why unknownDir failed: %v", err)
	}

	// 8. Erro de configuração
	configPath = "/invalid/path.yaml"
	cmdErr := newWhyCmd()
	cmdErr.SetArgs([]string{workspace})
	if err := cmdErr.Execute(); err == nil {
		t.Error("expected error with invalid configPath, got nil")
	}
}

func TestWhyCmd_Helpers(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(testFile, []byte("hello-devclean"), 0644)

	// getSize
	size := getSize(tmpDir)
	if size < int64(len("hello-devclean")) {
		t.Errorf("expected size >= %d, got %d", len("hello-devclean"), size)
	}

	// getAge
	age := getAge(testFile)
	if age != "today" && age != "0 days" {
		t.Logf("getAge returned: %q", age)
	}

	// Nonexistent file for getAge
	if getAge(filepath.Join(tmpDir, "nonexistent")) != "" {
		t.Error("expected empty age for nonexistent file")
	}

	// Older file
	oldFile := filepath.Join(tmpDir, "old.txt")
	_ = os.WriteFile(oldFile, []byte("old"), 0644)
	oldTime := time.Now().Add(-72 * time.Hour)
	_ = os.Chtimes(oldFile, oldTime, oldTime)
	oldAge := getAge(oldFile)
	if oldAge != "3 days" {
		t.Errorf("expected '3 days', got %q", oldAge)
	}
}

func TestHasSiblingManifest_UnknownEcosystem(t *testing.T) {
	if hasSiblingManifest("/some/path", "unknown") {
		t.Error("expected false for unknown ecosystem")
	}
}

func TestWhyCmd_ExcludedByFullPath(t *testing.T) {
	tmpDir := t.TempDir()
	workspace := filepath.Join(tmpDir, "workspace")
	_ = os.MkdirAll(workspace, 0755)
	excludedDir := filepath.Join(workspace, "full-match")
	_ = os.MkdirAll(excludedDir, 0755)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
exclude_paths:
  - %q
`, workspace, excludedDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newWhyCmd()
	cmd.SetArgs([]string{excludedDir})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("why full-match failed: %v", err)
	}
}

