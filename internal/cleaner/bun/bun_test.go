package bun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestBunCleaner_Name(t *testing.T) {
	c := New()
	if got := c.Name(); got != "bun" {
		t.Errorf("Name() = %q, want %q", got, "bun")
	}
}

func TestBunCleaner_IsInstalled(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("PATH", tempDir) // empty bin dir, no bun in PATH

	cachePath := filepath.Join(tempDir, "bun-cache")

	c := NewWithPath(cachePath)
	if c.IsInstalled() {
		t.Errorf("expected IsInstalled() = false when dir does not exist and bun not in PATH")
	}

	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		t.Fatalf("failed to create cachePath: %v", err)
	}

	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() = true when cache dir exists on disk")
	}
}


func TestBunCleaner_Scan(t *testing.T) {
	t.Run("stale cache scanned", func(t *testing.T) {
		tempDir := t.TempDir()
		cachePath := filepath.Join(tempDir, "install", "cache")
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		dummyFile := filepath.Join(cachePath, "pkg-1.0.0.tgz")
		if err := os.WriteFile(dummyFile, []byte("tarball contents"), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}

		oldTime := time.Now().AddDate(0, 0, -30)
		if err := os.Chtimes(cachePath, oldTime, oldTime); err != nil {
			t.Fatalf("chtimes failed: %v", err)
		}

		c := NewWithPath(cachePath)
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 14})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}

		if len(targets) != 1 {
			t.Fatalf("expected 1 target, got %d", len(targets))
		}

		tgt := targets[0]
		sum := sha256.Sum256([]byte(cachePath))
		expectedID := fmt.Sprintf("%x", sum[:6])

		if tgt.ID != expectedID {
			t.Errorf("tgt.ID = %q, want %q", tgt.ID, expectedID)
		}
		if tgt.Path != cachePath {
			t.Errorf("tgt.Path = %q, want %q", tgt.Path, cachePath)
		}
		if tgt.Category != "bun-cache" {
			t.Errorf("tgt.Category = %q, want %q", tgt.Category, "bun-cache")
		}
		if tgt.Risk != domain.RiskLow {
			t.Errorf("tgt.Risk = %v, want %v", tgt.Risk, domain.RiskLow)
		}
		if tgt.SizeBytes <= 0 {
			t.Errorf("tgt.SizeBytes = %d, want > 0", tgt.SizeBytes)
		}
	})

	t.Run("recent cache reported but not ttl eligible", func(t *testing.T) {
		tempDir := t.TempDir()
		cachePath := filepath.Join(tempDir, "install", "cache")
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		dummyFile := filepath.Join(cachePath, "pkg-1.0.0.tgz")
		if err := os.WriteFile(dummyFile, []byte("tarball contents"), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}

		recentTime := time.Now().AddDate(0, 0, -1)
		if err := os.Chtimes(cachePath, recentTime, recentTime); err != nil {
			t.Fatalf("chtimes failed: %v", err)
		}

		c := NewWithPath(cachePath)
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 14})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}

		if len(targets) != 1 {
			t.Fatalf("expected 1 target for recent cache, got %d", len(targets))
		}
		if targets[0].TTLEligible {
			t.Errorf("expected TTLEligible = false for recent cache, got true")
		}
		if targets[0].EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes = 0 for recent cache, got %d", targets[0].EligibleBytes)
		}
		if targets[0].SizeBytes <= 0 {
			t.Errorf("expected SizeBytes > 0, got %d", targets[0].SizeBytes)
		}
	})

	t.Run("recent cache included when MaxAgeDays is 0", func(t *testing.T) {
		tempDir := t.TempDir()
		cachePath := filepath.Join(tempDir, "install", "cache")
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}
		dummyFile := filepath.Join(cachePath, "pkg-1.0.0.tgz")
		if err := os.WriteFile(dummyFile, []byte("tarball contents"), 0o644); err != nil {
			t.Fatalf("write file failed: %v", err)
		}

		c := NewWithPath(cachePath)
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}

		if len(targets) != 1 {
			t.Fatalf("expected 1 target when MaxAgeDays=0, got %d", len(targets))
		}
		if !targets[0].TTLEligible {
			t.Errorf("expected TTLEligible = true when MaxAgeDays=0")
		}
		if targets[0].EligibleBytes != targets[0].SizeBytes {
			t.Errorf("expected EligibleBytes == SizeBytes (%d), got %d", targets[0].SizeBytes, targets[0].EligibleBytes)
		}
	})

	t.Run("non-existent cache", func(t *testing.T) {
		c := NewWithPath("/non/existent/path/for/bun/cache")
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 14})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets, got %d", len(targets))
		}
	})

	t.Run("empty cache directory", func(t *testing.T) {
		tempDir := t.TempDir()
		cachePath := filepath.Join(tempDir, "install", "cache")
		if err := os.MkdirAll(cachePath, 0o755); err != nil {
			t.Fatalf("mkdir failed: %v", err)
		}

		c := NewWithPath(cachePath)
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
		if err != nil {
			t.Fatalf("Scan() unexpected error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets for empty dir, got %d", len(targets))
		}
	})
}

func TestBunCleaner_Clean(t *testing.T) {
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, "install", "cache")
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	dummyFile := filepath.Join(cachePath, "pkg-1.0.0.tgz")
	if err := os.WriteFile(dummyFile, []byte("tarball contents"), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	c := NewWithPath(cachePath)
	targets := []domain.Target{
		{Path: cachePath},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean() unexpected error: %v", err)
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Errorf("expected cache directory to be removed")
	}
}
