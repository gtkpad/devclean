package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_NoFile_ReturnsDefaults(t *testing.T) {
	// With empty path (discovery mode), missing config returns defaults
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Ecosystems.Docker.MaxAgeDays != 30 {
		t.Errorf("Docker.MaxAgeDays = %d, want 30", cfg.Ecosystems.Docker.MaxAgeDays)
	}
	if cfg.Ecosystems.Node.MaxAgeDays != 14 {
		t.Errorf("Node.MaxAgeDays = %d, want 14", cfg.Ecosystems.Node.MaxAgeDays)
	}
	if cfg.Ecosystems.Go.MaxAgeDays != 60 {
		t.Errorf("Go.MaxAgeDays = %d, want 60", cfg.Ecosystems.Go.MaxAgeDays)
	}
	if cfg.Ecosystems.DotNet.MaxAgeDays != 45 {
		t.Errorf("DotNet.MaxAgeDays = %d, want 45", cfg.Ecosystems.DotNet.MaxAgeDays)
	}
	if cfg.Ecosystems.Python.MaxAgeDays != 30 {
		t.Errorf("Python.MaxAgeDays = %d, want 30", cfg.Ecosystems.Python.MaxAgeDays)
	}
	if cfg.Ecosystems.Rust.MaxAgeDays != 30 {
		t.Errorf("Rust.MaxAgeDays = %d, want 30", cfg.Ecosystems.Rust.MaxAgeDays)
	}
	if cfg.Ecosystems.Java.MaxAgeDays != 30 {
		t.Errorf("Java.MaxAgeDays = %d, want 30", cfg.Ecosystems.Java.MaxAgeDays)
	}
	if cfg.Ecosystems.NextJS.MaxAgeDays != 7 {
		t.Errorf("NextJS.MaxAgeDays = %d, want 7", cfg.Ecosystems.NextJS.MaxAgeDays)
	}
	if cfg.Ecosystems.Bun.MaxAgeDays != 14 {
		t.Errorf("Bun.MaxAgeDays = %d, want 14", cfg.Ecosystems.Bun.MaxAgeDays)
	}
	if cfg.Ecosystems.Xcode.MaxAgeDays != 14 {
		t.Errorf("Xcode.MaxAgeDays = %d, want 14", cfg.Ecosystems.Xcode.MaxAgeDays)
	}
	if cfg.Ecosystems.AI.MaxAgeDays != 30 {
		t.Errorf("AI.MaxAgeDays = %d, want 30", cfg.Ecosystems.AI.MaxAgeDays)
	}
	if cfg.Ecosystems.Pnpm.MaxAgeDays != 30 {
		t.Errorf("Pnpm.MaxAgeDays = %d, want 30", cfg.Ecosystems.Pnpm.MaxAgeDays)
	}
	if cfg.Ecosystems.Homebrew.MaxAgeDays != 30 {
		t.Errorf("Homebrew.MaxAgeDays = %d, want 30", cfg.Ecosystems.Homebrew.MaxAgeDays)
	}
	if cfg.Hooks.NextJS.Timeout != "30s" {
		t.Errorf("Hooks.NextJS.Timeout = %q, want 30s", cfg.Hooks.NextJS.Timeout)
	}
	if cfg.Hooks.AI.Timeout != "30s" {
		t.Errorf("Hooks.AI.Timeout = %q, want 30s", cfg.Hooks.AI.Timeout)
	}
	if !cfg.Ecosystems.Docker.Enabled {
		t.Error("Docker.Enabled should be true by default")
	}
	if len(cfg.ScanPaths) != 0 {
		t.Errorf("ScanPaths should be empty, got %v", cfg.ScanPaths)
	}
	if len(cfg.ProtectedPatterns) != 0 {
		t.Errorf("ProtectedPatterns should be empty, got %v", cfg.ProtectedPatterns)
	}
}

func TestLoad_ExplicitPath_NotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.yaml")

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() should error when explicit config path does not exist")
	}
}

func TestLoad_YAMLFile_OverridesDefaults(t *testing.T) {
	yaml := `
scan_paths:
  - /tmp/test-projects
protected_patterns:
  - "*.key"
ecosystems:
  node:
    enabled: true
    max_age_days: 45
    exclude_paths:
      - /tmp/keep-this
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Ecosystems.Node.MaxAgeDays != 45 {
		t.Errorf("Node.MaxAgeDays = %d, want 45", cfg.Ecosystems.Node.MaxAgeDays)
	}
	if cfg.Ecosystems.Docker.MaxAgeDays != 30 {
		t.Errorf("Docker.MaxAgeDays = %d, want 30 (default preserved)", cfg.Ecosystems.Docker.MaxAgeDays)
	}
	if len(cfg.ScanPaths) != 1 || cfg.ScanPaths[0] != "/tmp/test-projects" {
		t.Errorf("ScanPaths = %v, want [/tmp/test-projects]", cfg.ScanPaths)
	}
	if len(cfg.ProtectedPatterns) != 1 || cfg.ProtectedPatterns[0] != "*.key" {
		t.Errorf("ProtectedPatterns = %v, want [*.key]", cfg.ProtectedPatterns)
	}
	if len(cfg.Ecosystems.Node.ExcludePaths) != 1 || cfg.Ecosystems.Node.ExcludePaths[0] != "/tmp/keep-this" {
		t.Errorf("Node.ExcludePaths = %v, want [/tmp/keep-this]", cfg.Ecosystems.Node.ExcludePaths)
	}
}

func TestLoad_InvalidYAML_ReturnsError(t *testing.T) {
	path := writeTestConfig(t, "{{invalid yaml content")
	_, err := Load(path)
	if err == nil {
		t.Error("Load() should return error for invalid YAML")
	}
}

func TestLoad_PartialYAML_MergesWithDefaults(t *testing.T) {
	yaml := `
ecosystems:
  docker:
    max_age_days: 7
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Ecosystems.Docker.MaxAgeDays != 7 {
		t.Errorf("Docker.MaxAgeDays = %d, want 7 (overridden)", cfg.Ecosystems.Docker.MaxAgeDays)
	}
	if cfg.Ecosystems.Node.MaxAgeDays != 14 {
		t.Errorf("Node.MaxAgeDays = %d, want 14 (default preserved)", cfg.Ecosystems.Node.MaxAgeDays)
	}
	if cfg.Ecosystems.Go.MaxAgeDays != 60 {
		t.Errorf("Go.MaxAgeDays = %d, want 60 (default preserved)", cfg.Ecosystems.Go.MaxAgeDays)
	}
}

func TestDefaults_AllEcosystemsEnabled(t *testing.T) {
	d := Defaults()

	ecosystems := []struct {
		name    string
		cfg     EcosystemConfig
		wantTTL int
	}{
		{"Docker", d.Ecosystems.Docker, 30},
		{"Node", d.Ecosystems.Node, 14},
		{"Go", d.Ecosystems.Go, 60},
		{"DotNet", d.Ecosystems.DotNet, 45},
		{"Python", d.Ecosystems.Python, 30},
		{"Rust", d.Ecosystems.Rust, 30},
		{"Java", d.Ecosystems.Java, 30},
		{"AI", d.Ecosystems.AI, 30},
		{"Pnpm", d.Ecosystems.Pnpm, 30},
		{"Bun", d.Ecosystems.Bun, 14},
		{"NextJS", d.Ecosystems.NextJS, 7},
		{"Xcode", d.Ecosystems.Xcode, 14},
		{"Homebrew", d.Ecosystems.Homebrew, 30},
	}

	for _, ec := range ecosystems {
		if !ec.cfg.Enabled {
			t.Errorf("%s.Enabled = false, want true", ec.name)
		}
		if ec.cfg.MaxAgeDays != ec.wantTTL {
			t.Errorf("%s.MaxAgeDays = %d, want %d", ec.name, ec.cfg.MaxAgeDays, ec.wantTTL)
		}
	}
}

func TestLoad_ExplicitPath_UsedDirectly(t *testing.T) {
	yaml := `
scan_paths:
  - /explicit/path
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(cfg.ScanPaths) != 1 || cfg.ScanPaths[0] != "/explicit/path" {
		t.Errorf("ScanPaths = %v, want [/explicit/path]", cfg.ScanPaths)
	}
}

func TestEcosystemConfig_ToRules(t *testing.T) {
	ec := EcosystemConfig{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{"/skip/this"},
	}
	rules := ec.ToRules()

	if rules.Enabled != true {
		t.Error("ToRules().Enabled should be true")
	}
	if rules.MaxAgeDays != 30 {
		t.Errorf("ToRules().MaxAgeDays = %d, want 30", rules.MaxAgeDays)
	}
	if len(rules.ExcludePaths) != 1 || rules.ExcludePaths[0] != "/skip/this" {
		t.Errorf("ToRules().ExcludePaths = %v, want [/skip/this]", rules.ExcludePaths)
	}
}
