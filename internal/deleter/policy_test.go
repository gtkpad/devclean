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
