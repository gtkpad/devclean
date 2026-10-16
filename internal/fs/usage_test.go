package fs_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/fs"
)

func TestCalcUsage_SingleFile_Sparse(t *testing.T) {
	tmpDir := t.TempDir()
	sparseFile := filepath.Join(tmpDir, "sparse.raw")

	f, err := os.Create(sparseFile)
	if err != nil {
		t.Fatalf("os.Create failed: %v", err)
	}

	payload := []byte("hello sparse file header")
	if _, err := f.Write(payload); err != nil {
		f.Close()
		t.Fatalf("f.Write failed: %v", err)
	}

	const apparentSize int64 = 10 * 1024 * 1024 // 10 MB
	if err := f.Truncate(apparentSize); err != nil {
		f.Close()
		t.Fatalf("f.Truncate failed: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("f.Close failed: %v", err)
	}

	usage, err := fs.CalcUsage(sparseFile)
	if err != nil {
		t.Fatalf("fs.CalcUsage failed: %v", err)
	}

	if usage.ApparentBytes != apparentSize {
		t.Errorf("ApparentBytes = %d, want %d", usage.ApparentBytes, apparentSize)
	}
	if usage.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", usage.FileCount)
	}
	if usage.AllocatedBytes > usage.ApparentBytes {
		t.Errorf("AllocatedBytes (%d) should be <= ApparentBytes (%d)", usage.AllocatedBytes, usage.ApparentBytes)
	}
	if usage.AllocatedBytes < 0 {
		t.Errorf("AllocatedBytes (%d) must be non-negative", usage.AllocatedBytes)
	}
	if usage.Partial {
		t.Errorf("Partial = true, want false")
	}
}

func TestCalcUsage_Hardlinks_Deduplication(t *testing.T) {
	tmpDir := t.TempDir()
	origFile := filepath.Join(tmpDir, "original.dat")

	// 4096 bytes data
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(origFile, data, 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Create 3 hardlinks pointing to origFile
	for i := 1; i <= 3; i++ {
		linkPath := filepath.Join(tmpDir, filepath.Base(origFile)+".link"+string(rune('0'+i)))
		if err := os.Link(origFile, linkPath); err != nil {
			t.Fatalf("os.Link failed: %v", err)
		}
	}

	usage, err := fs.CalcUsage(tmpDir)
	if err != nil {
		t.Fatalf("CalcUsage failed: %v", err)
	}

	// 4 directory entries referencing the same inode must be counted as 1 file
	if usage.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (hardlink deduplication)", usage.FileCount)
	}
	if usage.ApparentBytes != 4096 {
		t.Errorf("ApparentBytes = %d, want 4096", usage.ApparentBytes)
	}
	if usage.AllocatedBytes > 4096*2 { // Allows block padding but rejects 4x duplication
		t.Errorf("AllocatedBytes = %d, expected ~4096 (deduplicated)", usage.AllocatedBytes)
	}
	if usage.Partial {
		t.Errorf("Partial = true, want false")
	}
}

func TestCalcUsage_Symlinks_Skipped(t *testing.T) {
	extDir := t.TempDir()
	extFile := filepath.Join(extDir, "external.bin")
	if err := os.WriteFile(extFile, make([]byte, 8192), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	targetDir := t.TempDir()
	// Create symlink to external file
	symlinkToFile := filepath.Join(targetDir, "link-to-file")
	if err := os.Symlink(extFile, symlinkToFile); err != nil {
		t.Fatalf("Symlink file failed: %v", err)
	}

	// Create symlink to external dir
	symlinkToDir := filepath.Join(targetDir, "link-to-dir")
	if err := os.Symlink(extDir, symlinkToDir); err != nil {
		t.Fatalf("Symlink dir failed: %v", err)
	}

	usage, err := fs.CalcUsage(targetDir)
	if err != nil {
		t.Fatalf("CalcUsage failed: %v", err)
	}

	// Symlinks must be skipped: 0 files counted, 0 bytes
	if usage.FileCount != 0 {
		t.Errorf("FileCount = %d, want 0 (symlinks skipped)", usage.FileCount)
	}
	if usage.ApparentBytes != 0 {
		t.Errorf("ApparentBytes = %d, want 0", usage.ApparentBytes)
	}
	if usage.AllocatedBytes != 0 {
		t.Errorf("AllocatedBytes = %d, want 0", usage.AllocatedBytes)
	}
	if usage.Partial {
		t.Errorf("Partial = true, want false")
	}
}

func TestCalcUsage_PartialLock_FailClosed(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping chmod permission test on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("skipping chmod permission test when running as root")
	}

	tmpDir := t.TempDir()
	// Accessible file
	okFile := filepath.Join(tmpDir, "accessible.txt")
	if err := os.WriteFile(okFile, make([]byte, 1024), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Inaccessible subdirectory
	subDir := filepath.Join(tmpDir, "restricted")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Mkdir failed: %v", err)
	}
	subFile := filepath.Join(subDir, "hidden.txt")
	if err := os.WriteFile(subFile, make([]byte, 2048), 0644); err != nil {
		t.Fatalf("WriteFile in subDir failed: %v", err)
	}

	// Restrict permissions on subDir
	if err := os.Chmod(subDir, 0000); err != nil {
		t.Fatalf("Chmod failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(subDir, 0755)
	})

	usage, err := fs.CalcUsage(tmpDir)
	// Must not return fatal error; must degrade gracefully with Partial = true
	if err != nil {
		t.Fatalf("CalcUsage returned fatal error: %v", err)
	}
	if !usage.Partial {
		t.Errorf("usage.Partial = false, want true")
	}
	if usage.StatusNote == "" {
		t.Errorf("usage.StatusNote is empty, expected descriptive permission warning")
	}
	if usage.ApparentBytes < 1024 {
		t.Errorf("usage.ApparentBytes = %d, expected at least 1024 from accessible file", usage.ApparentBytes)
	}
}

func TestCalcUsage_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "data.bin")
	if err := os.WriteFile(testFile, make([]byte, 1024), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := fs.CalcUsageWithContext(ctx, tmpDir)
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
