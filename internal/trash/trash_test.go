package trash

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	return New(config.SafetyConfig{
		TrashEnabled:  true,
		RetentionDays: 7,
		MaxSizeMB:     100,
		TrashDir:      dir,
	})
}

func createTempTarget(t *testing.T, name string, size int64) domain.Target {
	t.Helper()
	dir := t.TempDir()
	targetDir := filepath.Join(dir, name)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create a file to represent content
	data := make([]byte, size)
	if err := os.WriteFile(filepath.Join(targetDir, "content.bin"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return domain.Target{
		ID:        name,
		Name:      name,
		Path:      targetDir,
		SizeBytes: size,
	}
}

func TestMove_SingleTarget(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "node_modules", 1024)

	err := mgr.Move(context.Background(), []domain.Target{target}, "node")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	// Original should be gone
	if _, err := os.Stat(target.Path); !os.IsNotExist(err) {
		t.Error("expected original path to be removed")
	}

	// Manifest should have one batch with one entry
	manifest, err := LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(manifest.Batches))
	}
	if len(manifest.Batches[0].Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(manifest.Batches[0].Entries))
	}
	if manifest.Batches[0].Entries[0].OriginalPath != target.Path {
		t.Errorf("expected original path %s, got %s", target.Path, manifest.Batches[0].Entries[0].OriginalPath)
	}
}

func TestMove_MultipleTargets(t *testing.T) {
	mgr := newTestManager(t)
	t1 := createTempTarget(t, "node_modules", 512)
	t2 := createTempTarget(t, "dist", 256)

	err := mgr.Move(context.Background(), []domain.Target{t1, t2}, "node")
	if err != nil {
		t.Fatalf("Move failed: %v", err)
	}

	manifest, err := LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected 1 batch, got %d", len(manifest.Batches))
	}
	if len(manifest.Batches[0].Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(manifest.Batches[0].Entries))
	}
	if manifest.Batches[0].TotalSize != 768 {
		t.Errorf("expected total size 768, got %d", manifest.Batches[0].TotalSize)
	}
}

func TestEnforceRetention_ExpiredRemoved(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "old_modules", 100)

	err := mgr.Move(context.Background(), []domain.Target{target}, "node")
	if err != nil {
		t.Fatal(err)
	}

	// Manually set the entry time to 10 days ago (past 7-day retention)
	manifest, _ := LoadManifest(mgr.ManifestPath())
	manifest.Batches[0].Entries[0].MovedAt = time.Now().Add(-10 * 24 * time.Hour)
	manifest.Batches[0].CreatedAt = time.Now().Add(-10 * 24 * time.Hour)
	SaveManifest(mgr.ManifestPath(), manifest)

	err = mgr.EnforceRetention(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	manifest, _ = LoadManifest(mgr.ManifestPath())
	if len(manifest.Batches) != 0 {
		t.Errorf("expected 0 batches after retention, got %d", len(manifest.Batches))
	}
}

func TestEnforceRetention_FreshKept(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "recent_modules", 100)

	err := mgr.Move(context.Background(), []domain.Target{target}, "node")
	if err != nil {
		t.Fatal(err)
	}

	err = mgr.EnforceRetention(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	manifest, _ := LoadManifest(mgr.ManifestPath())
	if len(manifest.Batches) != 1 {
		t.Errorf("expected 1 batch kept, got %d", len(manifest.Batches))
	}
}

func TestEnforceMaxSize_OldestPurgedFirst(t *testing.T) {
	// Manager with 1 KB max
	dir := t.TempDir()
	mgr := New(config.SafetyConfig{
		TrashEnabled:  true,
		RetentionDays: 7,
		MaxSizeMB:     0, // 0 MB = 0 bytes, so everything is over limit
		TrashDir:      dir,
	})
	// Manually we set a very small max
	mgr.maxSize = 500

	t1 := createTempTarget(t, "old", 300)
	t2 := createTempTarget(t, "new", 400)

	mgr.Move(context.Background(), []domain.Target{t1}, "node")

	// Make the first batch older
	manifest, _ := LoadManifest(mgr.ManifestPath())
	manifest.Batches[0].Entries[0].MovedAt = time.Now().Add(-2 * time.Hour)
	SaveManifest(mgr.ManifestPath(), manifest)

	mgr.Move(context.Background(), []domain.Target{t2}, "node")

	// Total is 700, max is 500. Should remove oldest (300) first.
	err := mgr.EnforceMaxSize(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	manifest, _ = LoadManifest(mgr.ManifestPath())
	total := manifest.TotalSize()
	if total > 500 {
		t.Errorf("expected total <= 500 after max size enforcement, got %d", total)
	}
	// The newer entry (400) should still be there
	if len(manifest.Batches) != 1 {
		t.Fatalf("expected 1 batch remaining, got %d", len(manifest.Batches))
	}
	if manifest.Batches[0].Entries[0].SizeBytes != 400 {
		t.Errorf("expected newer entry (400 bytes) to survive, got %d", manifest.Batches[0].Entries[0].SizeBytes)
	}
}

func TestLoadManifest_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "manifest.json")
	manifest, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Batches) != 0 {
		t.Errorf("expected empty manifest, got %d batches", len(manifest.Batches))
	}
}

func TestMove_VirtualTargetIgnored(t *testing.T) {
	mgr := newTestManager(t)
	virtualTarget := domain.Target{
		ID:        "docker-img-1",
		Name:      "test-image",
		Path:      "docker://sha256:1234567890abcdef",
		SizeBytes: 1024 * 1024 * 50,
	}

	err := mgr.Move(context.Background(), []domain.Target{virtualTarget}, "docker")
	if err != nil {
		t.Fatalf("Move failed for virtual target: %v", err)
	}

	manifest, err := LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if len(manifest.Batches) != 0 {
		t.Fatalf("expected 0 batches for virtual-only move, got %d", len(manifest.Batches))
	}
}

func TestTarget_IsVirtual(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"docker://sha256:abc", true},
		{"docker://volume/my-vol", true},
		{"custom://artifact", true},
		{"/home/user/project/node_modules", false},
		{"/Users/user/.cache/go-build", false},
		{"", false},
	}

	for _, tc := range tests {
		target := domain.Target{Path: tc.path}
		if got := target.IsVirtual(); got != tc.want {
			t.Errorf("Target{Path: %q}.IsVirtual() = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestTrash_CorruptedManifestResilience(t *testing.T) {
	mgr := newTestManager(t)
	manifestPath := mgr.ManifestPath()

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0755); err != nil {
		t.Fatal(err)
	}
	// Gravar JSON corrompido / truncado
	if err := os.WriteFile(manifestPath, []byte("{invalid-json-content-trunc"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// 1. List não deve dar panic
	_, err := mgr.List(ctx)
	if err == nil {
		t.Error("expected error when listing with corrupted manifest, got nil")
	}

	// 2. Purge(all=true) não deve dar panic
	err = mgr.Purge(ctx, true)
	if err == nil {
		t.Error("expected error when purging with corrupted manifest, got nil")
	}

	// 3. UndoLast não deve dar panic
	err = mgr.UndoLast(ctx)
	if err == nil {
		t.Error("expected error when undoing with corrupted manifest, got nil")
	}
}

func TestTrash_ListOrderingAndUndoEntry(t *testing.T) {
	mgr := newTestManager(t)
	t1 := createTempTarget(t, "batch1_target", 128)
	t2 := createTempTarget(t, "batch2_target", 256)

	ctx := context.Background()
	if err := mgr.Move(ctx, []domain.Target{t1}, "node"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if err := mgr.Move(ctx, []domain.Target{t2}, "rust"); err != nil {
		t.Fatal(err)
	}

	batches, err := mgr.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("expected 2 batches, got %d", len(batches))
	}

	// Newest batch deve vir primeiro
	if batches[0].CreatedAt.Before(batches[1].CreatedAt) {
		t.Error("expected batches to be sorted newest first")
	}

	// Testar UndoEntry em uma entrada específica do batch mais novo
	entryToUndo := batches[0].Entries[0]
	if err := mgr.UndoEntry(ctx, entryToUndo.ID); err != nil {
		t.Fatalf("UndoEntry failed: %v", err)
	}

	// Verificar restauração
	if _, err := os.Stat(t2.Path); os.IsNotExist(err) {
		t.Errorf("expected t2 target %s to be restored", t2.Path)
	}

	// UndoEntry com ID inexistente deve retornar erro
	if err := mgr.UndoEntry(ctx, "nonexistent-id"); err == nil {
		t.Error("expected error for nonexistent entry ID, got nil")
	}
}

func TestTrash_CopyDir(t *testing.T) {
	srcDir := t.TempDir()
	destDir := filepath.Join(t.TempDir(), "copied")

	subDir := filepath.Join(srcDir, "nested", "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	file1 := filepath.Join(srcDir, "root.txt")
	file2 := filepath.Join(subDir, "nested.bin")
	if err := os.WriteFile(file1, []byte("root-data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("nested-data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copyDir(srcDir, destDir); err != nil {
		t.Fatalf("copyDir failed: %v", err)
	}

	// Validar que arquivos copiados existem e preservam conteúdo
	data1, err := os.ReadFile(filepath.Join(destDir, "root.txt"))
	if err != nil || string(data1) != "root-data" {
		t.Errorf("copied root.txt mismatch: %s (err: %v)", string(data1), err)
	}

	data2, err := os.ReadFile(filepath.Join(destDir, "nested", "pkg", "nested.bin"))
	if err != nil || string(data2) != "nested-data" {
		t.Errorf("copied nested.bin mismatch: %s (err: %v)", string(data2), err)
	}
}

func TestTrash_ManagerBranches(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	// 1. New com TrashDir vazio
	mgrEmpty := New(config.SafetyConfig{})
	expectedDefault := filepath.Join(tmpDir, ".local", "share", "devclean", "trash")
	if mgrEmpty.BaseDir() != expectedDefault {
		t.Errorf("expected BaseDir %s, got %s", expectedDefault, mgrEmpty.BaseDir())
	}

	// 2. New com caminho contendo ".."
	mgrDotDot := New(config.SafetyConfig{TrashDir: "../unsafe/dir"})
	if mgrDotDot.BaseDir() != expectedDefault {
		t.Errorf("expected fallback for dotdot, got %s", mgrDotDot.BaseDir())
	}

	// 3. containsDotDot
	if !containsDotDot("../c") {
		t.Error("expected containsDotDot to return true for ../c")
	}
	if containsDotDot("a/b/c") {
		t.Error("expected containsDotDot to return false")
	}

	// 4. Purge com all=false (apenas expirados)
	mgr := newTestManager(t)
	targetOld := createTempTarget(t, "old_target", 100)
	targetNew := createTempTarget(t, "new_target", 200)

	ctx := context.Background()
	if err := mgr.Move(ctx, []domain.Target{targetOld}, "node"); err != nil {
		t.Fatalf("Move old failed: %v", err)
	}
	if err := mgr.Move(ctx, []domain.Target{targetNew}, "node"); err != nil {
		t.Fatalf("Move new failed: %v", err)
	}

	// Modificar data do primeiro lote para simular expiração (>7 dias)
	manifest, err := LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}
	if len(manifest.Batches) != 2 || len(manifest.Batches[0].Entries) == 0 {
		t.Fatalf("expected 2 batches before purge, got %d", len(manifest.Batches))
	}
	past := time.Now().Add(-10 * 24 * time.Hour)
	manifest.Batches[0].CreatedAt = past
	manifest.Batches[0].Entries[0].MovedAt = past
	if err := SaveManifest(mgr.ManifestPath(), manifest); err != nil {
		t.Fatalf("SaveManifest failed: %v", err)
	}

	if err := mgr.Purge(ctx, false); err != nil {
		t.Fatalf("Purge(false) failed: %v", err)
	}

	manifestAfter, err := LoadManifest(mgr.ManifestPath())
	if err != nil {
		t.Fatalf("LoadManifest after failed: %v", err)
	}
	if len(manifestAfter.Batches) != 1 {
		t.Errorf("expected 1 remaining batch after purge, got %d", len(manifestAfter.Batches))
	}

	// 5. Purge com all=true
	if err := mgr.Purge(ctx, true); err != nil {
		t.Fatalf("Purge(true) failed: %v", err)
	}
	manifestEmpty, _ := LoadManifest(mgr.ManifestPath())
	if len(manifestEmpty.Batches) != 0 {
		t.Errorf("expected 0 batches after Purge(true), got %d", len(manifestEmpty.Batches))
	}
}

func TestTrash_EdgeCases(t *testing.T) {
	mgr := newTestManager(t)
	target := createTempTarget(t, "edge_target", 1024)

	// Context cancel no Move
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err := mgr.Move(cancelCtx, []domain.Target{target}, "node")
	if err == nil {
		t.Error("expected error with cancelled context on Move")
	}

	// Move normal
	ctx := context.Background()
	_ = mgr.Move(ctx, []domain.Target{target}, "node")

	// Context cancel no EnforceRetention
	manifest, _ := LoadManifest(mgr.ManifestPath())
	manifest.Batches[0].Entries[0].MovedAt = time.Now().Add(-10 * 24 * time.Hour)
	_ = SaveManifest(mgr.ManifestPath(), manifest)

	err = mgr.EnforceRetention(cancelCtx)
	if err == nil {
		t.Error("expected error with cancelled context on EnforceRetention")
	}

	// Context cancel no EnforceMaxSize
	smallMgr := New(config.SafetyConfig{
		TrashEnabled: true,
		MaxSizeMB:    0,
		TrashDir:     mgr.BaseDir(),
	})
	err = smallMgr.EnforceMaxSize(cancelCtx)
	if err == nil {
		t.Error("expected error with cancelled context on EnforceMaxSize")
	}
}

func TestTrash_UndoBranches(t *testing.T) {
	mgr := newTestManager(t)
	ctx := context.Background()

	t1 := createTempTarget(t, "item_batch", 100)
	t2 := createTempTarget(t, "item_entry", 200)

	_ = mgr.Move(ctx, []domain.Target{t1}, "node")
	_ = mgr.Move(ctx, []domain.Target{t2}, "rust")

	manifest, err := LoadManifest(mgr.ManifestPath())
	if err != nil || len(manifest.Batches) != 2 {
		t.Fatalf("unexpected manifest state: %v", err)
	}

	// 1. Undo por batch ID
	batchID := manifest.Batches[0].ID
	if err := mgr.Undo(ctx, batchID); err != nil {
		t.Fatalf("Undo(batchID) failed: %v", err)
	}
	if _, err := os.Stat(t1.Path); os.IsNotExist(err) {
		t.Errorf("expected t1 to be restored by batch ID")
	}

	// 2. Undo por entry ID
	manifest2, _ := LoadManifest(mgr.ManifestPath())
	if len(manifest2.Batches) != 1 || len(manifest2.Batches[0].Entries) != 1 {
		t.Fatalf("expected 1 batch with 1 entry, got %v", manifest2)
	}
	entryID := manifest2.Batches[0].Entries[0].ID
	if err := mgr.Undo(ctx, entryID); err != nil {
		t.Fatalf("Undo(entryID) failed: %v", err)
	}
	if _, err := os.Stat(t2.Path); os.IsNotExist(err) {
		t.Errorf("expected t2 to be restored by entry ID")
	}

	// 3. Undo por ID inexistente
	if err := mgr.Undo(ctx, "nonexistent-id"); err == nil {
		t.Error("expected error for nonexistent ID, got nil")
	}
}


