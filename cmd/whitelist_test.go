package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWhitelistList_WithEntries(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `exclude_paths:
  - "/custom/excluded/path"
  - "/another/path"
protected_patterns:
  - "*.secret"
  - "backup-*"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	listCmd := newWhitelistListCmd()
	var buf bytes.Buffer
	listCmd.SetOut(&buf)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("whitelist list failed: %v", err)
	}

	out := buf.String()

	// Verify exclude_paths
	if !strings.Contains(out, "Caminhos Excluídos (exclude_paths):") {
		t.Error("missing exclude_paths section header")
	}
	if !strings.Contains(out, "/custom/excluded/path") || !strings.Contains(out, "/another/path") {
		t.Errorf("missing exclude_paths entries, got:\n%s", out)
	}

	// Verify protected_patterns
	if !strings.Contains(out, "Padrões Protegidos (protected_patterns):") {
		t.Error("missing protected_patterns section header")
	}
	if !strings.Contains(out, "*.secret") || !strings.Contains(out, "backup-*") {
		t.Errorf("missing protected_patterns entries, got:\n%s", out)
	}

	// Verify immutable blacklist items and annotation
	if !strings.Contains(out, "Blacklist Imutável do Sistema:") {
		t.Error("missing immutable blacklist section header")
	}
	for _, item := range []string{".git", ".env", ".ssh", "secrets.json"} {
		expectedLine := fmt.Sprintf("• %s [imutável, somente-leitura]", item)
		if !strings.Contains(out, expectedLine) {
			t.Errorf("expected immutable item %q with annotation, got:\n%s", expectedLine, out)
		}
	}
}

func TestWhitelistList_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `exclude_paths: []
protected_patterns: []
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	listCmd := newWhitelistListCmd()
	var buf bytes.Buffer
	listCmd.SetOut(&buf)

	if err := listCmd.Execute(); err != nil {
		t.Fatalf("whitelist list failed: %v", err)
	}

	out := buf.String()

	// Both custom sections should state (nenhum)
	if strings.Count(out, "(nenhum)") < 2 {
		t.Errorf("expected at least two '(nenhum)' indicators for empty sections, got:\n%s", out)
	}

	// Immutable items should still be listed
	for _, item := range []string{".git", ".env", ".ssh", "secrets.json"} {
		expectedLine := fmt.Sprintf("• %s [imutável, somente-leitura]", item)
		if !strings.Contains(out, expectedLine) {
			t.Errorf("expected immutable item %q even when custom lists are empty, got:\n%s", expectedLine, out)
		}
	}
}
