package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func createEmptyConfig(t *testing.T) string {
	t.Helper()
	content := `ecosystems:
  docker:
    enabled: false
  node:
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
  cpp:
    enabled: false
  flutter:
    enabled: false
  terraform:
    enabled: false
  cocoapods:
    enabled: false
  elixir:
    enabled: false
  shell:
    enabled: false
  macos:
    enabled: false
  browsers:
    enabled: false
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return cfgPath
}

func createTestWorkspace(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	appDir := filepath.Join(tmpDir, "node-app")
	nmDir := filepath.Join(appDir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "dummy.js"), []byte("console.log('test');"), 0644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := fmt.Sprintf(`scan_paths:
  - %q
ecosystems:
  node:
    enabled: true
    max_age_days: 0
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
  cpp:
    enabled: false
  flutter:
    enabled: false
  terraform:
    enabled: false
  cocoapods:
    enabled: false
  elixir:
    enabled: false
  shell:
    enabled: false
  macos:
    enabled: false
  browsers:
    enabled: false
`, tmpDir)
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}
	return tmpDir, cfgPath
}

func TestScanCmd_NoTargets_ReturnsError(t *testing.T) {
	cfgFile := createEmptyConfig(t)
	origConfigPath := configPath
	configPath = cfgFile
	defer func() { configPath = origConfigPath }()

	cmd := newScanCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "no targets found" {
		t.Fatalf("expected 'no targets found', got %q", err.Error())
	}
}

func TestScanCmd_InvalidConfig(t *testing.T) {
	origConfigPath := configPath
	configPath = "/invalid/path.yaml"
	defer func() { configPath = origConfigPath }()

	cmd := newScanCmd()
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for invalid config")
	}
}


func TestScanCmd_FlagsValidation(t *testing.T) {
	cmd := newScanCmd()
	cmd.SetArgs([]string{"--ecosystem", "invalid-ecosystem"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid ecosystem, got nil")
	}
	if !strings.Contains(err.Error(), "unknown ecosystem") {
		t.Fatalf("expected unknown ecosystem error, got %v", err)
	}
}

func TestScanCmd_JSONFormatOutput(t *testing.T) {
	_, cfgPath := createTestWorkspace(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	scanCmd := newScanCmd()
	scanCmd.SetArgs([]string{"--format", "json"})
	var out bytes.Buffer
	scanCmd.SetOut(&out)

	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan --format json failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, `"ecosystem": "node"`) && !strings.Contains(outStr, "node_modules") {
		t.Errorf("JSON output did not contain expected targets: %s", outStr)
	}
}

func TestScanCmd_TableFormatOutput(t *testing.T) {
	_, cfgPath := createTestWorkspace(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	scanCmd := newScanCmd()
	scanCmd.SetArgs([]string{"--format", "table"})
	var out bytes.Buffer
	scanCmd.SetOut(&out)

	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan --format table failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "ECOSYSTEM") || !strings.Contains(outStr, "node") {
		t.Errorf("table output did not contain expected ecosystem headers: %s", outStr)
	}
}

func TestScanCmd_FiltersAndSmart(t *testing.T) {
	_, cfgPath := createTestWorkspace(t)
	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	scanCmd := newScanCmd()
	scanCmd.SetArgs([]string{"--smart", "--all", "--ecosystem", "node"})
	var out bytes.Buffer
	scanCmd.SetOut(&out)

	if err := scanCmd.Execute(); err != nil {
		t.Fatalf("scan --smart --all --ecosystem node failed: %v", err)
	}

	outStr := out.String()
	if !strings.Contains(outStr, "SCORE") {
		t.Errorf("expected smart score header in output, got: %s", outStr)
	}
}

func TestScanCmd_Helpers(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{ID: "n1", Path: "/a/node_modules", SizeBytes: 100, LastAccessed: time.Now()},
			},
		},
		{
			Ecosystem: "go",
			Targets: []domain.Target{
				{ID: "g1", Path: "/b/mod", SizeBytes: 200, LastAccessed: time.Now()},
			},
		},
	}

	// 1. totalTargets
	if totalTargets(results) != 2 {
		t.Errorf("totalTargets = %d, want 2", totalTargets(results))
	}

	// 2. filterByEcosystem
	filteredNode := filterByEcosystem(results, "node")
	if len(filteredNode) != 1 || filteredNode[0].Ecosystem != "node" {
		t.Errorf("filterByEcosystem(node) unexpected result: %+v", filteredNode)
	}

	filteredNone := filterByEcosystem(results, "docker")
	if len(filteredNone) != 0 {
		t.Errorf("filterByEcosystem(docker) should be empty, got %d", len(filteredNone))
	}

	// 3. validateEcosystem
	valid := []string{"node", "go", "docker"}
	if err := validateEcosystem("NODE", valid); err != nil {
		t.Errorf("validateEcosystem case-insensitive match failed: %v", err)
	}
	if err := validateEcosystem("unknown", valid); err == nil {
		t.Error("validateEcosystem should fail for unknown ecosystem")
	}
}

func TestScanCmd_NewEcosystemsValidation(t *testing.T) {
	newEcos := []string{"ai", "pnpm", "bun", "nextjs", "xcode", "homebrew"}
	for _, eco := range newEcos {
		t.Run(eco, func(t *testing.T) {
			cfgFile := createEmptyConfig(t)
			origConfigPath := configPath
			configPath = cfgFile
			defer func() { configPath = origConfigPath }()

			cmd := newScanCmd()
			cmd.SetArgs([]string{"-e", eco})
			err := cmd.Execute()
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if strings.Contains(err.Error(), "unknown ecosystem") {
				t.Fatalf("unexpected unknown ecosystem error for %s: %v", eco, err)
			}
			if err.Error() != "no targets found" {
				t.Fatalf("expected 'no targets found', got %v", err)
			}
		})
	}

	t.Run("invalid ecosystem", func(t *testing.T) {
		cmd := newScanCmd()
		cmd.SetArgs([]string{"-e", "invalid-eco-name"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "unknown ecosystem") {
			t.Fatalf("expected unknown ecosystem error, got %v", err)
		}
	})
}

