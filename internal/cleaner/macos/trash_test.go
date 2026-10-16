package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestTrash_ScanAlwaysReviewOnly(t *testing.T) {
	tempHome := t.TempDir()
	trashDir := filepath.Join(tempHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		t.Fatalf("failed to create mock .Trash: %v", err)
	}

	// Create 2 files totaling at least 10KB
	f1 := filepath.Join(trashDir, "old_doc.pdf")
	f2 := filepath.Join(trashDir, "backup.tar")
	if err := os.WriteFile(f1, make([]byte, 5120), 0644); err != nil {
		t.Fatalf("failed to write f1: %v", err)
	}
	if err := os.WriteFile(f2, make([]byte, 5120), 0644); err != nil {
		t.Fatalf("failed to write f2: %v", err)
	}

	target := scanTrash(tempHome)

	// INVARIANT 1: Always ReviewOnly on scan (D-01)
	if !target.ReviewOnly {
		t.Errorf("expected target.ReviewOnly == true, got false")
	}

	// INVARIANT 2: EligibleBytes is always 0 on scan (D-01)
	if target.EligibleBytes != 0 {
		t.Errorf("expected target.EligibleBytes == 0, got %d", target.EligibleBytes)
	}

	// INVARIANT 3: TTLEligible is always false (D-01)
	if target.TTLEligible {
		t.Errorf("expected target.TTLEligible == false, got true")
	}

	// INVARIANT 4: ReviewReason must be "trash_requires_flag" (D-01)
	if target.ReviewReason != "trash_requires_flag" {
		t.Errorf("expected target.ReviewReason == 'trash_requires_flag', got %q", target.ReviewReason)
	}

	// Category and ID
	if target.ID != "macos-trash" {
		t.Errorf("expected ID 'macos-trash', got %q", target.ID)
	}
	if target.Name != "macOS Trash (~/.Trash)" {
		t.Errorf("expected Name 'macOS Trash (~/.Trash)', got %q", target.Name)
	}
	if target.Category != "macos-trash" {
		t.Errorf("expected Category 'macos-trash', got %q", target.Category)
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("expected Risk RiskLow, got %s", target.Risk)
	}
	if target.Path != trashDir {
		t.Errorf("expected Path %s, got %s", trashDir, target.Path)
	}

	// Disk usage should reflect the files
	if target.SizeBytes < 10240 {
		t.Errorf("expected SizeBytes >= 10240, got %d", target.SizeBytes)
	}
}

func TestTrash_CleanPurgesItemsPreservingRoot(t *testing.T) {
	tempHome := t.TempDir()
	trashDir := filepath.Join(tempHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		t.Fatalf("failed to create mock .Trash: %v", err)
	}

	// Create subfiles and subdirectories
	subDir := filepath.Join(trashDir, "old_project")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create subDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("failed to write nested file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, "image.png"), []byte("pngdata"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	ctx := context.Background()
	if err := cleanTrash(ctx, trashDir); err != nil {
		t.Fatalf("cleanTrash failed: %v", err)
	}

	// Assert root .Trash STILL EXISTS (D-03)
	info, err := os.Stat(trashDir)
	if err != nil {
		t.Fatalf("expected root .Trash to be preserved, but stat failed: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("expected root .Trash to be a directory")
	}

	// Assert all inner items are purged
	entries, err := os.ReadDir(trashDir)
	if err != nil {
		t.Fatalf("failed to read .Trash: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries inside .Trash after cleanTrash, got %d: %+v", len(entries), entries)
	}
}

func TestTrash_ChmodRecoveryOnLockedFiles(t *testing.T) {
	tempHome := t.TempDir()
	trashDir := filepath.Join(tempHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		t.Fatalf("failed to create mock .Trash: %v", err)
	}

	// Create read-only file (0444) inside a read-only subdirectory (0555)
	readOnlyDir := filepath.Join(trashDir, "locked_folder")
	if err := os.MkdirAll(readOnlyDir, 0755); err != nil {
		t.Fatalf("failed to create locked folder: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0755)
		_ = os.Chmod(filepath.Join(readOnlyDir, "locked_file.txt"), 0644)
	})
	readOnlyFile := filepath.Join(readOnlyDir, "locked_file.txt")
	if err := os.WriteFile(readOnlyFile, []byte("immutable content"), 0444); err != nil {
		t.Fatalf("failed to create readonly file: %v", err)
	}
	// Make directory read-only too (0555)
	if err := os.Chmod(readOnlyDir, 0555); err != nil {
		t.Fatalf("failed to chmod dir: %v", err)
	}

	ctx := context.Background()
	if err := cleanTrash(ctx, trashDir); err != nil {
		t.Fatalf("cleanTrash failed to recover permissions on locked items: %v", err)
	}

	// Verify items are deleted and root preserved
	if _, err := os.Stat(readOnlyFile); !os.IsNotExist(err) {
		t.Errorf("expected locked_file.txt to be deleted, but it still exists")
	}
	if _, err := os.Stat(readOnlyDir); !os.IsNotExist(err) {
		t.Errorf("expected locked_folder to be deleted, but it still exists")
	}
	if _, err := os.Stat(trashDir); err != nil {
		t.Errorf("expected root trashDir to be preserved: %v", err)
	}
}

func TestTrash_IgnoresExternalVolumes(t *testing.T) {
	tempHome := t.TempDir()
	trashDir := filepath.Join(tempHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0700); err != nil {
		t.Fatalf("failed to create mock .Trash: %v", err)
	}

	// Create a fake external volume directory structure elsewhere
	fakeExternalVolume := t.TempDir()
	fakeExternalTrash := filepath.Join(fakeExternalVolume, ".Trashes")
	if err := os.MkdirAll(fakeExternalTrash, 0700); err != nil {
		t.Fatalf("failed to create fake external trash: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeExternalTrash, "external.bin"), make([]byte, 50000), 0644); err != nil {
		t.Fatalf("failed to write external trash file: %v", err)
	}

	target := scanTrash(tempHome)

	// Invariant: scanTrash strictly scans $HOME/.Trash, NOT external volumes (MAC-01, D-02)
	if target.Path != trashDir {
		t.Errorf("expected target path to be %s, got %s", trashDir, target.Path)
	}
	if filepath.Base(target.Path) != ".Trash" {
		t.Errorf("expected base to be .Trash, got %s", filepath.Base(target.Path))
	}
}

func TestTrash_NonExistentDir(t *testing.T) {
	tempHome := t.TempDir() // .Trash does NOT exist
	target := scanTrash(tempHome)
	if target.ID != "" {
		t.Errorf("expected empty target when .Trash does not exist, got ID=%q", target.ID)
	}

	ctx := context.Background()
	nonExistentTrash := filepath.Join(tempHome, ".Trash")
	if err := cleanTrash(ctx, nonExistentTrash); err != nil {
		t.Errorf("expected cleanTrash to return nil on non-existent dir, got: %v", err)
	}
}
