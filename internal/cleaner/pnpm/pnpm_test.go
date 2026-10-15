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
	tempDir := t.TempDir()
	storeDir := filepath.Join(tempDir, "pnpm-store")
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
	if target.Path != "pnpm://store/prune" {
		t.Errorf("target.Path = %q, want %q", target.Path, "pnpm://store/prune")
	}
	if !target.IsVirtual() {
		t.Errorf("expected target.IsVirtual() to be true")
	}
	if target.Category != "virtual-pnpm-store" {
		t.Errorf("target.Category = %q, want %q", target.Category, "virtual-pnpm-store")
	}
	if target.Risk != domain.RiskLow {
		t.Errorf("target.Risk = %v, want %v", target.Risk, domain.RiskLow)
	}
	if target.SizeBytes <= 0 {
		t.Errorf("target.SizeBytes = %d, want > 0", target.SizeBytes)
	}
}

func TestPnpmCleaner_Scan_FailureOrEmpty(t *testing.T) {
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
	t.Run("clean success", func(t *testing.T) {
		mock := cleaner.NewMockExecutor()
		mock.Outputs["pnpm store prune"] = []byte("pruned")

		c := NewWithExecutor(mock)
		targets := []domain.Target{
			{Path: "pnpm://store/prune"},
			{Path: "other://something"},
		}

		if err := c.Clean(context.Background(), targets); err != nil {
			t.Fatalf("Clean() unexpected error: %v", err)
		}

		foundPrune := false
		for _, call := range mock.Calls {
			if len(call) == 3 && call[0] == "pnpm" && call[1] == "store" && call[2] == "prune" {
				foundPrune = true
				break
			}
		}
		if !foundPrune {
			t.Errorf("expected 'pnpm store prune' call in executor calls, got %v", mock.Calls)
		}
	})

	t.Run("clean error", func(t *testing.T) {
		mock := cleaner.NewMockExecutor()
		mock.Errors["pnpm store prune"] = errors.New("prune failed")

		c := NewWithExecutor(mock)
		targets := []domain.Target{
			{Path: "pnpm://store/prune"},
		}

		if err := c.Clean(context.Background(), targets); err == nil {
			t.Errorf("expected Clean() error when prune fails, got nil")
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
