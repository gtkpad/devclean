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
	if cfg.Ecosystems.CPP.MaxAgeDays != 14 {
		t.Errorf("CPP.MaxAgeDays = %d, want 14", cfg.Ecosystems.CPP.MaxAgeDays)
	}
	if cfg.Ecosystems.Flutter.MaxAgeDays != 14 {
		t.Errorf("Flutter.MaxAgeDays = %d, want 14", cfg.Ecosystems.Flutter.MaxAgeDays)
	}
	if cfg.Ecosystems.Terraform.MaxAgeDays != 30 {
		t.Errorf("Terraform.MaxAgeDays = %d, want 30", cfg.Ecosystems.Terraform.MaxAgeDays)
	}
	if cfg.Ecosystems.CocoaPods.MaxAgeDays != 14 {
		t.Errorf("CocoaPods.MaxAgeDays = %d, want 14", cfg.Ecosystems.CocoaPods.MaxAgeDays)
	}
	if cfg.Ecosystems.Elixir.MaxAgeDays != 14 {
		t.Errorf("Elixir.MaxAgeDays = %d, want 14", cfg.Ecosystems.Elixir.MaxAgeDays)
	}
	if cfg.Hooks.NextJS.Timeout != "30s" {
		t.Errorf("Hooks.NextJS.Timeout = %q, want 30s", cfg.Hooks.NextJS.Timeout)
	}
	if cfg.Hooks.AI.Timeout != "30s" {
		t.Errorf("Hooks.AI.Timeout = %q, want 30s", cfg.Hooks.AI.Timeout)
	}
	if cfg.Hooks.CPP.Timeout != "30s" {
		t.Errorf("Hooks.CPP.Timeout = %q, want 30s", cfg.Hooks.CPP.Timeout)
	}
	if cfg.Hooks.Flutter.Timeout != "30s" {
		t.Errorf("Hooks.Flutter.Timeout = %q, want 30s", cfg.Hooks.Flutter.Timeout)
	}
	if cfg.Hooks.Terraform.Timeout != "30s" {
		t.Errorf("Hooks.Terraform.Timeout = %q, want 30s", cfg.Hooks.Terraform.Timeout)
	}
	if cfg.Hooks.CocoaPods.Timeout != "30s" {
		t.Errorf("Hooks.CocoaPods.Timeout = %q, want 30s", cfg.Hooks.CocoaPods.Timeout)
	}
	if cfg.Hooks.Elixir.Timeout != "30s" {
		t.Errorf("Hooks.Elixir.Timeout = %q, want 30s", cfg.Hooks.Elixir.Timeout)
	}
	if !cfg.Ecosystems.Docker.Enabled {
		t.Error("Docker.Enabled should be true by default")
	}
	if !cfg.Ecosystems.CPP.Enabled || !cfg.Ecosystems.Flutter.Enabled || !cfg.Ecosystems.Terraform.Enabled || !cfg.Ecosystems.CocoaPods.Enabled || !cfg.Ecosystems.Elixir.Enabled {
		t.Error("New ecosystems should be enabled by default")
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
		{"CPP", d.Ecosystems.CPP, 14},
		{"Flutter", d.Ecosystems.Flutter, 14},
		{"Terraform", d.Ecosystems.Terraform, 30},
		{"CocoaPods", d.Ecosystems.CocoaPods, 14},
		{"Elixir", d.Ecosystems.Elixir, 14},
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

func TestLoad_InstallerDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Installer.MaxDepth != 2 {
		t.Errorf("Installer.MaxDepth = %d, want 2", cfg.Installer.MaxDepth)
	}
	if len(cfg.Installer.Paths) != 2 || cfg.Installer.Paths[0] != "~/Downloads" || cfg.Installer.Paths[1] != "~/Desktop" {
		t.Errorf("Installer.Paths = %v, want [~/Downloads ~/Desktop]", cfg.Installer.Paths)
	}
}

func TestLoad_InstallerCustomConfig(t *testing.T) {
	yaml := `
installer:
  paths:
    - /custom/installers
  max_depth: 3
`
	path := writeTestConfig(t, yaml)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Installer.MaxDepth != 3 {
		t.Errorf("Installer.MaxDepth = %d, want 3", cfg.Installer.MaxDepth)
	}
	if len(cfg.Installer.Paths) != 1 || cfg.Installer.Paths[0] != "/custom/installers" {
		t.Errorf("Installer.Paths = %v, want [/custom/installers]", cfg.Installer.Paths)
	}
}

func TestValidate_InstallerMaxDepth(t *testing.T) {
	cfg := Defaults()
	cfg.Installer.MaxDepth = 6
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxDepth > 5, got nil")
	}

	cfg.Installer.MaxDepth = -1
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxDepth < 0, got nil")
	}

	cfg.Installer.MaxDepth = 2
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected nil error for valid MaxDepth, got %v", err)
	}
}

func TestConfig_DefaultsV22(t *testing.T) {
	d := Defaults()
	if !d.Safety.TrashEnabled {
		t.Errorf("Safety.TrashEnabled = false, want true for v2.2")
	}
	if d.Safety.RetentionDays != 30 {
		t.Errorf("Safety.RetentionDays = %d, want 30", d.Safety.RetentionDays)
	}
	if d.Safety.MaxSizeMB != 10240 {
		t.Errorf("Safety.MaxSizeMB = %d, want 10240", d.Safety.MaxSizeMB)
	}
}

func TestValidate_SafetyRules(t *testing.T) {
	cfg := Defaults()
	cfg.Safety.RetentionDays = 0
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for RetentionDays < 1, got nil")
	}

	cfg = Defaults()
	cfg.Safety.MaxSizeMB = 50
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for MaxSizeMB < 100, got nil")
	}

	cfg = Defaults()
	cfg.Safety.RetentionDays = 30
	cfg.Safety.MaxSizeMB = 10240
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected nil error for valid SafetyConfig, got %v", err)
	}
}

func TestSchedulerConfig_Defaults(t *testing.T) {
	d := Defaults()
	if d.Scheduler.Args == nil {
		t.Fatal("expected Scheduler.Args to be non-nil empty slice, got nil")
	}
	if len(d.Scheduler.Args) != 0 {
		t.Errorf("expected empty slice, got %v", d.Scheduler.Args)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") error: %v", err)
	}
	if cfg.Scheduler.Args == nil {
		t.Fatal("expected loaded default Scheduler.Args to be non-nil slice")
	}
	if len(cfg.Scheduler.Args) != 0 {
		t.Errorf("expected empty slice from Load(\"\"), got %v", cfg.Scheduler.Args)
	}
}

func TestSchedulerConfig_ValidationValid(t *testing.T) {
	cfg := Defaults()
	cfg.Scheduler.Args = []string{"--preset", "safe", "--dry-run"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid scheduler args, got error: %v", err)
	}

	cfg.Scheduler.Args = []string{"--preset=standard", "--smart", "--trash"}
	if err := cfg.Validate(); err != nil {
		t.Errorf("expected valid scheduler args, got error: %v", err)
	}
}

func TestSchedulerConfig_ValidationInvalid(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"semicolon injection", []string{"--preset", "safe; rm -rf /"}},
		{"pipe injection", []string{"--preset", "safe | cat"}},
		{"ampersand injection", []string{"--preset", "safe & echo pwn"}},
		{"variable expansion", []string{"--preset", "$SECRET"}},
		{"backtick injection", []string{"--preset", "`id`"}},
		{"redirection", []string{"--preset", "safe > /dev/null"}},
		{"newline injection", []string{"--preset", "safe\nrm -rf /"}},
		{"unknown flag", []string{"--malicious-flag"}},
		{"arbitrary command", []string{"curl", "http://attacker.com"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Scheduler.Args = tc.args
			if err := cfg.Validate(); err == nil {
				t.Errorf("expected validation error for args %v, got nil", tc.args)
			}
		})
	}
}

