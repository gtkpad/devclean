package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChmodRecursive_ReadOnlyTree(t *testing.T) {
	tmp := t.TempDir()
	treeRoot := filepath.Join(tmp, "readonly_tree")
	subDir1 := filepath.Join(treeRoot, "sub1")
	subDir2 := filepath.Join(subDir1, "sub2")

	if err := os.MkdirAll(subDir2, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	file1 := filepath.Join(treeRoot, "file1.txt")
	file2 := filepath.Join(subDir1, "file2.txt")
	file3 := filepath.Join(subDir2, "file3.txt")

	for _, f := range []string{file1, file2, file3} {
		if err := os.WriteFile(f, []byte("data"), 0o644); err != nil {
			t.Fatalf("failed to write file %s: %v", f, err)
		}
	}

	// Register cleanup to ensure TempDir can be cleaned up even if test fails
	t.Cleanup(func() {
		_ = filepath.Walk(treeRoot, func(p string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(p, 0o777)
			}
			return nil
		})
	})

	// Make files read-only (0444)
	for _, f := range []string{file1, file2, file3} {
		if err := os.Chmod(f, 0o444); err != nil {
			t.Fatalf("failed to chmod file %s: %v", f, err)
		}
	}

	// Make directories read-only (0555) from deepest to root
	for _, d := range []string{subDir2, subDir1, treeRoot} {
		if err := os.Chmod(d, 0o555); err != nil {
			t.Fatalf("failed to chmod dir %s: %v", d, err)
		}
	}

	// Apply ChmodRecursive
	if err := ChmodRecursive(treeRoot, 0o755, 0o666); err != nil {
		t.Fatalf("ChmodRecursive failed: %v", err)
	}

	// Verify that the tree can now be completely removed without EACCES
	if err := os.RemoveAll(treeRoot); err != nil {
		t.Fatalf("os.RemoveAll failed after ChmodRecursive: %v", err)
	}

	if _, err := os.Stat(treeRoot); !os.IsNotExist(err) {
		t.Errorf("expected treeRoot to be removed, but stat returned: %v", err)
	}
}

func TestChmodRecursive_NonExistent(t *testing.T) {
	nonExistent := filepath.Join(t.TempDir(), "does-not-exist")
	if err := ChmodRecursive(nonExistent, 0o755, 0o666); err != nil {
		t.Errorf("expected nil error for non-existent path, got: %v", err)
	}
}

func TestChmodRecursive_SingleFile(t *testing.T) {
	tmp := t.TempDir()
	filePath := filepath.Join(tmp, "single_readonly.txt")

	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := os.Chmod(filePath, 0o400); err != nil {
		t.Fatalf("failed to chmod file: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(filePath, 0o666)
	})

	if err := ChmodRecursive(filePath, 0o755, 0o666); err != nil {
		t.Fatalf("ChmodRecursive failed: %v", err)
	}

	// Check file is writable
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_APPEND, 0o666)
	if err != nil {
		t.Fatalf("failed to open file for writing after ChmodRecursive: %v", err)
	}
	f.Close()

	if err := os.Remove(filePath); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}
}

func TestChmodRecursive_Symlink(t *testing.T) {
	tmp := t.TempDir()
	targetFile := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(targetFile, []byte("target"), 0o444); err != nil {
		t.Fatalf("failed to write target file: %v", err)
	}

	dir := filepath.Join(tmp, "symlink_dir")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	linkPath := filepath.Join(dir, "link_to_target")
	if err := os.Symlink(targetFile, linkPath); err != nil {
		t.Skip("symlinks not supported in environment")
	}

	// Broken symlink
	brokenLink := filepath.Join(dir, "broken_link")
	_ = os.Symlink(filepath.Join(tmp, "nonexistent"), brokenLink)

	if err := ChmodRecursive(dir, 0o755, 0o666); err != nil {
		t.Fatalf("ChmodRecursive with symlink failed: %v", err)
	}
}

func TestChmodRecursive_RootSymlink(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0o444); err != nil {
		t.Fatalf("failed to write target: %v", err)
	}

	link := filepath.Join(tmp, "root_link")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}

	if err := ChmodRecursive(link, 0o755, 0o666); err != nil {
		t.Fatalf("ChmodRecursive on root symlink failed: %v", err)
	}
}

func TestChmodRecursive_InvalidPath(t *testing.T) {
	err := ChmodRecursive("\x00invalid-path", 0o755, 0o666)
	if err == nil {
		t.Error("expected error for path with null byte, got nil")
	}
}


