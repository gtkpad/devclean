package pnpm

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	mock := cleaner.NewMockExecutor()
	c := NewWithExecutor(mock)

	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be true when LookPath succeeds")
	}

	mock.Errors["lookpath:pnpm"] = errors.New("not found")
	if c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be false when LookPath fails")
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
