package deleter_test

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
)

func TestCoordinator_ReviewOnly_Tracer(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	cleaner := &fakeCleaner{name: "test", installed: true}
	cleaners.Register("test", cleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	target := createPhysicalTarget(t, tempDir, "review-store", 2048)
	target.ReviewOnly = true
	target.ReviewReason = domain.ReviewReasonSharedStore

	var warnings []string
	opts := deleter.Options{
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
	}

	targetsByEco := map[string][]domain.Target{
		"test": {target},
	}

	res, err := coord.Delete(context.Background(), targetsByEco, opts)
	if err != nil {
		t.Fatalf("unexpected error from Delete(): %v", err)
	}

	// 1. Target must be recorded in SkippedReviewOnly
	if len(res.SkippedReviewOnly) != 1 {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want 1", len(res.SkippedReviewOnly))
	}
	if res.SkippedReviewOnly[0].ID != target.ID {
		t.Errorf("res.SkippedReviewOnly[0].ID = %q, want %q", res.SkippedReviewOnly[0].ID, target.ID)
	}

	// 2. Metrics should reflect 0 deleted
	if res.TotalCount != 0 {
		t.Errorf("res.TotalCount = %d, want 0", res.TotalCount)
	}
	if res.TotalFreed != 0 {
		t.Errorf("res.TotalFreed = %d, want 0", res.TotalFreed)
	}

	// 3. Target file must still exist on disk untouched
	contentFile := filepath.Join(target.Path, "file.bin")
	if _, err := os.Stat(contentFile); err != nil {
		t.Errorf("target content file should still exist, got error: %v", err)
	}

	// 4. Fake cleaner must never have been called
	if len(cleaner.Cleaned()) != 0 {
		t.Errorf("cleaner should not have been called, cleaned count: %d", len(cleaner.Cleaned()))
	}

	// 5. Warning callback must have been fired
	if len(warnings) != 1 {
		t.Fatalf("len(warnings) = %d, want 1", len(warnings))
	}
	if !strings.Contains(warnings[0], "shared_store") {
		t.Errorf("warning %q should contain 'shared_store'", warnings[0])
	}
	if !strings.Contains(warnings[0], target.Path) {
		t.Errorf("warning %q should contain target path %q", warnings[0], target.Path)
	}
}

func TestCoordinator_AllReviewOnlyBatch(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	dockerCleaner := &fakeCleaner{name: "docker", installed: true}
	xcodeCleaner := &fakeCleaner{name: "xcode", installed: true}
	cleaners.Register("docker", dockerCleaner)
	cleaners.Register("xcode", xcodeCleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	// Create 3 review-only targets across 2 ecosystems
	t1 := createPhysicalTarget(t, tempDir, "docker/store1", 1024)
	t1.ReviewOnly = true
	t1.ReviewReason = domain.ReviewReasonSharedStore

	t2 := createPhysicalTarget(t, tempDir, "docker/store2", 2048)
	t2.ReviewOnly = true
	t2.ReviewReason = "" // empty reason to test fallback default

	t3 := createPhysicalTarget(t, tempDir, "xcode/sim", 4096)
	t3.ReviewOnly = true
	t3.ReviewReason = domain.ReviewReasonActiveProcess

	var warnings []string
	opts := deleter.Options{
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
	}

	targets := map[string][]domain.Target{
		"docker": {t1, t2},
		"xcode":  {t3},
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("unexpected error for 100%% review-only batch: %v", err)
	}

	// Invariant D-07: TotalCount 0, TotalFreed 0, err == nil
	if res.TotalCount != 0 {
		t.Errorf("res.TotalCount = %d, want 0", res.TotalCount)
	}
	if res.TotalFreed != 0 {
		t.Errorf("res.TotalFreed = %d, want 0", res.TotalFreed)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(res.Errors), res.Errors)
	}
	if res.Duration <= 0 {
		t.Errorf("res.Duration should be > 0, got %v", res.Duration)
	}

	// All 3 targets should be skipped into SkippedReviewOnly
	if len(res.SkippedReviewOnly) != 3 {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want 3", len(res.SkippedReviewOnly))
	}

	// All 3 physical files must remain untouched
	for _, trg := range []domain.Target{t1, t2, t3} {
		f := filepath.Join(trg.Path, "file.bin")
		if _, err := os.Stat(f); err != nil {
			t.Errorf("file %s should still exist, got error: %v", f, err)
		}
	}

	// Neither cleaner should have been called
	if len(dockerCleaner.Cleaned()) != 0 {
		t.Errorf("docker cleaner was called unexpectedly: %d", len(dockerCleaner.Cleaned()))
	}
	if len(xcodeCleaner.Cleaned()) != 0 {
		t.Errorf("xcode cleaner was called unexpectedly: %d", len(xcodeCleaner.Cleaned()))
	}

	// 3 warnings should have been emitted
	if len(warnings) != 3 {
		t.Fatalf("expected 3 warnings, got %d", len(warnings))
	}
	// Verify fallback default reason for t2
	hasFallbackReason := false
	for _, w := range warnings {
		if strings.Contains(w, t2.Path) && strings.Contains(w, domain.ReviewReasonSharedStore) {
			hasFallbackReason = true
			break
		}
	}
	if !hasFallbackReason {
		t.Errorf("warning for t2 should fall back to ReviewReasonSharedStore, warnings: %v", warnings)
	}
}

func TestCoordinator_MixedBatch_ReviewOnly(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	cleaners.Register("node", nodeCleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	safeTarget := createPhysicalTarget(t, tempDir, "node/safe_modules", 1024)
	reviewTarget := createPhysicalTarget(t, tempDir, "node/pnpm_store", 2048)
	reviewTarget.ReviewOnly = true
	reviewTarget.ReviewReason = domain.ReviewReasonOrphanLeftover

	var warnings []string
	opts := deleter.Options{
		OnWarning: func(msg string) {
			warnings = append(warnings, msg)
		},
	}

	targets := map[string][]domain.Target{
		"node": {safeTarget, reviewTarget},
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("unexpected error for mixed batch: %v", err)
	}

	// Safe target should be processed (trashed)
	if res.TotalCount != 1 {
		t.Errorf("res.TotalCount = %d, want 1", res.TotalCount)
	}
	if res.TotalFreed != 1024 {
		t.Errorf("res.TotalFreed = %d, want 1024", res.TotalFreed)
	}

	// Review target should be recorded in SkippedReviewOnly
	if len(res.SkippedReviewOnly) != 1 {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want 1", len(res.SkippedReviewOnly))
	}
	if res.SkippedReviewOnly[0].ID != reviewTarget.ID {
		t.Errorf("res.SkippedReviewOnly[0].ID = %q, want %q", res.SkippedReviewOnly[0].ID, reviewTarget.ID)
	}

	// Safe target should no longer be at original path
	safeFile := filepath.Join(safeTarget.Path, "file.bin")
	if _, err := os.Stat(safeFile); !os.IsNotExist(err) {
		t.Errorf("safe target file should have been moved/deleted, stat err: %v", err)
	}

	// Review target file must still exist on disk untouched
	reviewFile := filepath.Join(reviewTarget.Path, "file.bin")
	if _, err := os.Stat(reviewFile); err != nil {
		t.Errorf("review target file should still exist, got err: %v", err)
	}

	// Exactly 1 warning for the review target
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if !strings.Contains(warnings[0], "orphan_leftover") {
		t.Errorf("warning %q should contain 'orphan_leftover'", warnings[0])
	}
	if !strings.Contains(warnings[0], reviewTarget.Path) {
		t.Errorf("warning %q should contain review target path %q", warnings[0], reviewTarget.Path)
	}
}

func TestCoordinator_StrictExpungeLock(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	// Pre-delete hook configured with a command that would fail if executed
	hookRunner := hooks.New(config.HooksConfig{
		Node: config.HookConfig{
			PreDelete: "exit 42",
		},
	})

	cleaners := newFakeCleanerProvider()
	cleaner := &fakeCleaner{name: "node", installed: true}
	cleaners.Register("node", cleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, hookRunner, configDir, dataDir)

	target := createPhysicalTarget(t, tempDir, "node/locked_item", 4096)
	target.ReviewOnly = true
	target.ReviewReason = domain.ReviewReasonSystemProtected

	opts := deleter.Options{
		Mode: "force",
		Auto: false,
	}

	targets := map[string][]domain.Target{
		"node": {target},
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	// 1. Errors must be empty (pre-delete hook was NEVER called)
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got: %v (pre-delete hook may have executed!)", res.Errors)
	}

	// 2. Cleaner was NEVER called
	if len(cleaner.Cleaned()) != 0 {
		t.Errorf("cleaner should not have been called, got %d cleaned", len(cleaner.Cleaned()))
	}

	// 3. Target recorded in SkippedReviewOnly
	if len(res.SkippedReviewOnly) != 1 {
		t.Fatalf("expected 1 skipped review target, got %d", len(res.SkippedReviewOnly))
	}

	// 4. Target still on disk
	contentFile := filepath.Join(target.Path, "file.bin")
	if _, err := os.Stat(contentFile); err != nil {
		t.Errorf("target file should still exist, got err: %v", err)
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestCoordinator_SingleGateModes(t *testing.T) {
	modes := []struct {
		name     string
		mode     string
		auto     bool
		useTrash *bool
		dryRun   bool
	}{
		{"ModeAuto", "auto", true, nil, false},
		{"ModeForce_Trash", "force", false, boolPtr(true), false},
		{"ModeForce_Direct", "force", false, boolPtr(false), false},
		{"ModeTUI", "tui", false, nil, false},
		{"ModeMCP", "mcp", false, nil, false},
		{"ModeDryRun_Auto", "auto", true, nil, true},
		{"ModeDryRun_Force", "force", false, nil, true},
	}

	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			tempDir := t.TempDir()
			configDir := filepath.Join(tempDir, "config")
			dataDir := filepath.Join(tempDir, "data")
			trashDir := filepath.Join(tempDir, "trash")

			cfg := &config.Config{
				Safety: config.SafetyConfig{
					TrashEnabled: true,
					TrashDir:     trashDir,
				},
			}

			bl := safety.NewBlacklist(nil)
			trashMgr := trash.New(cfg.Safety)

			cleaners := newFakeCleanerProvider()
			cleaner := &fakeCleaner{name: "test", installed: true}
			cleaners.Register("test", cleaner)

			coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

			target := createPhysicalTarget(t, tempDir, "test/review_item", 2048)
			target.ReviewOnly = true
			target.ReviewReason = domain.ReviewReasonSessionSensitive

			opts := deleter.Options{
				Mode:     tc.mode,
				Auto:     tc.auto,
				UseTrash: tc.useTrash,
				DryRun:   tc.dryRun,
			}

			targets := map[string][]domain.Target{
				"test": {target},
			}

			res, err := coord.Delete(context.Background(), targets, opts)
			if err != nil {
				t.Fatalf("[%s] Delete() returned error: %v", tc.name, err)
			}

			if res.TotalCount != 0 {
				t.Errorf("[%s] TotalCount = %d, want 0", tc.name, res.TotalCount)
			}
			if res.TotalFreed != 0 {
				t.Errorf("[%s] TotalFreed = %d, want 0", tc.name, res.TotalFreed)
			}
			if len(res.SkippedReviewOnly) != 1 {
				t.Fatalf("[%s] SkippedReviewOnly count = %d, want 1", tc.name, len(res.SkippedReviewOnly))
			}
			if res.SkippedReviewOnly[0].ID != target.ID {
				t.Errorf("[%s] SkippedReviewOnly ID = %q, want %q", tc.name, res.SkippedReviewOnly[0].ID, target.ID)
			}

			// File must remain on disk
			contentFile := filepath.Join(target.Path, "file.bin")
			if _, err := os.Stat(contentFile); err != nil {
				t.Errorf("[%s] target file should still exist, got: %v", tc.name, err)
			}

			// Cleaner never called
			if len(cleaner.Cleaned()) != 0 {
				t.Errorf("[%s] cleaner was called unexpectedly: %d", tc.name, len(cleaner.Cleaned()))
			}
		})
	}
}

func TestCoordinator_DryRun_NoTrashPrune(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled:  true,
			TrashDir:      trashDir,
			RetentionDays: 1, // 1 day retention
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	// Pre-populate trash with an expired batch
	oldTarget := createPhysicalTarget(t, tempDir, "old-item", 1024)
	if err := trashMgr.Move(context.Background(), []domain.Target{oldTarget}, "test"); err != nil {
		t.Fatalf("failed to move target to trash: %v", err)
	}

	// Backdate the batch and entries in manifest to 10 days ago
	manifestPath := trashMgr.ManifestPath()
	manifest, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(manifest.Batches) == 0 {
		t.Fatalf("expected at least 1 batch in trash")
	}
	oldTime := time.Now().Add(-10 * 24 * time.Hour)
	manifest.Batches[0].CreatedAt = oldTime
	for i := range manifest.Batches[0].Entries {
		manifest.Batches[0].Entries[i].MovedAt = oldTime
	}
	if err := trash.SaveManifest(manifestPath, manifest); err != nil {
		t.Fatalf("failed to save backdated manifest: %v", err)
	}

	cleaners := newFakeCleanerProvider()
	cleaner := &fakeCleaner{name: "test", installed: true}
	cleaners.Register("test", cleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	reviewTarget := createPhysicalTarget(t, tempDir, "review-item", 2048)
	reviewTarget.ReviewOnly = true
	reviewTarget.ReviewReason = domain.ReviewReasonSharedStore

	opts := deleter.Options{
		DryRun: true,
	}

	targets := map[string][]domain.Target{
		"test": {reviewTarget},
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("DryRun Delete() failed: %v", err)
	}

	// Invariant D-14: SkippedReviewOnly is populated identically in DryRun mode
	if len(res.SkippedReviewOnly) != 1 {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want 1", len(res.SkippedReviewOnly))
	}
	if res.SkippedReviewOnly[0].ID != reviewTarget.ID {
		t.Errorf("res.SkippedReviewOnly[0].ID = %q, want %q", res.SkippedReviewOnly[0].ID, reviewTarget.ID)
	}

	// Invariant POL-04, D-13: Trash retention was NOT enforced under DryRun
	manifestAfter, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to reload manifest: %v", err)
	}
	if len(manifestAfter.Batches) != 1 {
		t.Fatalf("DryRun pruned trash batches! Expected 1 batch, got %d", len(manifestAfter.Batches))
	}
	if len(manifestAfter.Batches[0].Entries) == 0 {
		t.Fatalf("DryRun pruned trash entries! Expected entries to remain")
	}

	// Now run live (DryRun: false) and verify trash maintenance IS performed by Coordinator
	optsLive := deleter.Options{
		DryRun: false,
	}
	_, err = coord.Delete(context.Background(), targets, optsLive)
	if err != nil {
		t.Fatalf("Live Delete() failed: %v", err)
	}

	manifestLive, err := trash.LoadManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to reload manifest after live: %v", err)
	}
	// The expired batch should now have been pruned by Coordinator Stage 6.1
	if len(manifestLive.Batches) != 0 {
		t.Errorf("Live run should have pruned expired batch, got %d batches", len(manifestLive.Batches))
	}
}

func TestCoordinator_BlacklistSymlink(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist([]string{"*protected_pattern*"})
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	cleaner := &fakeCleaner{name: "test", installed: true}
	cleaners.Register("test", cleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	// 1. Real .git dir + symlink
	realGit := filepath.Join(tempDir, ".git")
	if err := os.MkdirAll(realGit, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realGit, "config"), []byte("git config"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkGit := filepath.Join(tempDir, "target_symlink_git")
	if err := os.Symlink(realGit, symlinkGit); err != nil {
		t.Fatal(err)
	}

	// 2. Real custom protected dir + symlink
	realProtected := filepath.Join(tempDir, "protected_pattern_dir")
	if err := os.MkdirAll(realProtected, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realProtected, "file.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkProtected := filepath.Join(tempDir, "target_symlink_protected")
	if err := os.Symlink(realProtected, symlinkProtected); err != nil {
		t.Fatal(err)
	}

	// 3. Normal safe target
	normalTarget := createPhysicalTarget(t, tempDir, "test/normal_target", 1024)

	targets := map[string][]domain.Target{
		"test": {
			{
				ID:        "symlink-git",
				Name:      "symlink-git",
				Path:      symlinkGit,
				SizeBytes: 100,
			},
			{
				ID:        "symlink-protected",
				Name:      "symlink-protected",
				Path:      symlinkProtected,
				SizeBytes: 100,
			},
			normalTarget,
		},
	}

	opts := deleter.Options{
		Mode: "force",
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Only normalTarget should be deleted
	if res.TotalCount != 1 {
		t.Errorf("res.TotalCount = %d, want 1", res.TotalCount)
	}
	if len(res.CleanedPaths) != 1 || res.CleanedPaths[0] != normalTarget.Path {
		t.Errorf("res.CleanedPaths = %v, want [%s]", res.CleanedPaths, normalTarget.Path)
	}

	// Symlinks must NOT have been deleted or moved
	if _, err := os.Lstat(symlinkGit); err != nil {
		t.Errorf("symlinkGit was modified/deleted: %v", err)
	}
	if _, err := os.Lstat(symlinkProtected); err != nil {
		t.Errorf("symlinkProtected was modified/deleted: %v", err)
	}
	if _, err := os.Stat(filepath.Join(realGit, "config")); err != nil {
		t.Errorf("realGit config was damaged: %v", err)
	}
	if _, err := os.Stat(filepath.Join(realProtected, "file.txt")); err != nil {
		t.Errorf("realProtected was damaged: %v", err)
	}
}

func TestCoordinator_DryRun_ZeroMutation_Canary(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	cleaner := &fakeCleaner{name: "test", installed: true}
	cleaners.Register("test", cleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	// 1. Create canary file with specific content
	canaryDir := filepath.Join(tempDir, "canary_target")
	if err := os.MkdirAll(canaryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canaryPath := filepath.Join(canaryDir, "payload.bin")
	canaryContent := []byte("ZERO-MUTATION-CANARY-DATA-VERIFY-POL-04")
	if err := os.WriteFile(canaryPath, canaryContent, 0o644); err != nil {
		t.Fatal(err)
	}

	hashBefore := sha256.Sum256(canaryContent)
	statBefore, err := os.Stat(canaryPath)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Create ReviewOnly target
	reviewDir := filepath.Join(tempDir, "review_target")
	if err := os.MkdirAll(reviewDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reviewPath := filepath.Join(reviewDir, "store.bin")
	if err := os.WriteFile(reviewPath, []byte("shared-store-data"), 0o644); err != nil {
		t.Fatal(err)
	}

	canaryTarget := domain.Target{
		ID:        "canary-1",
		Name:      "canary",
		Path:      canaryDir,
		SizeBytes: statBefore.Size(),
	}
	reviewTarget := domain.Target{
		ID:           "review-1",
		Name:         "review",
		Path:         reviewDir,
		SizeBytes:    1024,
		ReviewOnly:   true,
		ReviewReason: domain.ReviewReasonSharedStore,
	}

	opts := deleter.Options{
		DryRun: true,
		Auto:   true,
		Mode:   "auto",
	}

	targets := map[string][]domain.Target{
		"test": {canaryTarget, reviewTarget},
	}

	res, err := coord.Delete(context.Background(), targets, opts)
	if err != nil {
		t.Fatalf("DryRun Delete failed: %v", err)
	}

	// Results verification
	if res.TotalCount != 1 {
		t.Errorf("res.TotalCount = %d, want 1 (would_delete count)", res.TotalCount)
	}
	if res.TotalFreed != statBefore.Size() {
		t.Errorf("res.TotalFreed = %d, want %d", res.TotalFreed, statBefore.Size())
	}
	if len(res.SkippedReviewOnly) != 1 {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want 1", len(res.SkippedReviewOnly))
	}

	// Invariant POL-04, D-15:
	// 1. Canary file must exist with identical SHA-256, size, and mtime
	statAfter, err := os.Stat(canaryPath)
	if err != nil {
		t.Fatalf("canary file missing after DryRun: %v", err)
	}
	dataAfter, err := os.ReadFile(canaryPath)
	if err != nil {
		t.Fatalf("reading canary file: %v", err)
	}
	hashAfter := sha256.Sum256(dataAfter)
	if hashBefore != hashAfter {
		t.Errorf("canary file SHA-256 mutated! Before: %x, After: %x", hashBefore, hashAfter)
	}
	if statBefore.ModTime() != statAfter.ModTime() {
		t.Errorf("canary file mtime mutated! Before: %v, After: %v", statBefore.ModTime(), statAfter.ModTime())
	}
	if statBefore.Size() != statAfter.Size() {
		t.Errorf("canary file size mutated! Before: %d, After: %d", statBefore.Size(), statAfter.Size())
	}

	// 2. Trash directory must be completely empty with zero manifests
	if entries, err := os.ReadDir(trashDir); err == nil && len(entries) > 0 {
		t.Errorf("trashDir has entries after DryRun: %v", entries)
	}
	manifestPath := filepath.Join(trashDir, "manifest.json")
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Errorf("trash manifest.json was created during DryRun: %s", manifestPath)
	}

	// 3. History JSONL file must be non-existent or 0 bytes
	historyPath := filepath.Join(dataDir, "history.jsonl")
	if stat, err := os.Stat(historyPath); err == nil && stat.Size() > 0 {
		t.Errorf("history JSONL was written during DryRun (%d bytes)", stat.Size())
	}

	// 4. Scheduler store must have recorded zero runs
	schedulerPath := filepath.Join(configDir, "last-run.json")
	if _, err := os.Stat(schedulerPath); !os.IsNotExist(err) {
		t.Errorf("scheduler last-run.json was written during DryRun: %s", schedulerPath)
	}
}

func TestCoordinator_DryRun_ZeroMutation_Phase08(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	nodeCleaner := &fakeCleaner{name: "node", installed: true}
	pnpmCleaner := &fakeCleaner{name: "pnpm", installed: true}
	shellCleaner := &fakeCleaner{name: "shell", installed: true}
	xcodeCleaner := &fakeCleaner{name: "xcode", installed: true}
	cleaners.Register("node", nodeCleaner)
	cleaners.Register("pnpm", pnpmCleaner)
	cleaners.Register("shell", shellCleaner)
	cleaners.Register("xcode", xcodeCleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	type canarySpec struct {
		eco          string
		id           string
		name         string
		category     string
		risk         domain.RiskLevel
		relPath      string
		payload      string
		reviewOnly   bool
		reviewReason string
	}

	specs := []canarySpec{
		// Node & Headless
		{
			eco:        "node",
			id:         "node-puppeteer-canary",
			name:       "Puppeteer browser cache",
			category:   "node-puppeteer",
			risk:       domain.RiskMedium,
			relPath:    "node/puppeteer_cache/chrome",
			payload:    "PUPPETEER-CHROMIUM-BIN-CANARY-DATA-V1",
			reviewOnly: false,
		},
		{
			eco:        "node",
			id:         "node-cacache-canary",
			name:       "npm cache",
			category:   "npm-cache",
			risk:       domain.RiskLow,
			relPath:    "node/npm_cacache/index",
			payload:    "NPM-CACACHE-METADATA-CANARY-DATA-V1",
			reviewOnly: false,
		},
		// pnpm
		{
			eco:        "pnpm",
			id:         "pnpm-meta-canary",
			name:       "pnpm metadata cache",
			category:   "pnpm-cache",
			risk:       domain.RiskLow,
			relPath:    "pnpm/metadata/registry.json",
			payload:    "PNPM-METADATA-REGISTRY-CANARY-DATA-V1",
			reviewOnly: false,
		},
		// Shell & CLI
		{
			eco:        "shell",
			id:         "shell-mise-cache-canary",
			name:       "mise cache",
			category:   "shell-mise-cache",
			risk:       domain.RiskLow,
			relPath:    "shell/mise_cache/plugins",
			payload:    "MISE-CACHE-METADATA-CANARY-DATA-V1",
			reviewOnly: false,
		},
		{
			eco:        "shell",
			id:         "shell-mise-downloads-canary",
			name:       "mise downloads",
			category:   "shell-mise-downloads",
			risk:       domain.RiskLow,
			relPath:    "shell/mise_downloads/go.tar.gz",
			payload:    "MISE-DOWNLOADS-TARBALL-CANARY-DATA-V1",
			reviewOnly: false,
		},
		{
			eco:        "shell",
			id:         "shell-zcompdump-canary",
			name:       "Zsh completion cache",
			category:   "shell-zsh-compdump",
			risk:       domain.RiskLow,
			relPath:    "shell/zsh_compdump/.zcompdump-mac",
			payload:    "ZSH-COMPLETION-DUMP-CANARY-DATA-V1",
			reviewOnly: false,
		},
		// Xcode
		{
			eco:        "xcode",
			id:         "xcode-deriveddata-canary",
			name:       "Xcode DerivedData: ProjectA",
			category:   "xcode-derived-data",
			risk:       domain.RiskLow,
			relPath:    "xcode/DerivedData/ProjectA/build.bin",
			payload:    "XCODE-DERIVEDDATA-CANARY-DATA-V1",
			reviewOnly: false,
		},
		{
			eco:          "xcode",
			id:           "xcode-archives-canary",
			name:         "Xcode Archives",
			category:     "xcode-archives",
			risk:         domain.RiskHigh,
			relPath:      "xcode/Archives/2026/App.xcarchive",
			payload:      "XCODE-ARCHIVES-DSYM-CANARY-DATA-V1",
			reviewOnly:   true,
			reviewReason: domain.ReviewReasonActiveProcess,
		},
		{
			eco:          "xcode",
			id:           "xcode-sim-cache-canary",
			name:         "CoreSimulator Caches",
			category:     "xcode-coresimulator-caches",
			risk:         domain.RiskLow,
			relPath:      "xcode/CoreSimulator/Caches/cache.db",
			payload:      "CORESIMULATOR-CACHES-CANARY-DATA-V1",
			reviewOnly:   true,
			reviewReason: domain.ReviewReasonActiveProcess,
		},
	}

	type fileState struct {
		absPath string
		hash    [32]byte
		size    int64
		mtime   time.Time
	}

	var preStates []fileState
	var expectedFreedBytes int64
	var expectedWouldDeleteCount int
	var expectedSkippedCount int

	targetsByEco := make(map[string][]domain.Target)

	for _, sp := range specs {
		fullPath := filepath.Join(tempDir, sp.relPath)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to create dir %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte(sp.payload), 0o644); err != nil {
			t.Fatalf("failed to write payload to %s: %v", fullPath, err)
		}

		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("stat %s: %v", fullPath, err)
		}

		preStates = append(preStates, fileState{
			absPath: fullPath,
			hash:    sha256.Sum256([]byte(sp.payload)),
			size:    info.Size(),
			mtime:   info.ModTime(),
		})

		targetDir := dir
		targetSize := info.Size()

		if sp.reviewOnly {
			expectedSkippedCount++
		} else {
			expectedWouldDeleteCount++
			expectedFreedBytes += targetSize
		}

		trg := domain.Target{
			ID:            sp.id,
			Name:          sp.name,
			Path:          targetDir,
			SizeBytes:     targetSize,
			EligibleBytes: targetSize,
			TTLEligible:   !sp.reviewOnly,
			Risk:          sp.risk,
			Category:      sp.category,
			ReviewOnly:    sp.reviewOnly,
			ReviewReason:  sp.reviewReason,
		}
		if sp.reviewOnly {
			trg.EligibleBytes = 0
		}

		targetsByEco[sp.eco] = append(targetsByEco[sp.eco], trg)
	}

	opts := deleter.Options{
		DryRun: true,
		Auto:   true,
		Mode:   "auto",
	}

	res, err := coord.Delete(context.Background(), targetsByEco, opts)
	if err != nil {
		t.Fatalf("DryRun Delete failed: %v", err)
	}

	// 1. Assert result metrics
	if res.TotalCount != expectedWouldDeleteCount {
		t.Errorf("res.TotalCount = %d, want %d", res.TotalCount, expectedWouldDeleteCount)
	}
	if res.TotalFreed != expectedFreedBytes {
		t.Errorf("res.TotalFreed = %d, want %d", res.TotalFreed, expectedFreedBytes)
	}
	if len(res.SkippedReviewOnly) != expectedSkippedCount {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want %d", len(res.SkippedReviewOnly), expectedSkippedCount)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(res.Errors), res.Errors)
	}

	// 2. Strict POL-04, VAL-02 zero-mutation assertion on EVERY canary file
	for _, pre := range preStates {
		postInfo, err := os.Stat(pre.absPath)
		if err != nil {
			t.Fatalf("canary file missing after DryRun: %s (err: %v)", pre.absPath, err)
		}
		data, err := os.ReadFile(pre.absPath)
		if err != nil {
			t.Fatalf("reading canary file %s: %v", pre.absPath, err)
		}
		postHash := sha256.Sum256(data)
		if postHash != pre.hash {
			t.Errorf("SHA-256 mutated for %s! Before: %x, After: %x", pre.absPath, pre.hash, postHash)
		}
		if postInfo.Size() != pre.size {
			t.Errorf("Byte size mutated for %s! Before: %d, After: %d", pre.absPath, pre.size, postInfo.Size())
		}
		if postInfo.ModTime() != pre.mtime {
			t.Errorf("mtime mutated for %s! Before: %v, After: %v", pre.absPath, pre.mtime, postInfo.ModTime())
		}
	}

	// 3. Trash directory must remain completely empty with zero manifests
	if entries, err := os.ReadDir(trashDir); err == nil && len(entries) > 0 {
		t.Errorf("trashDir has entries after DryRun: %v", entries)
	}
	manifestPath := filepath.Join(trashDir, "manifest.json")
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Errorf("trash manifest.json was created during DryRun: %s", manifestPath)
	}

	// 4. Cleaners must never have been called
	if len(nodeCleaner.Cleaned()) != 0 || len(pnpmCleaner.Cleaned()) != 0 ||
		len(shellCleaner.Cleaned()) != 0 || len(xcodeCleaner.Cleaned()) != 0 {
		t.Errorf("cleaners were invoked during DryRun")
	}
}

func TestCoordinator_DryRun_ZeroMutation_Phase09(t *testing.T) {
	tempDir := t.TempDir()
	configDir := filepath.Join(tempDir, "config")
	dataDir := filepath.Join(tempDir, "data")
	trashDir := filepath.Join(tempDir, "trash")

	cfg := &config.Config{
		Safety: config.SafetyConfig{
			TrashEnabled: true,
			TrashDir:     trashDir,
		},
	}

	bl := safety.NewBlacklist(nil)
	trashMgr := trash.New(cfg.Safety)

	cleaners := newFakeCleanerProvider()
	aiCleaner := &fakeCleaner{name: "ai", installed: true}
	cleaners.Register("ai", aiCleaner)

	coord := deleter.New(cleaners, cfg, bl, trashMgr, nil, configDir, dataDir)

	type fileState struct {
		absPath string
		hash    [32]byte
		size    int64
		mtime   time.Time
	}

	var preStates []fileState

	recordFile := func(relPath, payload string) string {
		fullPath := filepath.Join(tempDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(payload), 0o644); err != nil {
			t.Fatalf("write failed: %v", err)
		}
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Fatalf("stat failed: %v", err)
		}
		preStates = append(preStates, fileState{
			absPath: fullPath,
			hash:    sha256.Sum256([]byte(payload)),
			size:    info.Size(),
			mtime:   info.ModTime(),
		})
		return fullPath
	}

	// 1. Code Chromium Cache
	codeCacheFile := recordFile("ai/code_cache/Cache/data.bin", "CODE-CACHE-CANARY-DATA-V1")
	codeCacheInfo, _ := os.Stat(codeCacheFile)

	// 2. Cursor Workspace Storage
	_ = recordFile("ai/cursor_ws/User/workspaceStorage/ws1/state.vscdb", "PRIMARY-VSCDB-STATE-CANARY")
	cursorBackupFile := recordFile("ai/cursor_ws/User/workspaceStorage/ws1/state.vscdb.backup", "BACKUP-VSCDB-CANARY")
	cursorBackupInfo, _ := os.Stat(cursorBackupFile)

	// 3. Windsurf Extensions
	_ = recordFile("ai/windsurf_ext/.windsurf/extensions/.obsolete", `{"test.ext-1.0.0": true}`)
	windsurfExtPkg := recordFile("ai/windsurf_ext/.windsurf/extensions/test.ext-1.0.0/package.json", "EXT-CANARY")
	windsurfExtInfo, _ := os.Stat(windsurfExtPkg)

	// 4. Claude CLI Inactive Version
	claudeV1File := recordFile("ai/claude_versions/.local/share/claude/versions/1.0.0/claude", "CLAUDE-V1-CANARY")
	claudeV1Info, _ := os.Stat(claudeV1File)

	// 5. Claude CLI Active Version
	claudeV2File := recordFile("ai/claude_versions/.local/share/claude/versions/2.0.0/claude", "CLAUDE-V2-ACTIVE-CANARY")
	claudeV2Info, _ := os.Stat(claudeV2File)
	claudeBinDir := filepath.Join(tempDir, "ai/claude_versions/.local/bin")
	if err := os.MkdirAll(claudeBinDir, 0o755); err != nil {
		t.Fatal(err)
	}
	claudeActiveSymlink := filepath.Join(claudeBinDir, "claude")
	_ = os.Symlink(filepath.Dir(claudeV2File), claudeActiveSymlink)

	// 6. Claude CLI ReviewOnly Version
	claudeV09File := recordFile("ai/claude_versions/.local/share/claude/versions/0.9.0/claude", "CLAUDE-V09-REVIEWONLY-CANARY")
	claudeV09Info, _ := os.Stat(claudeV09File)
	claudeShareDir := filepath.Join(tempDir, "ai/claude_versions/.local/share/claude")
	_ = os.Symlink(filepath.Join(claudeShareDir, "versions", "9.9.9"), filepath.Join(claudeShareDir, "current"))

	targets := []domain.Target{
		{
			ID:            "code-cache-canary",
			Name:          "VS Code: Chromium & GPU Caches",
			Path:          "editor://code/chromium-cache",
			Category:      "editor-chromium-cache",
			Risk:          domain.RiskLow,
			SizeBytes:     codeCacheInfo.Size(),
			EligibleBytes: codeCacheInfo.Size(),
			TTLEligible:   true,
			ReviewOnly:    false,
		},
		{
			ID:            "cursor-ws-canary",
			Name:          "Cursor: Workspace State Backups",
			Path:          "editor://cursor/workspace-backups",
			Category:      "editor-workspace-backups",
			Risk:          domain.RiskMedium,
			SizeBytes:     cursorBackupInfo.Size(),
			EligibleBytes: cursorBackupInfo.Size(),
			TTLEligible:   true,
			ReviewOnly:    false,
		},
		{
			ID:            "windsurf-ext-canary",
			Name:          "Windsurf: Cached VSIXs & Obsolete Exts",
			Path:          "editor://windsurf/extensions",
			Category:      "editor-extensions",
			Risk:          domain.RiskMedium,
			SizeBytes:     windsurfExtInfo.Size(),
			EligibleBytes: windsurfExtInfo.Size(),
			TTLEligible:   true,
			ReviewOnly:    false,
		},
		{
			ID:            "claude-v1-canary",
			Name:          "Claude CLI version: 1.0.0",
			Path:          filepath.Dir(claudeV1File),
			Category:      "ai-claude-versions",
			Risk:          domain.RiskLow,
			SizeBytes:     claudeV1Info.Size(),
			EligibleBytes: claudeV1Info.Size(),
			TTLEligible:   true,
			ReviewOnly:    false,
		},
		{
			ID:            "claude-v2-active-canary",
			Name:          "Claude CLI version: 2.0.0",
			Path:          filepath.Dir(claudeV2File),
			Category:      "ai-claude-versions",
			Risk:          domain.RiskLow,
			SizeBytes:     claudeV2Info.Size(),
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonActiveProcess,
		},
		{
			ID:            "claude-v09-review-canary",
			Name:          "Claude CLI version: 0.9.0",
			Path:          filepath.Dir(claudeV09File),
			Category:      "ai-claude-versions",
			Risk:          domain.RiskLow,
			SizeBytes:     claudeV09Info.Size(),
			EligibleBytes: 0,
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonAmbiguous,
		},
	}

	targetsByEco := map[string][]domain.Target{
		"ai": targets,
	}

	expectedWouldDeleteCount := 4
	expectedSkippedCount := 2
	expectedFreedBytes := codeCacheInfo.Size() + cursorBackupInfo.Size() + windsurfExtInfo.Size() + claudeV1Info.Size()

	opts := deleter.Options{
		DryRun: true,
		Auto:   true,
		Mode:   "auto",
	}

	res, err := coord.Delete(context.Background(), targetsByEco, opts)
	if err != nil {
		t.Fatalf("DryRun Delete failed: %v", err)
	}

	// 1. Assert result metrics
	if res.TotalCount != expectedWouldDeleteCount {
		t.Errorf("res.TotalCount = %d, want %d", res.TotalCount, expectedWouldDeleteCount)
	}
	if res.TotalFreed != expectedFreedBytes {
		t.Errorf("res.TotalFreed = %d, want %d", res.TotalFreed, expectedFreedBytes)
	}
	if len(res.SkippedReviewOnly) != expectedSkippedCount {
		t.Fatalf("len(res.SkippedReviewOnly) = %d, want %d", len(res.SkippedReviewOnly), expectedSkippedCount)
	}
	if len(res.Errors) != 0 {
		t.Errorf("expected 0 errors, got %d: %v", len(res.Errors), res.Errors)
	}

	// 2. Strict POL-04, VAL-02 zero-mutation assertion on EVERY canary file
	for _, pre := range preStates {
		postInfo, err := os.Stat(pre.absPath)
		if err != nil {
			t.Fatalf("canary file missing after DryRun: %s (err: %v)", pre.absPath, err)
		}
		data, err := os.ReadFile(pre.absPath)
		if err != nil {
			t.Fatalf("reading canary file %s: %v", pre.absPath, err)
		}
		postHash := sha256.Sum256(data)
		if postHash != pre.hash {
			t.Errorf("SHA-256 mutated for %s! Before: %x, After: %x", pre.absPath, pre.hash, postHash)
		}
		if postInfo.Size() != pre.size {
			t.Errorf("Byte size mutated for %s! Before: %d, After: %d", pre.absPath, pre.size, postInfo.Size())
		}
		if postInfo.ModTime() != pre.mtime {
			t.Errorf("mtime mutated for %s! Before: %v, After: %v", pre.absPath, pre.mtime, postInfo.ModTime())
		}
	}

	// 3. Trash directory must remain completely empty with zero manifests
	if entries, err := os.ReadDir(trashDir); err == nil && len(entries) > 0 {
		t.Errorf("trashDir has entries after DryRun: %v", entries)
	}
	manifestPath := filepath.Join(trashDir, "manifest.json")
	if _, err := os.Stat(manifestPath); !os.IsNotExist(err) {
		t.Errorf("trash manifest.json was created during DryRun: %s", manifestPath)
	}

	// 4. History log must not be mutated
	historyFile := filepath.Join(dataDir, "history.jsonl")
	if info, err := os.Stat(historyFile); err == nil && info.Size() > 0 {
		t.Errorf("history.jsonl has content after DryRun: size %d", info.Size())
	}

	// 5. Cleaner must never have been called
	if len(aiCleaner.Cleaned()) != 0 {
		t.Errorf("aiCleaner was invoked during DryRun")
	}
}





