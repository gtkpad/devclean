package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
`
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return cfgPath
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
