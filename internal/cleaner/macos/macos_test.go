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

	appCache := filepath.Join(mockHome, "Library", "Caches", "runningapp")
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

func TestMacOSCleaner_CleanConsolidatedSubPaths(t *testing.T) {
	t.Run("SurgicalLeafDeletionAndRootPreservation", func(t *testing.T) {
		tempDir := t.TempDir()
		cachesRoot := filepath.Join(tempDir, "Library", "Caches")
		leaf1 := filepath.Join(cachesRoot, "com.test.leaf1")
		leaf2 := filepath.Join(cachesRoot, "com.test.leaf2")
		sibling := filepath.Join(cachesRoot, "go-build")

		if err := os.MkdirAll(leaf1, 0o755); err != nil {
			t.Fatalf("failed to create leaf1: %v", err)
		}
		if err := os.MkdirAll(leaf2, 0o755); err != nil {
			t.Fatalf("failed to create leaf2: %v", err)
		}
		if err := os.MkdirAll(sibling, 0o755); err != nil {
			t.Fatalf("failed to create sibling: %v", err)
		}

		if err := os.WriteFile(filepath.Join(leaf1, "cache1.bin"), []byte("data1"), 0o644); err != nil {
			t.Fatalf("failed to write leaf1 dummy file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(leaf2, "cache2.bin"), []byte("data2"), 0o644); err != nil {
			t.Fatalf("failed to write leaf2 dummy file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sibling, "keep.bin"), []byte("important build cache"), 0o644); err != nil {
			t.Fatalf("failed to write sibling dummy file: %v", err)
		}

		c := NewWithDirAndExecutor(tempDir, newMockProcessExecutor())
		target := domain.Target{
			ID:             "macos-user-app-caches-consolidated",
			Name:           "macOS User App Caches (consolidated)",
			Path:           cachesRoot,
			IsConsolidated: true,
			SubPaths:       []string{leaf1, leaf2},
		}

		if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
			t.Fatalf("Clean failed: %v", err)
		}

		// Assert leaves removed
		if _, err := os.Stat(leaf1); !os.IsNotExist(err) {
			t.Errorf("expected leaf1 to be removed, got err: %v", err)
		}
		if _, err := os.Stat(leaf2); !os.IsNotExist(err) {
			t.Errorf("expected leaf2 to be removed, got err: %v", err)
		}

		// Assert root container preserved
		if fi, err := os.Stat(cachesRoot); err != nil || !fi.IsDir() {
			t.Errorf("cachesRoot must still exist as a directory, err: %v", err)
		}

		// Assert excluded sibling directory preserved
		if fi, err := os.Stat(sibling); err != nil || !fi.IsDir() {
			t.Errorf("sibling directory go-build must be preserved, err: %v", err)
		}
		if _, err := os.Stat(filepath.Join(sibling, "keep.bin")); err != nil {
			t.Errorf("sibling file keep.bin must still exist, err: %v", err)
		}
	})

	t.Run("EmptySubPaths_StrictNoop", func(t *testing.T) {
		tempDir := t.TempDir()
		cachesRoot := filepath.Join(tempDir, "Library", "Caches")
		sibling := filepath.Join(cachesRoot, "go-build")

		if err := os.MkdirAll(sibling, 0o755); err != nil {
			t.Fatalf("failed to create sibling: %v", err)
		}
		if err := os.WriteFile(filepath.Join(sibling, "keep.bin"), []byte("important build cache"), 0o644); err != nil {
			t.Fatalf("failed to write sibling dummy file: %v", err)
		}

		c := NewWithDirAndExecutor(tempDir, newMockProcessExecutor())
		target := domain.Target{
			ID:             "macos-user-app-caches-consolidated",
			Name:           "macOS User App Caches (consolidated)",
			Path:           cachesRoot,
			IsConsolidated: true,
			SubPaths:       []string{},
		}

		if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
			t.Fatalf("Clean failed: %v", err)
		}

		// Assert root and contents remain completely untouched
		if fi, err := os.Stat(cachesRoot); err != nil || !fi.IsDir() {
			t.Errorf("cachesRoot must be preserved when SubPaths is empty, err: %v", err)
		}
		if _, err := os.Stat(filepath.Join(sibling, "keep.bin")); err != nil {
			t.Errorf("sibling file must be preserved, err: %v", err)
		}
	})

	t.Run("ActiveProcess_SkipsActiveLeafOnly", func(t *testing.T) {
		tempDir := t.TempDir()
		cachesRoot := filepath.Join(tempDir, "Library", "Caches")
		leafActive := filepath.Join(cachesRoot, "runningapp")
		leafInactive := filepath.Join(cachesRoot, "stoppedapp")

		if err := os.MkdirAll(leafActive, 0o755); err != nil {
			t.Fatalf("failed to create leafActive: %v", err)
		}
		if err := os.MkdirAll(leafInactive, 0o755); err != nil {
			t.Fatalf("failed to create leafInactive: %v", err)
		}

		if err := os.WriteFile(filepath.Join(leafActive, "data.bin"), []byte("active"), 0o644); err != nil {
			t.Fatalf("failed to write leafActive dummy file: %v", err)
		}
		if err := os.WriteFile(filepath.Join(leafInactive, "data.bin"), []byte("inactive"), 0o644); err != nil {
			t.Fatalf("failed to write leafInactive dummy file: %v", err)
		}

		mockExec := newMockProcessExecutor()
		mockExec.runningProcesses["runningapp"] = true

		c := NewWithDirAndExecutor(tempDir, mockExec)
		target := domain.Target{
			ID:             "macos-user-app-caches-consolidated",
			Name:           "macOS User App Caches (consolidated)",
			Path:           cachesRoot,
			IsConsolidated: true,
			SubPaths:       []string{leafActive, leafInactive},
		}

		if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
			t.Fatalf("Clean failed: %v", err)
		}

		// Assert leafActive is preserved due to running process
		if _, err := os.Stat(leafActive); err != nil {
			t.Errorf("expected active leaf directory %s to be preserved, err: %v", leafActive, err)
		}
		// Assert leafInactive is deleted
		if _, err := os.Stat(leafInactive); !os.IsNotExist(err) {
			t.Errorf("expected inactive leaf directory %s to be deleted, err: %v", leafInactive, err)
		}
		// Assert cachesRoot is preserved
		if fi, err := os.Stat(cachesRoot); err != nil || !fi.IsDir() {
			t.Errorf("cachesRoot must still exist as a directory, err: %v", err)
		}
	})
}

func TestMacOSCleaner_CleanDaemons_ChildOnlyPurge(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")

	// 1. Setup sandboxed Data/tmp container with nested children
	dataTmpDir := filepath.Join(mockHome, "Library", "Containers", "com.apple.mediaanalysisd", "Data", "tmp")
	nestedSubdir := filepath.Join(dataTmpDir, "nested_tmp")
	if err := os.MkdirAll(nestedSubdir, 0o755); err != nil {
		t.Fatalf("failed to create dataTmpDir: %v", err)
	}
	file1 := filepath.Join(dataTmpDir, "temp1.dat")
	file2 := filepath.Join(dataTmpDir, "temp2.dat")
	nestedFile := filepath.Join(nestedSubdir, "file.bin")
	_ = os.WriteFile(file1, []byte("temp1"), 0o644)
	_ = os.WriteFile(file2, []byte("temp2"), 0o644)
	_ = os.WriteFile(nestedFile, []byte("nested"), 0o644)

	// 2. Setup standard daemon cache directory (Suggestions) that can be fully removed
	suggDir := filepath.Join(mockHome, "Library", "Suggestions")
	if err := os.MkdirAll(suggDir, 0o755); err != nil {
		t.Fatalf("failed to create suggDir: %v", err)
	}
	_ = os.WriteFile(filepath.Join(suggDir, "suggest.dat"), []byte("suggestions"), 0o644)

	mockExec := newMockProcessExecutor()
	c := NewWithDirAndExecutor(mockHome, mockExec)

	targets := []domain.Target{
		{
			ID:             "macos-daemon-mediaanalysisd",
			Name:           "macOS Daemon: Media Analysis Daemon",
			Path:           dataTmpDir,
			Category:       "macos-daemon-caches",
			Risk:           domain.RiskLow,
			IsConsolidated: true,
			SubPaths:       []string{dataTmpDir},
		},
		{
			ID:       "macos-daemon-siri",
			Name:     "macOS Daemon: Siri Suggestions & TTS",
			Path:     suggDir,
			Category: "macos-daemon-caches",
			Risk:     domain.RiskLow,
		},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Assert Data/tmp parent folder is preserved
	info, err := os.Stat(dataTmpDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected Data/tmp container directory to still exist, err: %v", err)
	}

	// Assert child files and directories inside Data/tmp are removed
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("expected child file %s to be deleted, got err: %v", file1, err)
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Errorf("expected child file %s to be deleted, got err: %v", file2, err)
	}
	if _, err := os.Stat(nestedSubdir); !os.IsNotExist(err) {
		t.Errorf("expected child subdir %s to be deleted, got err: %v", nestedSubdir, err)
	}

	// Assert Suggestions parent directory is completely removed
	if _, err := os.Stat(suggDir); !os.IsNotExist(err) {
		t.Errorf("expected Suggestions directory %s to be completely removed, got err: %v", suggDir, err)
	}
}

func TestMacOSCleaner_CleanDaemons_ActiveDaemonDoesNotAbort(t *testing.T) {
	tempDir := t.TempDir()
	mockHome := filepath.Join(tempDir, "mock_home")

	cacheDir := filepath.Join(mockHome, "Library", "Caches", "com.apple.mediaanalysisd")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create cacheDir: %v", err)
	}
	dataFile := filepath.Join(cacheDir, "analysis.db")
	_ = os.WriteFile(dataFile, []byte("data"), 0o644)

	// Mock running daemon process
	mockExec := newMockProcessExecutor()
	mockExec.runningProcesses["mediaanalysisd"] = true
	mockExec.runningProcesses["com.apple.mediaanalysisd"] = true

	c := NewWithDirAndExecutor(mockHome, mockExec)

	target := domain.Target{
		ID:       "macos-daemon-mediaanalysisd",
		Name:     "macOS Daemon: Media Analysis Daemon",
		Path:     cacheDir,
		Category: "macos-daemon-caches",
		Risk:     domain.RiskLow,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Assert cache directory was successfully deleted, not aborted
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("expected active daemon cache directory %s to be deleted, got err: %v", cacheDir, err)
	}
}

