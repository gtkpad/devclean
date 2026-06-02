package trash

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestUndoLast_RestoresEntireLastBatch(t *testing.T) {
	mgr := newTestManager(t)
	t1 := createTempTarget(t, "mod1", 100)
	t2 := createTempTarget(t, "mod2", 200)
	origPath1 := t1.Path
	origPath2 := t2.Path

	if err := mgr.Move(context.Background(), []domain.Target{t1, t2}, "node"); err != nil {
		t.Fatal(err)
	}

	if err := mgr.UndoLast(context.Background()); err != nil {
		t.Fatalf("UndoLast failed: %v", err)
	}

	// Both should be restored
	if _, err := os.Stat(origPath1); os.IsNotExist(err) {
		t.Errorf("expected %s to be restored", origPath1)
	}
	if _, err := os.Stat(origPath2); os.IsNotExist(err) {
		t.Errorf("expected %s to be restored", origPath2)
	}

	// Manifest should be empty
	manifest, _ := LoadManifest(mgr.ManifestPath())
	if len(manifest.Batches) != 0 {
		t.Errorf("expected 0 batches, got %d", len(manifest.Batches))
	}
}

func TestUndoLast_EmptyManifest(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.UndoLast(context.Background())
	if err == nil {
		t.Fatal("expected error for empty manifest")
	}
	if err.Error() != "no batches to restore" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUndo_ByBatchID(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "mod_batch", 100)
	origPath := target.Path

	mgr.Move(context.Background(), []domain.Target{target}, "node")

	manifest, _ := LoadManifest(mgr.ManifestPath())
	batchID := manifest.Batches[0].ID

	if err := mgr.Undo(context.Background(), batchID); err != nil {
		t.Fatalf("Undo by batch ID failed: %v", err)
	}

	if _, err := os.Stat(origPath); os.IsNotExist(err) {
		t.Errorf("expected %s to be restored", origPath)
	}
}

func TestUndo_ByEntryID(t *testing.T) {
	mgr := newTestManager(t)
	t1 := createTempTarget(t, "entry1", 100)
	t2 := createTempTarget(t, "entry2", 200)
	origPath1 := t1.Path

	mgr.Move(context.Background(), []domain.Target{t1, t2}, "node")

	manifest, _ := LoadManifest(mgr.ManifestPath())
	entryID := manifest.Batches[0].Entries[0].ID

	if err := mgr.Undo(context.Background(), entryID); err != nil {
		t.Fatalf("Undo by entry ID failed: %v", err)
	}

	if _, err := os.Stat(origPath1); os.IsNotExist(err) {
		t.Errorf("expected %s to be restored", origPath1)
	}

	// Other entry should still be in trash
	manifest, _ = LoadManifest(mgr.ManifestPath())
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected 1 batch remaining, got %d", len(manifest.Batches))
	}
	if len(manifest.Batches[0].Entries) != 1 {
		t.Errorf("expected 1 entry remaining, got %d", len(manifest.Batches[0].Entries))
	}
}

func TestUndo_NotFound(t *testing.T) {
	mgr := newTestManager(t)
	err := mgr.Undo(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
	if err.Error() != "no batch or entry found with ID nonexistent-id" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidateRestore_PathConflict(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "conflict", 100)
	origPath := target.Path

	mgr.Move(context.Background(), []domain.Target{target}, "node")

	// Recreate something at the original path
	os.MkdirAll(origPath, 0o755)

	err := mgr.UndoLast(context.Background())
	if err == nil {
		t.Fatal("expected path conflict error")
	}
	if !contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got: %v", err)
	}
}

func TestValidateRestore_ParentIsSymlink(t *testing.T) {
	dir := t.TempDir()
	mgr := New(config.SafetyConfig{
		TrashEnabled:  true,
		RetentionDays: 7,
		MaxSizeMB:     100,
		TrashDir:      filepath.Join(dir, "trash"),
	})

	// Create target in a real directory
	realParent := filepath.Join(dir, "real_parent")
	os.MkdirAll(realParent, 0o755)
	targetDir := filepath.Join(realParent, "node_modules")
	os.MkdirAll(targetDir, 0o755)
	os.WriteFile(filepath.Join(targetDir, "f.txt"), []byte("data"), 0o644)

	target := domain.Target{
		ID: "t1", Name: "node_modules",
		Path: targetDir, SizeBytes: 4,
	}

	mgr.Move(context.Background(), []domain.Target{target}, "node")

	// Now replace the real parent with a symlink
	os.RemoveAll(realParent)
	otherDir := filepath.Join(dir, "other")
	os.MkdirAll(otherDir, 0o755)
	os.Symlink(otherDir, realParent)

	err := mgr.UndoLast(context.Background())
	if err == nil {
		t.Fatal("expected symlink parent error")
	}
	if !contains(err.Error(), "symlink") {
		t.Errorf("expected 'symlink' in error, got: %v", err)
	}
}

func TestRestoreBatch_AbortsOnAnyFailure(t *testing.T) {
	mgr := newTestManager(t)
	t1 := createTempTarget(t, "ok_entry", 100)
	t2 := createTempTarget(t, "conflict_entry", 200)
	origPath2 := t2.Path

	mgr.Move(context.Background(), []domain.Target{t1, t2}, "node")

	// Create conflict for second entry only
	os.MkdirAll(origPath2, 0o755)

	err := mgr.UndoLast(context.Background())
	if err == nil {
		t.Fatal("expected batch abort on conflict")
	}

	// Neither should be restored (batch is all-or-nothing at validation)
	manifest, _ := LoadManifest(mgr.ManifestPath())
	if len(manifest.Batches) != 1 {
		t.Errorf("expected batch still in manifest after abort, got %d", len(manifest.Batches))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
