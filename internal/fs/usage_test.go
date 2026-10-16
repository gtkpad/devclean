package fs_test

import (
	"os"
	"path/filepath"
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
