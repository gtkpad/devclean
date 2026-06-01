package safety

import (
	"path/filepath"
	"testing"
)

func TestBlacklist_CoreItems(t *testing.T) {
	bl := NewBlacklist(nil)

	tests := []struct {
		path string
		want bool
	}{
		{".git", true},
		{".env", true},
		{".ssh", true},
		{"secrets.json", true},
		{"/home/user/project/.git", true},
		{"/home/user/.ssh", true},
		{"/projects/.env", true},
		{"/deep/nested/path/secrets.json", true},
	}

	for _, tt := range tests {
		if got := bl.IsBlacklisted(tt.path); got != tt.want {
			t.Errorf("IsBlacklisted(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBlacklist_NonCoreNotBlocked(t *testing.T) {
	bl := NewBlacklist(nil)

	paths := []string{
		"node_modules",
		".cache",
		"/home/user/.cache",
		"go-build",
		"package-lock.json",
	}

	for _, p := range paths {
		if bl.IsBlacklisted(p) {
			t.Errorf("IsBlacklisted(%q) = true, want false (not a core item)", p)
		}
	}
}

func TestBlacklist_UserPatterns(t *testing.T) {
	bl := NewBlacklist([]string{"*.key", "credentials.*"})

	tests := []struct {
		path string
		want bool
	}{
		{"id_rsa.key", true},
		{"server.key", true},
		{"credentials.json", true},
		{"credentials.yaml", true},
		{"readme.md", false},
		{"main.go", false},
	}

	for _, tt := range tests {
		if got := bl.IsBlacklisted(tt.path); got != tt.want {
			t.Errorf("IsBlacklisted(%q) with patterns = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestBlacklist_EmptyPatterns(t *testing.T) {
	bl := NewBlacklist([]string{})

	// Only core should match
	if !bl.IsBlacklisted(".git") {
		t.Error("IsBlacklisted(.git) should be true even with empty patterns")
	}
	if bl.IsBlacklisted("random.txt") {
		t.Error("IsBlacklisted(random.txt) should be false with empty patterns")
	}
}

func TestBlacklist_CoreAtSubdirectoryDepth(t *testing.T) {
	bl := NewBlacklist(nil)

	// filepath.Base extracts the last component
	path := filepath.Join("/projects", "myapp", ".git")
	if !bl.IsBlacklisted(path) {
		t.Errorf("IsBlacklisted(%q) = false, want true (core at depth)", path)
	}
}
