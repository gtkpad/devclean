package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGetDiskSpace_TempDir(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("GetDiskSpace is only supported on Darwin and Linux")
	}

	dir := t.TempDir()

	space, err := GetDiskSpace(dir)
	if err != nil {
		t.Fatalf("GetDiskSpace(%q) failed: %v", dir, err)
	}

	if space.TotalBytes == 0 {
		t.Errorf("expected TotalBytes > 0, got %d", space.TotalBytes)
	}
	if space.FreeBytes == 0 {
		t.Errorf("expected FreeBytes > 0, got %d", space.FreeBytes)
	}
	if space.TotalBytes < space.FreeBytes {
		t.Errorf("TotalBytes (%d) should be >= FreeBytes (%d)", space.TotalBytes, space.FreeBytes)
	}
	if space.UsedBytes != space.TotalBytes-space.FreeBytes {
		t.Errorf("expected UsedBytes (%d) to equal TotalBytes - FreeBytes (%d)",
			space.UsedBytes, space.TotalBytes-space.FreeBytes)
	}
}

func TestGetDiskSpace_DeletedSubdir(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("GetDiskSpace is only supported on Darwin and Linux")
	}

	tmpDir := t.TempDir()
	deletedPath := filepath.Join(tmpDir, "sub", "cleaned_target", "node_modules")

	// The deletedPath does not exist on disk
	space, err := GetDiskSpace(deletedPath)
	if err != nil {
		t.Fatalf("GetDiskSpace on deleted path %q failed with ENOENT: %v", deletedPath, err)
	}

	if space.TotalBytes == 0 || space.FreeBytes == 0 {
		t.Errorf("expected valid metrics for deleted path fallback, got %+v", space)
	}
}

func TestGetDiskSpace_RegularFile(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("GetDiskSpace is only supported on Darwin and Linux")
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample_file.txt")
	if err := os.WriteFile(filePath, []byte("devclean test content"), 0644); err != nil {
		t.Fatal(err)
	}

	space, err := GetDiskSpace(filePath)
	if err != nil {
		t.Fatalf("GetDiskSpace(%q) on regular file failed: %v", filePath, err)
	}

	if space.TotalBytes == 0 || space.FreeBytes == 0 {
		t.Errorf("expected valid metrics for regular file, got %+v", space)
	}
}

func TestGetDiskSpace_CurrentDirAndEmpty(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("GetDiskSpace is only supported on Darwin and Linux")
	}

	spaceCur, err := GetDiskSpace(".")
	if err != nil {
		t.Fatalf("GetDiskSpace(\".\") failed: %v", err)
	}
	if spaceCur.TotalBytes == 0 {
		t.Error("expected non-zero TotalBytes for \".\"")
	}

	spaceEmpty, err := GetDiskSpace("")
	if err != nil {
		t.Fatalf("GetDiskSpace(\"\") failed: %v", err)
	}
	if spaceEmpty.TotalBytes == 0 {
		t.Error("expected non-zero TotalBytes for \"\"")
	}
}

func TestGetDiskSpace_RelativeNonExistent(t *testing.T) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("GetDiskSpace is only supported on Darwin and Linux")
	}

	space, err := GetDiskSpace("some_non_existent_relative_dir_xyz_123/sub")
	if err != nil {
		t.Fatalf("GetDiskSpace on relative non existent path failed: %v", err)
	}
	if space.TotalBytes == 0 {
		t.Error("expected non-zero TotalBytes for relative non existent path fallback")
	}
}
