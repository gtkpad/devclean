package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
	"github.com/gtkpad/devclean/internal/ui"
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
		{"rust", 30},
		{"java", 30},
		{"ai", 30},
		{"pnpm", 30},
		{"bun", 14},
		{"nextjs", 7},
		{"xcode", 14},
		{"homebrew", 30},
		{"cpp", 14},
		{"flutter", 14},
		{"terraform", 30},
		{"cocoapods", 14},
		{"elixir", 14},
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

func TestGetMaxAgeDays_All18Ecosystems(t *testing.T) {
	TestGetMaxAgeDays(t)
}

func testConfig() *config.Config {
	return &config.Config{
		Ecosystems: config.EcosystemsConfig{
			Docker:    config.EcosystemConfig{MaxAgeDays: 30},
			Node:      config.EcosystemConfig{MaxAgeDays: 14},
			Go:        config.EcosystemConfig{MaxAgeDays: 60},
			DotNet:    config.EcosystemConfig{MaxAgeDays: 45},
			Python:    config.EcosystemConfig{MaxAgeDays: 30},
			Rust:      config.EcosystemConfig{MaxAgeDays: 30},
			Java:      config.EcosystemConfig{MaxAgeDays: 30},
			AI:        config.EcosystemConfig{MaxAgeDays: 30},
			Pnpm:      config.EcosystemConfig{MaxAgeDays: 30},
			Bun:       config.EcosystemConfig{MaxAgeDays: 14},
			NextJS:    config.EcosystemConfig{MaxAgeDays: 7},
			Xcode:     config.EcosystemConfig{MaxAgeDays: 14},
			Homebrew:  config.EcosystemConfig{MaxAgeDays: 30},
			CPP:       config.EcosystemConfig{MaxAgeDays: 14},
			Flutter:   config.EcosystemConfig{MaxAgeDays: 14},
			Terraform: config.EcosystemConfig{MaxAgeDays: 30},
			CocoaPods: config.EcosystemConfig{MaxAgeDays: 14},
			Elixir:    config.EcosystemConfig{MaxAgeDays: 14},
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

func TestCleanCmd_FlagValidations(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte("dry_run: true\n"), 0644)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	tests := []struct {
		name string
		args []string
	}{
		{"auto and force", []string{"--auto", "--force"}},
		{"trash and permanent", []string{"--trash", "--permanent", "--force"}},
		{"smart without auto", []string{"--smart", "--force"}},
		{"smart and force", []string{"--smart", "--force", "--auto"}},
		{"invalid ecosystem", []string{"--force", "-e", "nonexistent"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCleanCmd()
			cmd.SetArgs(tt.args)
			err := cmd.Execute()
			if err == nil {
				t.Errorf("expected error for args %v, got nil", tt.args)
			}
		})
	}
}

func TestCleanCmd_AutoModeWithOldTarget(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	appDir := filepath.Join(tmpDir, "app")
	nodeModules := filepath.Join(appDir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0644)
	dummy := filepath.Join(nodeModules, "dummy.txt")
	_ = os.WriteFile(dummy, []byte("old"), 0644)
	past := time.Now().Add(-60 * 24 * time.Hour)
	_ = os.Chtimes(nodeModules, past, past)
	_ = os.Chtimes(dummy, past, past)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*store*"
  - "*cacache*"
intelligence:
  threshold: 0.0
ecosystems:
  docker:
    enabled: false
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
  node:
    enabled: true
    max_age_days: 10
`, tmpDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--auto", "--smart", "--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean --auto --smart --dry-run failed: %v", err)
	}
}

func TestCleanCmd_PermanentDirectDelete(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	appDir := filepath.Join(tmpDir, "app")
	nodeModules := filepath.Join(appDir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0644)
	dummy := filepath.Join(nodeModules, "dummy.txt")
	_ = os.WriteFile(dummy, []byte("data"), 0644)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*store*"
  - "*cacache*"
ecosystems:
  docker:
    enabled: false
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
  node:
    enabled: true
`, tmpDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force", "--permanent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean --force --permanent failed: %v", err)
	}

	if _, err := os.Stat(nodeModules); !os.IsNotExist(err) {
		t.Error("expected node_modules to be deleted directly with permanent")
	}
}

func TestCleanCmd_InvalidConfig(t *testing.T) {
	origConfigPath := configPath
	configPath = "/invalid/path.yaml"
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestCleanCmd_DryRunEcosystemAndQuiet(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	nodeModules := filepath.Join(appDir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(nodeModules, "dummy.txt"), []byte("data"), 0644)

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
protected_patterns:
  - "*store*"
  - "*cacache*"
ecosystems:
  docker:
    enabled: false
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
  node:
    enabled: true
`, tmpDir)
	_ = os.WriteFile(cfgPath, []byte(cfgContent), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	origQuiet := quiet
	quiet = true
	defer func() { quiet = origQuiet }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--force", "--dry-run", "--ecosystem", "node"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("clean --force --dry-run --ecosystem node --quiet failed: %v", err)
	}
}

func TestCleanCmd_NewEcosystemsValidation(t *testing.T) {
	newEcos := []string{"ai", "pnpm", "bun", "nextjs", "xcode", "homebrew"}
	for _, eco := range newEcos {
		t.Run(eco, func(t *testing.T) {
			cfgFile := createEmptyConfig(t)
			origConfigPath := configPath
			configPath = cfgFile
			defer func() { configPath = origConfigPath }()

			cmd := newCleanCmd()
			cmd.SetArgs([]string{"--force", "-e", eco})
			err := cmd.Execute()
			// Should fail with "nothing to clean" (empty config), NOT "unknown ecosystem"
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != "nothing to clean" {
				t.Fatalf("expected 'nothing to clean' for eco %s, got %v", eco, err)
			}
		})
	}

	t.Run("invalid ecosystem", func(t *testing.T) {
		cmd := newCleanCmd()
		cmd.SetArgs([]string{"--force", "-e", "nonexistent-eco"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown ecosystem") {
			t.Fatalf("expected unknown ecosystem error, got %v", err)
		}
	})
}

func TestCleanCmd_AllFlag(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--auto", "--all"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when nothing to clean, got nil")
	}
	if err.Error() != "nothing to clean" {
		t.Fatalf("expected 'nothing to clean', got %v", err)
	}
}

func setupCleanFixture(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "app")
	nodeModules := filepath.Join(appDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "dummy.txt"), []byte("data-12345"), 0o644); err != nil {
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
  ai:
    enabled: false
  pnpm:
    enabled: false
  bun:
    enabled: false
  nextjs:
    enabled: false
  xcode:
    enabled: false
  homebrew:
    enabled: false
`, tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmpDir, cfgPath
}

func TestClean_ReportCard(t *testing.T) {
	origQuiet := quiet
	defer func() { quiet = origQuiet }()

	t.Run("renders visual card on force deletion", func(t *testing.T) {
		quiet = false
		_, cfgPath := setupCleanFixture(t)
		origConfigPath := configPath
		configPath = cfgPath
		defer func() { configPath = origConfigPath }()

		cmd := newCleanCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--force", "-e", "node"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "Clean Summary") {
			t.Errorf("expected 'Clean Summary' in card output, got: %s", out)
		}
		if !strings.Contains(out, "Espaço Recuperado:") {
			t.Errorf("expected 'Espaço Recuperado:' in card output, got: %s", out)
		}
		if !strings.Contains(out, "Alvos Processados:") {
			t.Errorf("expected 'Alvos Processados:' in card output, got: %s", out)
		}
		if !strings.Contains(out, "node") {
			t.Errorf("expected ecosystem 'node' in breakdown, got: %s", out)
		}
	})

	t.Run("renders dry run title with simulation", func(t *testing.T) {
		quiet = false
		tmpDir, cfgPath := setupCleanFixture(t)
		origConfigPath := configPath
		configPath = cfgPath
		defer func() { configPath = origConfigPath }()

		cmd := newCleanCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--force", "--dry-run", "-e", "node"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "Clean Dry-Run Summary") {
			t.Errorf("expected 'Clean Dry-Run Summary' in dry run card, got: %s", out)
		}

		// Ensure file was not deleted
		nodeModules := filepath.Join(tmpDir, "app", "node_modules")
		if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
			t.Fatal("expected node_modules to still exist in dry-run mode")
		}
	})

	t.Run("json format outputs structured metrics", func(t *testing.T) {
		quiet = false
		_, cfgPath := setupCleanFixture(t)
		origConfigPath := configPath
		configPath = cfgPath
		defer func() { configPath = origConfigPath }()

		cmd := newCleanCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--force", "--dry-run", "--format", "json", "-e", "node"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var report ui.CleanReportJSON
		if err := json.Unmarshal(outBuf.Bytes(), &report); err != nil {
			t.Fatalf("failed to parse JSON report: %v, raw output: %s", err, outBuf.String())
		}

		if report.Title != "Clean Dry-Run Summary" {
			t.Errorf("expected title 'Clean Dry-Run Summary', got %q", report.Title)
		}
		if report.TotalCount != 1 {
			t.Errorf("expected TotalCount 1, got %d", report.TotalCount)
		}
		if report.TotalFreed <= 0 {
			t.Errorf("expected TotalFreed > 0, got %d", report.TotalFreed)
		}
		if len(report.Ecosystems) != 1 || report.Ecosystems[0].Name != "node" {
			t.Errorf("expected 1 node ecosystem metric, got %+v", report.Ecosystems)
		}
	})

	t.Run("quiet mode suppresses all stdout", func(t *testing.T) {
		quiet = true
		_, cfgPath := setupCleanFixture(t)
		origConfigPath := configPath
		configPath = cfgPath
		defer func() { configPath = origConfigPath }()

		cmd := newCleanCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--force", "-e", "node"})

		err := cmd.Execute()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if outBuf.Len() > 0 {
			t.Errorf("expected empty stdout with quiet=true, got: %q", outBuf.String())
		}
	})

	t.Run("invalid format returns error", func(t *testing.T) {
		cmd := newCleanCmd()
		cmd.SetArgs([]string{"--force", "--format", "xml"})

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error for invalid format, got nil")
		}
		if !strings.Contains(err.Error(), "unknown format") {
			t.Errorf("expected unknown format error, got %v", err)
		}
	})
}

func setupMultiEcosystemCleanFixture(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("CARGO_HOME", filepath.Join(tmpDir, "fake_cargo"))

	// Node target: node_modules/cache.tmp
	appDir := filepath.Join(tmpDir, "node-app")
	nodeModules := filepath.Join(appDir, "node_modules")
	if err := os.MkdirAll(nodeModules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodeModules, "cache.tmp"), []byte("node-cache-payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Rust target: target/debug/build.bin
	rustDir := filepath.Join(tmpDir, "rust-app")
	rustTarget := filepath.Join(rustDir, "target")
	if err := os.MkdirAll(filepath.Join(rustTarget, "debug"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rustDir, "Cargo.toml"), []byte("[package]\nname = \"demo\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rustTarget, "debug", "build.bin"), []byte("rust-binary-payload"), 0o644); err != nil {
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
    enabled: true
  java:
    enabled: false
  ai:
    enabled: false
  pnpm:
    enabled: false
  bun:
    enabled: false
  nextjs:
    enabled: false
  xcode:
    enabled: false
  homebrew:
    enabled: false
`, tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return tmpDir, cfgPath
}

func TestCleanCmd_ReportAndJSON(t *testing.T) {
	origQuiet := quiet
	quiet = false
	defer func() { quiet = origQuiet }()

	tmpDir, cfgPath := setupMultiEcosystemCleanFixture(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--force", "--format", "json"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing clean --format json: %v", err)
	}

	var report ui.CleanReportJSON
	if err := json.Unmarshal(outBuf.Bytes(), &report); err != nil {
		t.Fatalf("failed to decode JSON output: %v, raw output: %s", err, outBuf.String())
	}

	if report.Title != "Clean Summary" {
		t.Errorf("expected title 'Clean Summary', got %q", report.Title)
	}
	if report.TotalCount != 2 {
		t.Errorf("expected total_count == 2, got %d", report.TotalCount)
	}
	if report.TotalFreed <= 0 {
		t.Errorf("expected total_freed > 0, got %d", report.TotalFreed)
	}
	if report.TotalFreedHuman == "" {
		t.Errorf("expected total_freed_human to be non-empty")
	}
	if report.DurationMs < 0 {
		t.Errorf("expected non-negative duration_ms, got %d", report.DurationMs)
	}
	if len(report.Ecosystems) != 2 {
		t.Errorf("expected 2 ecosystems (node, rust), got %d: %+v", len(report.Ecosystems), report.Ecosystems)
	}

	// Verify both build artifacts were physically removed
	nodeModules := filepath.Join(tmpDir, "node-app", "node_modules")
	if _, err := os.Stat(nodeModules); !os.IsNotExist(err) {
		t.Errorf("expected node_modules to be deleted, but still exists")
	}
	rustTarget := filepath.Join(tmpDir, "rust-app", "target")
	if _, err := os.Stat(rustTarget); !os.IsNotExist(err) {
		t.Errorf("expected rust target to be deleted, but still exists")
	}
}

func TestCleanCmd_QuietFlag(t *testing.T) {
	origQuiet := quiet
	quiet = true
	defer func() { quiet = origQuiet }()

	tmpDir, cfgPath := setupCleanFixture(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--force", "-e", "node"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing clean in quiet mode: %v", err)
	}

	if outBuf.Len() > 0 {
		t.Errorf("expected empty stdout with --quiet, got: %q", outBuf.String())
	}

	// Verify real removal still happened in quiet mode
	nodeModules := filepath.Join(tmpDir, "app", "node_modules")
	if _, err := os.Stat(nodeModules); !os.IsNotExist(err) {
		t.Errorf("expected node_modules to be deleted in quiet mode, but still exists")
	}
}

func TestCleanCmd_DryRunReport(t *testing.T) {
	origQuiet := quiet
	quiet = false
	defer func() { quiet = origQuiet }()

	tmpDir, cfgPath := setupCleanFixture(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--force", "--dry-run", "-e", "node"})

	err := cmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error executing clean --dry-run: %v", err)
	}

	out := outBuf.String()
	if !strings.Contains(out, "Clean Dry-Run Summary") {
		t.Errorf("expected 'Clean Dry-Run Summary' in output, got: %s", out)
	}
	if !strings.Contains(out, "Espaço Recuperado:") {
		t.Errorf("expected 'Espaço Recuperado:' in output, got: %s", out)
	}

	// Verify file was NOT physically deleted
	nodeModules := filepath.Join(tmpDir, "app", "node_modules")
	if _, err := os.Stat(nodeModules); os.IsNotExist(err) {
		t.Fatal("expected node_modules to still exist after dry-run")
	}
}

func TestCleanCmd_MutualExclusiveFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"trash and permanent", []string{"--force", "--trash", "--permanent"}},
		{"auto and force", []string{"--auto", "--force"}},
		{"smart and force", []string{"--smart", "--force"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := newCleanCmd()
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error for %v, got nil", tc.args)
			}
		})
	}
}

func TestCleanCmd_PresetValidation(t *testing.T) {
	tests := []struct {
		name    string
		preset  string
		wantErr bool
	}{
		{"valid safe", "safe", false},
		{"valid standard", "standard", false},
		{"valid aggressive", "aggressive", false},
		{"invalid preset nuclear", "nuclear", true},
		{"invalid preset unknown", "unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgFile := createEmptyConfig(t)
			origConfigPath := configPath
			configPath = cfgFile
			defer func() { configPath = origConfigPath }()

			cmd := newCleanCmd()
			cmd.SetArgs([]string{"--preset", tt.preset, "--dry-run"})
			err := cmd.Execute()
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "unknown preset") {
					t.Fatalf("expected unknown preset error, got %v", err)
				}
			} else {
				if err == nil || err.Error() != "nothing to clean" {
					t.Fatalf("expected 'nothing to clean' indicating preset was accepted, got %v", err)
				}
			}
		})
	}
}

func TestCleanCmd_PresetMutuallyExclusiveWithForce(t *testing.T) {
	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--preset", "safe", "--force"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--preset and --force are mutually exclusive") {
		t.Fatalf("expected mutually exclusive error, got %v", err)
	}
}

func TestCleanCmd_PresetImpliesAuto(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--preset", "safe", "--dry-run"})
	err := cmd.Execute()
	if err == nil || err.Error() != "nothing to clean" {
		t.Fatalf("expected 'nothing to clean', got %v", err)
	}
}

func TestCleanCmd_PresetAggressive_ZerosTTL(t *testing.T) {
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "node-app")
	nodeModules := filepath.Join(appDir, "node_modules")
	_ = os.MkdirAll(nodeModules, 0755)
	_ = os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(nodeModules, "temp.log"), []byte("log-data"), 0644)
	recent := time.Now().Add(-10 * time.Minute)
	_ = os.Chtimes(nodeModules, recent, recent)

	content := fmt.Sprintf(`scan_paths:
  - %q
safety:
  trash_enabled: false
ecosystems:
  node:
    enabled: true
    max_age_days: 14
`, tmpDir)
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	_ = os.WriteFile(cfgPath, []byte(content), 0644)

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	cmdStandard := newCleanCmd()
	cmdStandard.SetArgs([]string{"--preset", "standard", "-e", "node", "--dry-run"})
	if err := cmdStandard.Execute(); err == nil || err.Error() != "nothing to clean" {
		t.Fatalf("expected standard preset to skip recent target, got: %v", err)
	}

	cmdAggressive := newCleanCmd()
	var outBuf bytes.Buffer
	cmdAggressive.SetOut(&outBuf)
	cmdAggressive.SetArgs([]string{"--preset", "aggressive", "-e", "node", "--dry-run"})
	if err := cmdAggressive.Execute(); err != nil {
		t.Fatalf("aggressive preset should succeed and target recent item: %v", err)
	}
	if !strings.Contains(outBuf.String(), "Clean Dry-Run Summary") {
		t.Errorf("expected clean summary in output, got %s", outBuf.String())
	}
	if !strings.Contains(outBuf.String(), "node") {
		t.Errorf("expected node in breakdown, got %s", outBuf.String())
	}
}

func TestCleanCmd_PresetSafe_FiltersLocalBuilds(t *testing.T) {
	targets := []domain.Target{
		{ID: "cmake", Category: "cmake-build", Risk: domain.RiskLow},
		{ID: "vcpkg", Category: "vcpkg-installed", Risk: domain.RiskLow},
		{ID: "flutter-b", Category: "flutter-build", Risk: domain.RiskLow},
		{ID: "flutter-dt", Category: "flutter-dart-tool", Risk: domain.RiskLow},
		{ID: "tf-local", Category: "terraform-local", Risk: domain.RiskLow},
		{ID: "pods", Category: "cocoapods-pods", Risk: domain.RiskLow},
		{ID: "elixir-b", Category: "elixir-build", Risk: domain.RiskLow},
		{ID: "elixir-d", Category: "elixir-deps", Risk: domain.RiskLow},
		{ID: "cargo-t", Category: "cargo-target", Risk: domain.RiskLow},
		{ID: "node-m", Category: "node-modules", Risk: domain.RiskLow},
		{ID: "node", Category: "node", Risk: domain.RiskLow},
		{ID: "node-d", Category: "node-dist", Risk: domain.RiskLow},
		{ID: "nuxt-c", Category: "nuxt-cache", Risk: domain.RiskLow},
		{ID: "dotnet-b", Category: "dotnet-build", Risk: domain.RiskLow},
		{ID: "mvn-t", Category: "maven-target", Risk: domain.RiskLow},
		{ID: "grad-b", Category: "gradle-build", Risk: domain.RiskLow},
		{ID: "next-b", Category: "nextjs-build", Risk: domain.RiskLow},
		{ID: "py-venv", Category: "python-venv-orphan", Risk: domain.RiskLow},
		{ID: "py-cache", Category: "python-pycache", Risk: domain.RiskLow},
		{ID: "dd", Category: "derived-data", Risk: domain.RiskLow},
		{ID: "arch", Category: "archives", Risk: domain.RiskLow},
		{ID: "global-med", Category: "conan-cache", Risk: domain.RiskMedium},
		{ID: "conan-c", Category: "conan-cache", Risk: domain.RiskLow},
		{ID: "pub-c", Category: "pub-cache", Risk: domain.RiskLow},
		{ID: "hex-pkg", Category: "hex-packages", Risk: domain.RiskLow},
		{ID: "tf-cache", Category: "terraform-plugin-cache", Risk: domain.RiskLow},
		{ID: "pod-cache", Category: "cocoapods-cache", Risk: domain.RiskLow},
	}

	filtered := filterSafePreset(targets)
	if len(filtered) != 5 {
		t.Fatalf("expected exactly 5 global cache targets, got %d: %+v", len(filtered), filtered)
	}
	expectedIDs := map[string]bool{
		"conan-c": true, "pub-c": true, "hex-pkg": true, "tf-cache": true, "pod-cache": true,
	}
	for _, f := range filtered {
		if !expectedIDs[f.ID] {
			t.Errorf("unexpected target survived safe preset: %s (%s)", f.ID, f.Category)
		}
	}
}

func TestCleanCmd_PresetWithSmart(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newCleanCmd()
	cmd.SetArgs([]string{"--preset", "safe", "--smart", "--dry-run"})
	err := cmd.Execute()
	if err == nil || err.Error() != "nothing to clean" {
		t.Fatalf("expected 'nothing to clean', got %v", err)
	}
}

func TestCleanCmd_PresetRespectsBlacklist(t *testing.T) {
	bl := safety.NewBlacklist(nil)
	targets := []domain.Target{
		{Path: "/home/user/project/.git", Risk: domain.RiskLow},
		{Path: "/home/user/project/.env", Risk: domain.RiskLow},
		{Path: "/home/user/project/.ssh", Risk: domain.RiskLow},
		{Path: "/home/user/.cache/conan", Risk: domain.RiskLow},
	}

	safe := rejectBlacklisted(targets, bl)
	if len(safe) != 1 || safe[0].Path != "/home/user/.cache/conan" {
		t.Fatalf("expected only /home/user/.cache/conan to survive blacklist, got: %+v", safe)
	}
}








