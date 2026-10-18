package engine

import (
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/config"
)

func TestRegisterDefaultCleaners(t *testing.T) {
	cfg := &config.Config{
		ScanPaths: []string{"/tmp/test-projects"},
		Ecosystems: config.EcosystemsConfig{
			Docker:    config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			Node:      config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Go:        config.EcosystemConfig{Enabled: true, MaxAgeDays: 60},
			DotNet:    config.EcosystemConfig{Enabled: true, MaxAgeDays: 45},
			Python:    config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			Rust:      config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			Java:      config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			AI:        config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			Pnpm:      config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			Bun:       config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			NextJS:    config.EcosystemConfig{Enabled: true, MaxAgeDays: 7},
			Xcode:     config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Homebrew:  config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			CPP:       config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Flutter:   config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Terraform: config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			CocoaPods: config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Elixir:    config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Shell:     config.EcosystemConfig{Enabled: true, MaxAgeDays: 30},
			MacOS:     config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Browsers:  config.EcosystemConfig{Enabled: true, MaxAgeDays: 14},
			Leftovers: config.EcosystemConfig{Enabled: true, MaxAgeDays: 0},
		},
	}

	res := &fakeResolver{}
	eng := New(cfg, res)

	RegisterDefaultCleaners(eng, cfg, res)

	wantEcosystems := []string{
		"ai", "browsers", "bun", "cocoapods", "cpp", "docker", "dotnet",
		"elixir", "flutter", "go", "homebrew", "java", "leftovers", "macos", "nextjs",
		"node", "pnpm", "python", "rust", "shell", "terraform", "xcode",
	}

	gotEcosystems := eng.Ecosystems()
	if len(gotEcosystems) != len(wantEcosystems) {
		t.Fatalf("eng.Ecosystems() count = %d, want %d (%v)", len(gotEcosystems), len(wantEcosystems), gotEcosystems)
	}

	for i, want := range wantEcosystems {
		if gotEcosystems[i] != want {
			t.Errorf("eng.Ecosystems()[%d] = %q, want %q", i, gotEcosystems[i], want)
		}
	}

	for _, name := range wantEcosystems {
		c := eng.GetCleaner(name)
		if c == nil {
			t.Errorf("GetCleaner(%q) returned nil", name)
		} else if c.Name() != name {
			t.Errorf("GetCleaner(%q).Name() = %q, want %q", name, c.Name(), name)
		}
	}

	// Test mapping of each ecosystem configuration
	tests := []struct {
		name       string
		maxAgeDays int
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
		{"shell", 30},
		{"macos", 14},
		{"browsers", 14},
		{"leftovers", 0},
	}

	for _, tc := range tests {
		ecoCfg := eng.ecosystemConfig(tc.name)
		if ecoCfg.MaxAgeDays != tc.maxAgeDays {
			t.Errorf("ecosystemConfig(%q).MaxAgeDays = %d, want %d", tc.name, ecoCfg.MaxAgeDays, tc.maxAgeDays)
		}
		if !ecoCfg.Enabled {
			t.Errorf("ecosystemConfig(%q).Enabled = false, want true", tc.name)
		}
	}

	// Unknown ecosystem fallback
	unknownCfg := eng.ecosystemConfig("unknown-eco")
	if !unknownCfg.Enabled || unknownCfg.FailMode != "continue" {
		t.Errorf("ecosystemConfig(unknown-eco) = %+v, want Enabled: true, FailMode: continue", unknownCfg)
	}
}

func TestRegisterDefaultCleanersWithExecutor(t *testing.T) {
	cfg := &config.Config{
		ScanPaths: []string{"/tmp/test"},
	}
	res := &fakeResolver{}
	eng := New(cfg, res)
	exec := &cleaner.MockCommandExecutor{}

	RegisterDefaultCleanersWithExecutor(eng, cfg, res, exec)

	if len(eng.Ecosystems()) != 22 {
		t.Fatalf("expected 22 ecosystems, got %d", len(eng.Ecosystems()))
	}
}
