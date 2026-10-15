package deleter_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
