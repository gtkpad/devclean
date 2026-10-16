package macos

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestCodeSignClones_IsolatedToUserTmpdir(t *testing.T) {
	tempDir := t.TempDir()
	tDir := filepath.Join(tempDir, "T")
	cDir := filepath.Join(tempDir, "C")
	if err := os.MkdirAll(tDir, 0o755); err != nil {
		t.Fatalf("failed to create tDir: %v", err)
	}
	if err := os.MkdirAll(cDir, 0o755); err != nil {
		t.Fatalf("failed to create cDir: %v", err)
	}

	clone1 := filepath.Join(cDir, "chrome.code_sign_clone")
	if err := os.WriteFile(clone1, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("failed to create clone1: %v", err)
	}
	clone2 := filepath.Join(cDir, "nested.code_sign_clone")
	if err := os.WriteFile(clone2, make([]byte, 2048), 0o644); err != nil {
		t.Fatalf("failed to create clone2: %v", err)
	}
	unrelated := filepath.Join(cDir, "other.tmp")
	if err := os.WriteFile(unrelated, make([]byte, 5000), 0o644); err != nil {
		t.Fatalf("failed to create unrelated: %v", err)
	}

	t.Setenv("TMPDIR", tDir)

	// Case 1: Browser stopped (pgrep exit 1)
	mockExecStopped := newMockProcessExecutor()

	target, ok := scanCodeSignClones(context.Background(), mockExecStopped)
	if !ok {
		t.Fatalf("expected scanCodeSignClones to find targets, got false")
	}
	if target.ID != "macos-code-sign-clones" {
		t.Errorf("target.ID = %q, want 'macos-code-sign-clones'", target.ID)
	}
	if target.SizeBytes != 3072 {
		t.Errorf("target.SizeBytes = %d, want 3072", target.SizeBytes)
	}
	if target.EligibleBytes != 3072 {
		t.Errorf("target.EligibleBytes = %d, want 3072", target.EligibleBytes)
	}
	if target.ReviewOnly {
		t.Errorf("target.ReviewOnly = true, want false when browser is stopped")
	}

	// Case 2: Browser running (pgrep exit 0)
	mockExecRunning := newMockProcessExecutor()
	mockExecRunning.runningProcesses["Google Chrome"] = true

	targetRunning, ok := scanCodeSignClones(context.Background(), mockExecRunning)
	if !ok {
		t.Fatalf("expected scanCodeSignClones to find targets, got false")
	}
	if !targetRunning.ReviewOnly {
		t.Errorf("targetRunning.ReviewOnly = false, want true when browser is running")
	}
	if targetRunning.ReviewReason != domain.ReviewReasonActiveProcess {
		t.Errorf("targetRunning.ReviewReason = %q, want %q", targetRunning.ReviewReason, domain.ReviewReasonActiveProcess)
	}
	if targetRunning.EligibleBytes != 0 {
		t.Errorf("targetRunning.EligibleBytes = %d, want 0", targetRunning.EligibleBytes)
	}
}

func TestMacOSCleaner_Lifecycle(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")

	// Set up basic user paths
	cacheDir := filepath.Join(mockHome, "Library", "Caches", "com.example.app")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create cacheDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "data.bin"), make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("failed to create cache file: %v", err)
	}

	trashDir := filepath.Join(mockHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatalf("failed to create trashDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(trashDir, "old.txt"), []byte("garbage"), 0o644); err != nil {
		t.Fatalf("failed to create trash item: %v", err)
	}

	mockExec := newMockProcessExecutor()

	c := NewWithDirAndExecutor(mockHome, mockExec)
	if c.Name() != "macos" {
		t.Errorf("c.Name() = %q, want 'macos'", c.Name())
	}
	if !c.IsInstalled() {
		t.Errorf("c.IsInstalled() = false, want true")
	}

	// Scan when disabled returns nil
	disabledTargets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error when disabled: %v", err)
	}
	if len(disabledTargets) != 0 {
		t.Errorf("len(disabledTargets) = %d, want 0", len(disabledTargets))
	}

	// Scan when enabled returns targets
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("unexpected error when enabled: %v", err)
	}
	if len(targets) == 0 {
		t.Errorf("expected targets from Scan, got 0")
	}
}

func TestMacOSCleaner_PreDeleteProcessRevalidation(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")

	appCache := filepath.Join(mockHome, "Library", "Caches", "com.example.runningapp")
	if err := os.MkdirAll(appCache, 0o755); err != nil {
		t.Fatalf("failed to create appCache: %v", err)
	}
	dataFile := filepath.Join(appCache, "active.bin")
	if err := os.WriteFile(dataFile, make([]byte, 60*1024*1024), 0o644); err != nil {
		t.Fatalf("failed to write dataFile: %v", err)
	}

	// 1. Initial scan: process stopped
	mockExec := newMockProcessExecutor()

	c := NewWithDirAndExecutor(mockHome, mockExec)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var appTarget *domain.Target
	for i := range targets {
		if targets[i].Path == appCache {
			appTarget = &targets[i]
			break
		}
	}
	if appTarget == nil {
		t.Fatalf("missing target for %s", appCache)
	}
	if appTarget.ReviewOnly {
		t.Fatalf("initial scan should have ReviewOnly == false")
	}

	// 2. User starts the application before Clean() is invoked!
	mockExec.runningProcesses["runningapp"] = true

	// 3. Clean() must revalidate process and ABORT deletion!
	if err := c.Clean(context.Background(), []domain.Target{*appTarget}); err != nil {
		t.Fatalf("Clean failed with error: %v", err)
	}

	// Assert file was NOT deleted!
	if _, err := os.Stat(dataFile); err != nil {
		t.Errorf("expected file %s to remain on disk after pre-delete process revalidation, got err: %v", dataFile, err)
	}
}

func TestMacOSCleaner_CleanTrashDirect(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")

	trashDir := filepath.Join(mockHome, ".Trash")
	if err := os.MkdirAll(trashDir, 0o755); err != nil {
		t.Fatalf("failed to create trashDir: %v", err)
	}
	trashFile := filepath.Join(trashDir, "unwanted.pdf")
	if err := os.WriteFile(trashFile, []byte("contents"), 0o644); err != nil {
		t.Fatalf("failed to write trashFile: %v", err)
	}

	mockExec := newMockProcessExecutor()

	c := NewWithDirAndExecutor(mockHome, mockExec)
	trashTarget := domain.Target{
		ID:         "macos-trash",
		Path:       trashDir,
		Category:   "macos-trash",
		ReviewOnly: false, // Authorized by --trash flag
	}

	if err := c.Clean(context.Background(), []domain.Target{trashTarget}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Trash file should be gone
	if _, err := os.Stat(trashFile); !os.IsNotExist(err) {
		t.Errorf("trash file should be removed, got err: %v", err)
	}

	// Root ~/.Trash directory must be preserved
	if info, err := os.Stat(trashDir); err != nil || !info.IsDir() {
		t.Errorf("root .Trash directory must still exist as a directory, err: %v", err)
	}
}
