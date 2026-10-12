package fs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCalcDirSize(t *testing.T) {
	dir := t.TempDir()

	// Create files with known sizes
	writeFile(t, filepath.Join(dir, "a.txt"), 100)
	writeFile(t, filepath.Join(dir, "sub", "b.txt"), 200)
	writeFile(t, filepath.Join(dir, "sub", "c.txt"), 50)

	size, err := CalcDirSize(dir)
	if err != nil {
		t.Fatalf("CalcDirSize: %v", err)
	}
	if size != 350 {
		t.Errorf("expected 350 bytes, got %d", size)
	}
}

func TestCalcDirSize_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	size, err := CalcDirSize(dir)
	if err != nil {
		t.Fatalf("CalcDirSize: %v", err)
	}
	if size != 0 {
		t.Errorf("expected 0 bytes, got %d", size)
	}
}

func TestCalcDirSize_NonExistentDir(t *testing.T) {
	_, err := CalcDirSize("/nonexistent/path/xyz")
	if err == nil {
		t.Error("expected error for non-existent path")
	}
}

func TestCalcDirSizes_Parallel(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	writeFile(t, filepath.Join(dir1, "a.txt"), 100)
	writeFile(t, filepath.Join(dir2, "b.txt"), 200)

	ctx := context.Background()
	results, err := CalcDirSizes(ctx, []string{dir1, dir2})
	if err != nil {
		t.Fatalf("CalcDirSizes: %v", err)
	}
	if results[dir1] != 100 {
		t.Errorf("dir1: expected 100, got %d", results[dir1])
	}
	if results[dir2] != 200 {
		t.Errorf("dir2: expected 200, got %d", results[dir2])
	}
}

func TestCalcDirSizes_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dirs := make([]string, 100)
	for i := range dirs {
		dirs[i] = t.TempDir()
	}

	_, err := CalcDirSizes(ctx, dirs)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCalcDirSize_ReadOnlyDirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "readonly_mod")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	content := []byte("pkg-bytes-content")
	filePath := filepath.Join(subDir, "file.a")
	if err := os.WriteFile(filePath, content, 0444); err != nil {
		t.Fatal(err)
	}

	// Gancho de cleanup reverso antes de restringir permissões
	t.Cleanup(func() {
		_ = filepath.Walk(subDir, func(path string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(path, 0755)
			}
			return nil
		})
	})

	if err := os.Chmod(subDir, 0555); err != nil {
		t.Fatal(err)
	}

	size, err := CalcDirSize(subDir)
	if err != nil {
		t.Fatalf("CalcDirSize on read-only dir failed: %v", err)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}
}
