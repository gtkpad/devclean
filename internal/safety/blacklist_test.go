package safety

import (
	"os"
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

func TestCoreBlacklist(t *testing.T) {
	items := CoreBlacklist()
	expected := []string{".git", ".env", ".ssh", "secrets.json"}

	if len(items) != len(expected) {
		t.Fatalf("CoreBlacklist() len = %d, want %d", len(items), len(expected))
	}

	for i, v := range expected {
		if items[i] != v {
			t.Errorf("CoreBlacklist()[%d] = %q, want %q", i, items[i], v)
		}
	}

	// Verify it returns an independent copy
	items[0] = "mutated"
	items2 := CoreBlacklist()
	if items2[0] != ".git" {
		t.Errorf("CoreBlacklist() slice was mutated: items2[0] = %q, want .git", items2[0])
	}
}

func TestIsCoreBlacklisted(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".git", true},
		{".env", true},
		{".ssh", true},
		{"secrets.json", true},
		{"/Users/user/.git", true},
		{"/foo/bar/.env", true},
		{"/root/.ssh", true},
		{"/app/config/secrets.json", true},
		{"node_modules", false},
		{"/home/user/.cache", false},
		{"git", false},
		{"my-secrets.json", false},
		// Subtree nested inside protected folders
		{"/repo/.git/config", true},
		{"/repo/.git/objects/pack/pack-1.pack", true},
		{"/home/user/.ssh/id_rsa", true},
		{"/home/user/.ssh/authorized_keys", true},
		{"/app/.env/nested.txt", true},
		{"/app/subdir/secrets.json", true},
		// System roots
		{"/", true},
		{"/bin", true},
		{"/boot", true},
		{"/dev", true},
		{"/etc", true},
		{"/lib", true},
		{"/proc", true},
		{"/root", true},
		{"/sys", true},
		{"/usr", true},
		{"/var", true},
		{"/System", true},
		{"/Library", true},
		{"/Applications", true},
		{"/private", true},
		// Safe non-blacklisted subpaths
		{"/var/tmp/build", false},
		{"/var/folders/xyz/temp", false},
	}

	for _, tt := range tests {
		if got := IsCoreBlacklisted(tt.path); got != tt.want {
			t.Errorf("IsCoreBlacklisted(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

func TestSystemRoots(t *testing.T) {
	roots := SystemRoots()
	if len(roots) == 0 {
		t.Fatal("expected non-empty SystemRoots")
	}
	roots[0] = "mutated"
	roots2 := SystemRoots()
	if roots2[0] == "mutated" {
		t.Error("SystemRoots returned slice should be a copy, not mutated")
	}
}

func TestBlacklist_SymlinkEvaluation(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create a real .git folder and a symlink to it
	gitDir := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkToGit := filepath.Join(tempDir, "link_to_git")
	if err := os.Symlink(gitDir, symlinkToGit); err != nil {
		t.Fatal(err)
	}

	// 2. Create a safe folder and a symlink to it
	safeDir := filepath.Join(tempDir, "safe_dir")
	if err := os.MkdirAll(safeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkToSafe := filepath.Join(tempDir, "link_to_safe")
	if err := os.Symlink(safeDir, symlinkToSafe); err != nil {
		t.Fatal(err)
	}

	// 3. Create a custom protected directory and a symlink to it
	customDir := filepath.Join(tempDir, "my_custom_secret")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkToCustom := filepath.Join(tempDir, "link_to_custom")
	if err := os.Symlink(customDir, symlinkToCustom); err != nil {
		t.Fatal(err)
	}

	bl := NewBlacklist([]string{"*my_custom_secret*"})

	// Canonical symlink to .git must be blacklisted (POL-04, D-16)
	if !bl.IsBlacklisted(symlinkToGit) {
		t.Errorf("bl.IsBlacklisted(symlinkToGit) = false, want true")
	}
	if !IsCoreBlacklisted(symlinkToGit) {
		t.Errorf("IsCoreBlacklisted(symlinkToGit) = false, want true")
	}

	// Canonical symlink to custom protected directory must be blacklisted
	if !bl.IsBlacklisted(symlinkToCustom) {
		t.Errorf("bl.IsBlacklisted(symlinkToCustom) = false, want true")
	}

	// Symlink to safe directory must NOT be blacklisted
	if bl.IsBlacklisted(symlinkToSafe) {
		t.Errorf("bl.IsBlacklisted(symlinkToSafe) = true, want false")
	}
	if IsCoreBlacklisted(symlinkToSafe) {
		t.Errorf("IsCoreBlacklisted(symlinkToSafe) = true, want false")
	}
}


