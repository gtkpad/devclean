package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadProfile_Valid(t *testing.T) {
	dir := t.TempDir()
	profileDir := filepath.Join(dir, "profiles")
	os.MkdirAll(profileDir, 0o755)

	content := []byte(`
scan_paths:
  - /tmp/extra
ecosystems:
  node:
    max_age_days: 7
`)
	os.WriteFile(filepath.Join(profileDir, "ci.yaml"), content, 0o644)

	cfg, err := LoadProfile(dir, "ci")
	if err != nil {
		t.Fatalf("LoadProfile failed: %v", err)
	}
	if len(cfg.ScanPaths) != 1 || cfg.ScanPaths[0] != "/tmp/extra" {
		t.Errorf("expected scan_paths=[/tmp/extra], got %v", cfg.ScanPaths)
	}
	if cfg.Ecosystems.Node.MaxAgeDays != 7 {
		t.Errorf("expected node.max_age_days=7, got %d", cfg.Ecosystems.Node.MaxAgeDays)
	}
}

func TestLoadProfile_InvalidName(t *testing.T) {
	cases := []string{"../../etc", "foo/bar", "a b", "test.yaml", ""}
	for _, name := range cases {
		_, err := LoadProfile("/tmp", name)
		if err == nil {
			t.Errorf("expected error for profile name %q", name)
		}
	}
}

func TestLoadProfile_NotFound(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "profiles"), 0o755)
	_, err := LoadProfile(dir, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent profile")
	}
}

func TestMergeProfile_ArrayAppend(t *testing.T) {
	root := &Config{
		ScanPaths:         []string{"/a"},
		ProtectedPatterns: []string{"*.lock"},
	}
	profile := &Config{
		ScanPaths:         []string{"/b", "/a"}, // /a already in root, should dedup
		ProtectedPatterns: []string{"*.env"},
	}

	merged := MergeProfile(root, profile)

	if len(merged.ScanPaths) != 2 {
		t.Errorf("expected 2 scan_paths after dedup, got %d: %v", len(merged.ScanPaths), merged.ScanPaths)
	}
	if len(merged.ProtectedPatterns) != 2 {
		t.Errorf("expected 2 protected_patterns, got %d", len(merged.ProtectedPatterns))
	}
}

func TestMergeProfile_ScalarReplace(t *testing.T) {
	root := &Config{
		History: HistoryConfig{RetentionDays: 90},
	}
	profile := &Config{
		History: HistoryConfig{RetentionDays: 30},
	}

	merged := MergeProfile(root, profile)
	if merged.History.RetentionDays != 30 {
		t.Errorf("expected history.retention_days=30, got %d", merged.History.RetentionDays)
	}
}
