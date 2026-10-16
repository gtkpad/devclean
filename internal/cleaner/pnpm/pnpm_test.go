package pnpm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"


	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestPnpmCleaner_Name(t *testing.T) {
	c := New()
	if got := c.Name(); got != "pnpm" {
		t.Errorf("Name() = %q, want %q", got, "pnpm")
	}
}

func TestPnpmCleaner_IsInstalled(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	mock := cleaner.NewMockExecutor()
	c := NewWithExecutor(mock)

	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be true when LookPath succeeds")
	}

	mock.Errors["lookpath:pnpm"] = errors.New("not found")
	if c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be false when LookPath fails and cache is absent")
	}

	// Verify hybrid detection: LookPath fails but store exists on disk
	storeDir := filepath.Join(tempHome, "Library", "pnpm", "store")
	if runtime.GOOS != "darwin" {
		storeDir = filepath.Join(tempHome, ".local", "share", "pnpm", "store")
	}
	_ = os.MkdirAll(storeDir, 0755)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be true when LookPath fails but store exists on disk")
	}
}


func TestPnpmCleaner_Scan_Success(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	storeDir := filepath.Join(tempHome, "pnpm-store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("failed to create storeDir: %v", err)
	}
	testFile := filepath.Join(storeDir, "sample-pkg")
	if err := os.WriteFile(testFile, []byte("package data"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	mock.Outputs["pnpm store path"] = []byte(storeDir + "\n")

	c := NewWithExecutor(mock)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("Scan() returned %d targets, want 1", len(targets))
	}

	target := targets[0]
	resolvedStoreDir, resErr := filepath.EvalSymlinks(storeDir)
	if resErr != nil {
		resolvedStoreDir = storeDir
	}
	if target.Path != resolvedStoreDir {
		t.Errorf("target.Path = %q, want %q", target.Path, resolvedStoreDir)
	}
	if !target.ReviewOnly {
		t.Errorf("expected target.ReviewOnly to be true")
	}
	if target.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("target.ReviewReason = %q, want %q", target.ReviewReason, domain.ReviewReasonSharedStore)
	}
	if target.EligibleBytes != 0 {
		t.Errorf("target.EligibleBytes = %d, want 0", target.EligibleBytes)
	}
	if target.TTLEligible {
		t.Errorf("expected TTLEligible = false")
	}
	if target.Category != "pnpm-store" {
		t.Errorf("target.Category = %q, want %q", target.Category, "pnpm-store")
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("target.Risk = %v, want %v", target.Risk, domain.RiskLow)
	}
	if target.SizeBytes <= 0 {
		t.Errorf("target.SizeBytes = %d, want > 0", target.SizeBytes)
	}
}

func TestPnpmCleaner_Scan_FailureOrEmpty(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	t.Run("command error", func(t *testing.T) {
		mock := cleaner.NewMockExecutor()
		mock.Errors["pnpm store path"] = errors.New("command not found")
		c := NewWithExecutor(mock)

		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets on command error, got %d", len(targets))
		}
	})

	t.Run("empty output", func(t *testing.T) {
		mock := cleaner.NewMockExecutor()
		mock.Outputs["pnpm store path"] = []byte("   \n")
		c := NewWithExecutor(mock)

		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets on empty output, got %d", len(targets))
		}
	})

	t.Run("empty store directory", func(t *testing.T) {
		tempDir := t.TempDir()
		mock := cleaner.NewMockExecutor()
		mock.Outputs["pnpm store path"] = []byte(tempDir)
		c := NewWithExecutor(mock)

		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets on empty store dir, got %d", len(targets))
		}
	})
}

func TestPnpmCleaner_Clean(t *testing.T) {
	t.Run("clean skips virtual prune and executes no external prune", func(t *testing.T) {
		mock := cleaner.NewMockExecutor()
		c := NewWithExecutor(mock)
		targets := []domain.Target{
			{Path: "pnpm://store/prune", Category: "virtual-pnpm-store"},
			{Path: "other://something"},
		}

		if err := c.Clean(context.Background(), targets); err != nil {
			t.Fatalf("Clean() unexpected error: %v", err)
		}

		if len(mock.Calls) != 0 {
			t.Errorf("expected 0 calls to executor, got %v", mock.Calls)
		}
	})
}

func TestPnpmCleaner_IsInstalled_DiskPresence(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	mock := cleaner.NewMockExecutor()
	mock.Errors["lookpath:pnpm"] = errors.New("not found")
	c := NewWithExecutor(mock)

	if c.IsInstalled() {
		t.Fatalf("expected IsInstalled() to be false initially")
	}

	cacheDir := filepath.Join(tempHome, "Library", "Caches", "pnpm")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}

	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() == true when Library/Caches/pnpm exists")
	}
}

func TestPnpmCleaner_Scan_DiskMetadataCache(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cacheDir := filepath.Join(tempHome, ".cache", "pnpm")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "metadata.json"), []byte("cached-metadata"), 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	mock.Errors["pnpm store path"] = errors.New("pnpm not in path")
	c := NewWithExecutor(mock)

	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	var metaTarget *domain.Target
	for i := range targets {
		if targets[i].Category == "pnpm-cache" {
			metaTarget = &targets[i]
			break
		}
	}

	if metaTarget == nil {
		t.Fatalf("expected target with category 'pnpm-cache' to be discovered")
	}

	if metaTarget.Name != "pnpm metadata cache" {
		t.Errorf("Name = %q, want 'pnpm metadata cache'", metaTarget.Name)
	}
	if metaTarget.Path != cacheDir {
		t.Errorf("Path = %q, want %q", metaTarget.Path, cacheDir)
	}
	if metaTarget.Risk != domain.RiskLow {
		t.Errorf("Risk = %v, want RiskLow", metaTarget.Risk)
	}
	if metaTarget.SizeBytes <= 0 {
		t.Errorf("SizeBytes = %d, want > 0", metaTarget.SizeBytes)
	}
	if !metaTarget.TTLEligible {
		t.Errorf("expected TTLEligible = true with MaxAgeDays = 0")
	}
	if metaTarget.EligibleBytes != metaTarget.SizeBytes {
		t.Errorf("EligibleBytes = %d, want %d", metaTarget.EligibleBytes, metaTarget.SizeBytes)
	}
	if len(metaTarget.ID) != 12 {
		t.Errorf("ID = %q, want 12-char hex string", metaTarget.ID)
	}
}

func TestPnpmCleaner_Clean_DiskMetadataCache(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	cacheDir := filepath.Join(tempHome, "Library", "Caches", "pnpm")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	testFile := filepath.Join(cacheDir, "meta.bin")
	if err := os.WriteFile(testFile, []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	outsideDir := filepath.Join(tempHome, "outside")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	symlinkPath := filepath.Join(tempHome, "symlink-cache")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatalf("failed to symlink: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	mock.Outputs["pnpm store prune"] = []byte("pruned")
	c := NewWithExecutor(mock)

	targets := []domain.Target{
		{Path: "pnpm://store/prune"},
		{Path: cacheDir, Category: "pnpm-cache"},
		{Path: symlinkPath, Category: "pnpm-cache"},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean() unexpected error: %v", err)
	}

	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("expected cacheDir to be deleted")
	}

	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Errorf("expected symlinkPath to be skipped, got error: %v", err)
	}
	if _, err := os.Stat(outsideDir); err != nil {
		t.Errorf("outsideDir should not be affected: %v", err)
	}
}

func TestPnpmCleaner_Clean_SymlinkProtection(t *testing.T) {
	tempHome := t.TempDir()
	outsideDir := filepath.Join(tempHome, "valuable-outside-dir")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatalf("failed to create outside dir: %v", err)
	}
	canaryPath := filepath.Join(outsideDir, "canary.txt")
	if err := os.WriteFile(canaryPath, []byte("vital pnpm package source"), 0o644); err != nil {
		t.Fatalf("failed to write canary: %v", err)
	}

	symlinkPath := filepath.Join(tempHome, "symlink-cache-pnpm")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	c := NewWithExecutor(mock)

	targets := []domain.Target{
		{
			ID:       "symlink-target",
			Name:     "pnpm metadata symlink",
			Path:     symlinkPath,
			Category: "pnpm-cache",
		},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean() unexpected error: %v", err)
	}

	// Symlink target must not be traversed and outside files remain intact
	if _, err := os.Stat(canaryPath); err != nil {
		t.Errorf("VIOLATION: symlink target contents were deleted: %v", err)
	}
	data, err := os.ReadFile(canaryPath)
	if err != nil || string(data) != "vital pnpm package source" {
		t.Errorf("canary file data was corrupted: %v", err)
	}
	if _, err := os.Lstat(symlinkPath); err != nil {
		t.Errorf("symlink itself should be skipped and preserved: %v", err)
	}
}

func TestPnpmCleaner_Clean_PermissionHandling(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping unix permission test on windows")
	}
	tempDir := t.TempDir()
	lockedDir := filepath.Join(tempDir, "locked")
	if err := os.MkdirAll(lockedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(lockedDir, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(lockedDir, 0o555)
	t.Cleanup(func() {
		_ = os.Chmod(lockedDir, 0o755)
	})

	mock := cleaner.NewMockExecutor()
	c := NewWithExecutor(mock)

	targets := []domain.Target{
		{Path: file, Category: "pnpm-cache"},
	}

	// Must not panic
	_ = c.Clean(context.Background(), targets)
}

func TestPnpm_HybridStoreDiscovery_ReviewOnly(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Create CLI store path
	cliStore := filepath.Join(tempHome, "custom-pnpm-store")
	if err := os.MkdirAll(filepath.Join(cliStore, "v3"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f1 := filepath.Join(cliStore, "v3", "f1.bin")
	if err := os.WriteFile(f1, []byte("shared-pkg-data-12345"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Create candidate disk store path: ~/.pnpm-store
	diskStore := filepath.Join(tempHome, ".pnpm-store")
	if err := os.MkdirAll(filepath.Join(diskStore, "v3"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	f2 := filepath.Join(diskStore, "v3", "f2.bin")
	if err := os.WriteFile(f2, []byte("shared-pkg-data-67890"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	mock.Outputs["pnpm store path"] = []byte(cliStore + "\n")

	c := NewWithExecutor(mock)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var storeTargets []domain.Target
	for _, tgt := range targets {
		if tgt.Category == "pnpm-store" {
			storeTargets = append(storeTargets, tgt)
		}
	}

	// Should discover both cliStore and diskStore
	if len(storeTargets) != 2 {
		t.Fatalf("expected 2 store targets, got %d", len(storeTargets))
	}

	for _, st := range storeTargets {
		if !st.ReviewOnly {
			t.Errorf("store target %s: expected ReviewOnly == true", st.Path)
		}
		if st.ReviewReason != domain.ReviewReasonSharedStore {
			t.Errorf("store target %s: expected ReviewReason == %q, got %q", st.Path, domain.ReviewReasonSharedStore, st.ReviewReason)
		}
		if st.EligibleBytes != 0 {
			t.Errorf("store target %s: expected EligibleBytes == 0, got %d", st.Path, st.EligibleBytes)
		}
		if st.TTLEligible {
			t.Errorf("store target %s: expected TTLEligible == false", st.Path)
		}
		if st.ProviderMetrics == nil {
			t.Fatalf("store target %s: ProviderMetrics is nil", st.Path)
		}
		if st.ProviderMetrics.AllocatedBytes == 0 && st.ProviderMetrics.ApparentBytes == 0 {
			t.Errorf("store target %s: expected non-zero metrics", st.Path)
		}
		if st.ProviderMetrics.StatusNote != "content-addressable shared package store" {
			t.Errorf("store target %s: unexpected StatusNote %q", st.Path, st.ProviderMetrics.StatusNote)
		}
	}
}

func TestPnpm_Clean_MetadataCacheOnly_NoStorePrune(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	storeDir := filepath.Join(tempHome, ".pnpm-store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	sentinel := filepath.Join(storeDir, "sentinel.pkg")
	if err := os.WriteFile(sentinel, []byte("SENTINEL-PACKAGE-DATA"), 0o644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	cacheDir := filepath.Join(tempHome, "Library", "Caches", "pnpm")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	cacheFile := filepath.Join(cacheDir, "metadata.json")
	if err := os.WriteFile(cacheFile, []byte("cached-metadata"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	mock := cleaner.NewMockExecutor()
	c := NewWithExecutor(mock)

	targets := []domain.Target{
		{Path: "pnpm://store/prune", Category: "virtual-pnpm-store", ReviewOnly: true},
		{Path: storeDir, Category: "pnpm-store", ReviewOnly: true},
		{Path: cacheDir, Category: "pnpm-cache"},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Cache must be deleted
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("expected metadata cache dir to be deleted")
	}

	// Store sentinel must remain intact
	data, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("expected store sentinel to be preserved, got err: %v", err)
	}
	if string(data) != "SENTINEL-PACKAGE-DATA" {
		t.Errorf("store sentinel corrupted: %s", string(data))
	}

	// pnpm store prune must NEVER be called
	for _, call := range mock.Calls {
		if len(call) >= 3 && call[0] == "pnpm" && call[1] == "store" && call[2] == "prune" {
			t.Fatalf("FORBIDDEN: pnpm store prune was executed: %v", call)
		}
	}
}


