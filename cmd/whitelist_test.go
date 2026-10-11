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

func TestWhitelistAdd_PathWithTildeExpansion(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `scan_paths:
  - "/test"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("getting home dir: %v", err)
	}

	addCmd := newWhitelistAddCmd()
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	addCmd.SetArgs([]string{"~/projects/my-app"})

	if err := addCmd.Execute(); err != nil {
		t.Fatalf("whitelist add with tilde failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	expectedPath := filepath.Clean(filepath.Join(home, "projects", "my-app"))
	found := false
	for _, p := range cfg.ExcludePaths {
		if p == expectedPath {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected exclude_paths to contain %q, got: %v", expectedPath, cfg.ExcludePaths)
	}
}

func TestWhitelistAdd_RelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	addCmd := newWhitelistAddCmd()
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	addCmd.SetArgs([]string{"./relative/test/path"})

	if err := addCmd.Execute(); err != nil {
		t.Fatalf("whitelist add relative path failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	absExpected, err := filepath.Abs("./relative/test/path")
	if err != nil {
		t.Fatal(err)
	}
	absExpected = filepath.Clean(absExpected)

	found := false
	for _, p := range cfg.ExcludePaths {
		if p == absExpected {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected exclude_paths to contain %q, got: %v", absExpected, cfg.ExcludePaths)
	}
}

func TestWhitelistAdd_Pattern(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// Valid glob pattern
	addCmd := newWhitelistAddCmd()
	var buf bytes.Buffer
	addCmd.SetOut(&buf)
	addCmd.SetArgs([]string{"--pattern", "*.bak"})

	if err := addCmd.Execute(); err != nil {
		t.Fatalf("whitelist add --pattern failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	found := false
	for _, pat := range cfg.ProtectedPatterns {
		if pat == "*.bak" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected protected_patterns to contain '*.bak', got: %v", cfg.ProtectedPatterns)
	}

	// Invalid glob pattern should fail
	invalidCmd := newWhitelistAddCmd()
	invalidCmd.SetArgs([]string{"--pattern", "[abc"})
	if err := invalidCmd.Execute(); err == nil {
		t.Error("expected error when adding invalid glob pattern '[abc', got nil")
	}
}

func TestWhitelistAdd_IdempotentAndPreservesSections(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `safety:
  retention_days: 42
exclude_paths:
  - "/existing/path"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	// Add an existing path again
	addCmd1 := newWhitelistAddCmd()
	var buf1 bytes.Buffer
	addCmd1.SetOut(&buf1)
	addCmd1.SetArgs([]string{"/existing/path"})
	if err := addCmd1.Execute(); err != nil {
		t.Fatalf("first add failed: %v", err)
	}

	if !strings.Contains(buf1.String(), "already in exclude_paths") {
		t.Errorf("expected idempotency message, got: %s", buf1.String())
	}

	// Add a new path twice
	addCmd2 := newWhitelistAddCmd()
	addCmd2.SetArgs([]string{"/new/unique/path"})
	if err := addCmd2.Execute(); err != nil {
		t.Fatalf("second add failed: %v", err)
	}

	addCmd3 := newWhitelistAddCmd()
	addCmd3.SetArgs([]string{"/new/unique/path"})
	if err := addCmd3.Execute(); err != nil {
		t.Fatalf("third add failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	// Check count of /new/unique/path
	count := 0
	for _, p := range cfg.ExcludePaths {
		if p == "/new/unique/path" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected /new/unique/path exactly once, found %d times in %v", count, cfg.ExcludePaths)
	}

	// Verify safety section is preserved intact
	if cfg.Safety.RetentionDays != 42 {
		t.Errorf("expected safety.retention_days to be preserved as 42, got %d", cfg.Safety.RetentionDays)
	}
}

func TestWhitelistAdd_MissingArgs(t *testing.T) {
	addCmd := newWhitelistAddCmd()
	addCmd.SetArgs([]string{})
	if err := addCmd.Execute(); err == nil {
		t.Error("expected error when running whitelist add without args, got nil")
	}
}

func TestWhitelist_BlockImmutable(t *testing.T) {
	// Test 1: exact names .git, .env, .ssh, secrets.json
	for _, item := range []string{".git", ".env", ".ssh", "secrets.json"} {
		cmd := newWhitelistRemoveCmd()
		cmd.SetArgs([]string{item})
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("expected error removing immutable item %q, got nil", item)
		}
		expectedErr := fmt.Sprintf("cannot remove %q: system blacklist is immutable (NFR-03)", item)
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("expected error %q, got: %v", expectedErr, err)
		}
	}

	// Test 2: deep paths with immutable items
	deepPaths := []struct {
		path     string
		expected string
	}{
		{"/qualquer/path/.git", ".git"},
		{"/home/user/repo/.git", ".git"},
		{"/home/user/.ssh", ".ssh"},
		{"/app/.env", ".env"},
		{"/app/secrets.json", "secrets.json"},
		{"~/.ssh", ".ssh"},
	}

	for _, dp := range deepPaths {
		cmd := newWhitelistRemoveCmd()
		cmd.SetArgs([]string{dp.path})
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("expected error removing immutable deep path %q, got nil", dp.path)
		}
		expectedErr := fmt.Sprintf("cannot remove %q: system blacklist is immutable (NFR-03)", dp.expected)
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("expected error %q for %q, got: %v", expectedErr, dp.path, err)
		}
	}

	// Test 3: via --pattern flag
	for _, item := range []string{".git", ".env", ".ssh", "secrets.json"} {
		cmd := newWhitelistRemoveCmd()
		cmd.SetArgs([]string{"--pattern", item})
		err := cmd.Execute()
		if err == nil {
			t.Fatalf("expected error removing immutable item via --pattern %q, got nil", item)
		}
		expectedErr := fmt.Sprintf("cannot remove %q: system blacklist is immutable (NFR-03)", item)
		if !strings.Contains(err.Error(), expectedErr) {
			t.Errorf("expected error %q, got: %v", expectedErr, err)
		}
	}
}

func TestWhitelistRemove_ExcludePath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `exclude_paths:
  - "/path/to/keep"
  - "/path/to/remove"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	removeCmd := newWhitelistRemoveCmd()
	var buf bytes.Buffer
	removeCmd.SetOut(&buf)
	removeCmd.SetArgs([]string{"/path/to/remove"})

	if err := removeCmd.Execute(); err != nil {
		t.Fatalf("whitelist remove failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	for _, p := range cfg.ExcludePaths {
		if p == "/path/to/remove" {
			t.Errorf("expected /path/to/remove to be removed, but still present in %v", cfg.ExcludePaths)
		}
	}

	foundKeep := false
	for _, p := range cfg.ExcludePaths {
		if p == "/path/to/keep" {
			foundKeep = true
			break
		}
	}
	if !foundKeep {
		t.Errorf("expected /path/to/keep to remain, but missing from %v", cfg.ExcludePaths)
	}
}

func TestWhitelistRemove_Pattern(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `protected_patterns:
  - "*.keep"
  - "*.remove"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	removeCmd := newWhitelistRemoveCmd()
	var buf bytes.Buffer
	removeCmd.SetOut(&buf)
	removeCmd.SetArgs([]string{"--pattern", "*.remove"})

	if err := removeCmd.Execute(); err != nil {
		t.Fatalf("whitelist remove --pattern failed: %v", err)
	}

	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading updated config: %v", err)
	}

	for _, pat := range cfg.ProtectedPatterns {
		if pat == "*.remove" {
			t.Errorf("expected *.remove to be removed, but still present in %v", cfg.ProtectedPatterns)
		}
	}

	foundKeep := false
	for _, pat := range cfg.ProtectedPatterns {
		if pat == "*.keep" {
			foundKeep = true
			break
		}
	}
	if !foundKeep {
		t.Errorf("expected *.keep to remain in %v", cfg.ProtectedPatterns)
	}
}

func TestWhitelistRemove_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	cfgContent := `exclude_paths:
  - "/existing/path"
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0644); err != nil {
		t.Fatal(err)
	}

	origConfigPath := configPath
	configPath = cfgPath
	defer func() { configPath = origConfigPath }()

	removeCmd := newWhitelistRemoveCmd()
	var buf bytes.Buffer
	removeCmd.SetOut(&buf)
	removeCmd.SetArgs([]string{"/non/existent/path"})

	if err := removeCmd.Execute(); err != nil {
		t.Fatalf("whitelist remove of non-existent path should not return error, got: %v", err)
	}

	if !strings.Contains(buf.String(), "not found in whitelist") {
		t.Errorf("expected 'not found in whitelist' message, got: %s", buf.String())
	}

	// Verify YAML content was not broken
	cfg, err := loadConfigWithProfile()
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	if len(cfg.ExcludePaths) != 1 || cfg.ExcludePaths[0] != "/existing/path" {
		t.Errorf("expected exclude_paths to remain unchanged, got: %v", cfg.ExcludePaths)
	}
}

func TestWhitelistRemove_MissingArgs(t *testing.T) {
	removeCmd := newWhitelistRemoveCmd()
	removeCmd.SetArgs([]string{})
	if err := removeCmd.Execute(); err == nil {
		t.Error("expected error when running whitelist remove without args, got nil")
	}
}


