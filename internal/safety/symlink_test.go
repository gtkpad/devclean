package safety

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSymlink_RegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "regular.txt")
	if err := os.WriteFile(file, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	isLink, err := IsSymlink(file)
	if err != nil {
		t.Fatalf("IsSymlink(%q) error: %v", file, err)
	}
	if isLink {
		t.Errorf("IsSymlink(%q) = true, want false (regular file)", file)
	}
}

func TestIsSymlink_RegularDirectory(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	isLink, err := IsSymlink(subdir)
	if err != nil {
		t.Fatalf("IsSymlink(%q) error: %v", subdir, err)
	}
	if isLink {
		t.Errorf("IsSymlink(%q) = true, want false (regular dir)", subdir)
	}
}

func TestIsSymlink_SymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	isLink, err := IsSymlink(link)
	if err != nil {
		t.Fatalf("IsSymlink(%q) error: %v", link, err)
	}
	if !isLink {
		t.Errorf("IsSymlink(%q) = false, want true (symlink to file)", link)
	}
}

func TestIsSymlink_SymlinkToDirectory(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "targetdir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "linkdir")
	if err := os.Symlink(targetDir, link); err != nil {
		t.Fatal(err)
	}

	isLink, err := IsSymlink(link)
	if err != nil {
		t.Fatalf("IsSymlink(%q) error: %v", link, err)
	}
	if !isLink {
		t.Errorf("IsSymlink(%q) = false, want true (symlink to dir)", link)
	}
}

func TestIsSymlink_NonExistentPath(t *testing.T) {
	_, err := IsSymlink("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("IsSymlink on non-existent path should return error")
	}
}

func TestSymlink_CyclicAndBlacklistedTarget(t *testing.T) {
	dir := t.TempDir()
	cyclicLink := filepath.Join(dir, "loop")
	if err := os.Symlink(dir, cyclicLink); err != nil {
		t.Fatal(err)
	}

	// IsSymlink deve identificar corretamente sem entrar em recursão infinita
	isLink, err := IsSymlink(cyclicLink)
	if err != nil || !isLink {
		t.Fatalf("expected IsSymlink = true on cyclic link, got %v, err: %v", isLink, err)
	}

	// Link apontando para .git na blacklist
	gitReal := filepath.Join(dir, ".git")
	if err := os.Mkdir(gitReal, 0755); err != nil {
		t.Fatal(err)
	}
	gitLink := filepath.Join(dir, "git-link")
	if err := os.Symlink(gitReal, gitLink); err != nil {
		t.Fatal(err)
	}

	bl := NewBlacklist(nil)
	resolved, err := filepath.EvalSymlinks(gitLink)
	if err != nil {
		t.Fatal(err)
	}
	if !bl.IsBlacklisted(resolved) {
		t.Errorf("symlink target %s should be recognized as blacklisted", resolved)
	}
}
