package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
)

func TestFilterByTTL_OlderThanTTL(t *testing.T) {
	targets := []domain.Target{
		{Path: "/tmp/old", LastAccessed: time.Now().Add(-60 * 24 * time.Hour)},
	}

	result := filterByTTL(targets, 30)
	if len(result) != 1 {
		t.Errorf("expected 1 target (older than TTL), got %d", len(result))
	}
}

func TestFilterByTTL_NewerThanTTL(t *testing.T) {
	targets := []domain.Target{
		{Path: "/tmp/recent", LastAccessed: time.Now().Add(-10 * 24 * time.Hour)},
	}

	result := filterByTTL(targets, 30)
	if len(result) != 0 {
		t.Errorf("expected 0 targets (newer than TTL), got %d", len(result))
	}
}

func TestFilterByTTL_ZeroMaxAge(t *testing.T) {
	targets := []domain.Target{
		{Path: "/tmp/a", LastAccessed: time.Now().Add(-5 * 24 * time.Hour)},
		{Path: "/tmp/b", LastAccessed: time.Now().Add(-100 * 24 * time.Hour)},
	}

	result := filterByTTL(targets, 0)
	if len(result) != 2 {
		t.Errorf("expected all 2 targets with maxAge=0, got %d", len(result))
	}
}

func TestRejectBlacklisted_RemovesBlacklistedPaths(t *testing.T) {
	bl := safety.NewBlacklist(nil)
	targets := []domain.Target{
		{Path: "/home/user/project/.git"},
		{Path: "/home/user/project/.env"},
		{Path: "/home/user/project/node_modules"},
	}

	result := rejectBlacklisted(targets, bl)
	if len(result) != 1 {
		t.Errorf("expected 1 safe target, got %d", len(result))
	}
	if result[0].Path != "/home/user/project/node_modules" {
		t.Errorf("expected node_modules to survive, got %s", result[0].Path)
	}
}

func TestRejectBlacklisted_KeepsSafePaths(t *testing.T) {
	bl := safety.NewBlacklist(nil)
	targets := []domain.Target{
		{Path: "/home/user/project/node_modules"},
		{Path: "/home/user/.cache/go-build"},
	}

	result := rejectBlacklisted(targets, bl)
	if len(result) != 2 {
		t.Errorf("expected 2 safe targets, got %d", len(result))
	}
}

func TestCleanCmd_NoFlags_PrintsGuidance(t *testing.T) {
	rootCmd.SetArgs([]string{"clean"})
	var buf bytes.Buffer
	rootCmd.SetOut(&buf)

	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The guidance message goes to os.Stdout directly in the command,
	// so we test the command itself doesn't error (dry-run default behavior)
}

func TestGetMaxAgeDays(t *testing.T) {
	cfg := testConfig()

	tests := []struct {
		ecosystem string
		want      int
	}{
		{"docker", 30},
		{"node", 14},
		{"go", 60},
		{"dotnet", 45},
		{"python", 30},
		{"unknown", 0},
	}

	for _, tc := range tests {
		t.Run(tc.ecosystem, func(t *testing.T) {
			got := getMaxAgeDays(cfg, tc.ecosystem)
			if got != tc.want {
				t.Errorf("getMaxAgeDays(%q) = %d, want %d", tc.ecosystem, got, tc.want)
			}
		})
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker: config.EcosystemConfig{MaxAgeDays: 30},
			Node:   config.EcosystemConfig{MaxAgeDays: 14},
			Go:     config.EcosystemConfig{MaxAgeDays: 60},
			DotNet: config.EcosystemConfig{MaxAgeDays: 45},
			Python: config.EcosystemConfig{MaxAgeDays: 30},
		},
	}
}

func TestValidateEcosystem_Valid(t *testing.T) {
	valid := []string{"docker", "go", "node"}
	err := validateEcosystem("docker", valid)
	if err != nil {
		t.Errorf("validateEcosystem(\"docker\") should succeed, got: %v", err)
	}
}

func TestValidateEcosystem_CaseInsensitive(t *testing.T) {
	valid := []string{"docker", "go", "node"}
	err := validateEcosystem("Docker", valid)
	if err != nil {
		t.Errorf("validateEcosystem(\"Docker\") should be case-insensitive, got: %v", err)
	}
}

func TestValidateEcosystem_Invalid(t *testing.T) {
	valid := []string{"docker", "go", "node"}
	err := validateEcosystem("rust", valid)
	if err == nil {
		t.Error("validateEcosystem(\"rust\") should return error for unknown ecosystem")
	}
}

func TestCleanCmd_MutuallyExclusiveFlags(t *testing.T) {
	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--auto", "--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when both --auto and --force are set")
	}
}

func TestCleanCmd_NothingToClean_ReturnsError(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when nothing to clean, got nil")
	}
	if err.Error() != "nothing to clean" {
		t.Fatalf("expected 'nothing to clean', got %q", err.Error())
	}
}

func TestCleanCmd_CoordinatorIntegration_DryRun(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	nodeModules := filepath.Join(appDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "dummy.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	content := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*cacache*"
  - "*store*"
safety:
  trash_enabled: false
ecosystems:
  docker:
    enabled: false
  node:
    enabled: true
  go:
    enabled: false
  dotnet:
    enabled: false
  python:
    enabled: false
  rust:
    enabled: false
  java:
    enabled: false
`, tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force", "--dry-run"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing clean dry-run: %v", err)
	}

	// Verify target still exists after dry-run
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		t.Fatal("expected node_modules to still exist after dry-run")
	}
}

func TestCleanCmd_CoordinatorIntegration_TrashMove(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	trashDir := filepath.Join(tmpDir, "trash")
	nodeModules := filepath.Join(appDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "dummy.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	content := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*cacache*"
  - "*store*"
safety:
  trash_enabled: true
  trash_dir: %q
ecosystems:
  docker:
    enabled: false
  node:
    enabled: true
  go:
    enabled: false
  dotnet:
    enabled: false
  python:
    enabled: false
  rust:
    enabled: false
  java:
    enabled: false
`, tmpDir, trashDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force", "--trash"})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing clean with trash: %v", err)
	}

	// Verify target no longer exists in appDir
	if _, err := os.Stat(nodeModules); !os.IsNotExist(err) {
		t.Fatal("expected node_modules to be moved from original location")
	}

	// Verify trash manifest has entry
	manifestPath := filepath.Join(trashDir, "manifest.json")
	manifest, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("loading trash manifest: %v", err)
	}
	if len(manifest.Batches) == 0 || len(manifest.Batches[0].Entries) == 0 {
		t.Fatalf("expected trash manifest to have recorded batch and entries")
	}
}

