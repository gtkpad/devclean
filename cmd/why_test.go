package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHasSiblingManifest_Node(t *testing.T) {
	dir := t.TempDir()
	nodeModules := filepath.Join(dir, "node_modules")
	os.MkdirAll(nodeModules, 0o755)
	os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0o644)

	if !hasSiblingManifest(nodeModules, "node") {
		t.Error("expected sibling manifest found for node")
	}
}

func TestHasSiblingManifest_NoManifest(t *testing.T) {
	dir := t.TempDir()
	nodeModules := filepath.Join(dir, "node_modules")
	os.MkdirAll(nodeModules, 0o755)

	if hasSiblingManifest(nodeModules, "node") {
		t.Error("expected no sibling manifest without package.json")
	}
}

func TestHasSiblingManifest_Rust(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	os.MkdirAll(target, 0o755)
	os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]"), 0o644)

	if !hasSiblingManifest(target, "rust") {
		t.Error("expected sibling manifest found for rust")
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		input    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024 * 3, "3.0 MB"},
		{1024 * 1024 * 1024 * 2, "2.0 GB"},
	}
	for _, tc := range cases {
		got := formatBytes(tc.input)
		if got != tc.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
