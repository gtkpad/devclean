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

