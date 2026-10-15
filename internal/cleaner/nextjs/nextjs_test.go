package nextjs

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestNextJSCleaner_Name(t *testing.T) {
	c := New(nil)
	if got := c.Name(); got != "nextjs" {
		t.Errorf("Name() = %q, want %q", got, "nextjs")
	}
}

func TestNextJSCleaner_IsInstalled(t *testing.T) {
	// Should be true when next.config.js exists in scanPaths
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "next.config.js"), []byte("module.exports = {}"), 0644)
	c := New([]string{tmp})
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled() to be true when next.config.js exists in scanPaths")
	}

	// Should be false when PATH is empty and no manifests exist in scanPaths
	t.Setenv("PATH", t.TempDir())
	cEmpty := New([]string{t.TempDir()})
	if cEmpty.IsInstalled() {
		t.Errorf("expected IsInstalled() to be false when PATH is empty and no manifests exist")
	}
}


func TestNextJSCleaner_ScanAndClean_PreservesServer(t *testing.T) {
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "app-demo")
	cacheDir := filepath.Join(appDir, ".next", "cache")
	serverDir := filepath.Join(appDir, ".next", "server")
	staticDir := filepath.Join(appDir, ".next", "static")

	if err := os.MkdirAll(filepath.Join(cacheDir, "webpack"), 0o755); err != nil {
		t.Fatalf("mkdir cache failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(serverDir, "pages"), 0o755); err != nil {
		t.Fatalf("mkdir server failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(staticDir, "chunks"), 0o755); err != nil {
		t.Fatalf("mkdir static failed: %v", err)
	}

	cacheFile := filepath.Join(cacheDir, "webpack", "bundle.cache")
	serverFile := filepath.Join(serverDir, "pages", "index.js")
	staticFile := filepath.Join(staticDir, "chunks", "main.js")

	if err := os.WriteFile(cacheFile, []byte("cache content"), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}
	if err := os.WriteFile(serverFile, []byte("server code"), 0o644); err != nil {
		t.Fatalf("write server file: %v", err)
	}
	if err := os.WriteFile(staticFile, []byte("static assets"), 0o644); err != nil {
		t.Fatalf("write static file: %v", err)
	}

	oldTime := time.Now().AddDate(0, 0, -30)
	if err := os.Chtimes(cacheDir, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes failed: %v", err)
	}

	c := New([]string{tempDir})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 7})
	if err != nil {
		t.Fatalf("Scan() unexpected error: %v", err)
	}

	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}

	tgt := targets[0]
	if tgt.Path != cacheDir {
		t.Errorf("target.Path = %q, want %q", tgt.Path, cacheDir)
	}
	if tgt.Category != "nextjs-cache" {
		t.Errorf("target.Category = %q, want %q", tgt.Category, "nextjs-cache")
	}
	if tgt.Risk != domain.RiskLow {
		t.Errorf("target.Risk = %v, want %v", tgt.Risk, domain.RiskLow)
	}
	if tgt.Name != "app-demo/.next/cache" {
		t.Errorf("target.Name = %q, want %q", tgt.Name, "app-demo/.next/cache")
	}
	if tgt.SizeBytes <= 0 {
		t.Errorf("target.SizeBytes = %d, want > 0", tgt.SizeBytes)
	}

	// Clean the targets
	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean() unexpected error: %v", err)
	}

	// Verify that cache was removed
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("expected cacheDir %q to be deleted", cacheDir)
	}

	// Critical check (D-07): .next/server and .next/static MUST remain intact
	if _, err := os.Stat(serverFile); err != nil {
		t.Errorf("critical failure: server file %q was deleted or not accessible: %v", serverFile, err)
	}
	if _, err := os.Stat(staticFile); err != nil {
		t.Errorf("critical failure: static file %q was deleted or not accessible: %v", staticFile, err)
	}
}

func TestNextJSCleaner_Scan_TTLAndFiltering(t *testing.T) {
	tempDir := t.TempDir()
	appDir := filepath.Join(tempDir, "recent-app")
	cacheDir := filepath.Join(appDir, ".next", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "build.cache"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	recentTime := time.Now().AddDate(0, 0, -1)
	if err := os.Chtimes(cacheDir, recentTime, recentTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	c := New([]string{tempDir})

	t.Run("skip recent cache with MaxAgeDays", func(t *testing.T) {
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 7})
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target discovered, got %d", len(targets))
		}
		if targets[0].TTLEligible {
			t.Errorf("expected TTLEligible=false for recent cache, got true")
		}
		if targets[0].EligibleBytes != 0 {
			t.Errorf("expected EligibleBytes=0 for recent cache, got %d", targets[0].EligibleBytes)
		}
	})

	t.Run("include recent cache with MaxAgeDays=0", func(t *testing.T) {
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(targets) != 1 {
			t.Fatalf("expected 1 target when MaxAgeDays=0, got %d", len(targets))
		}
		if !targets[0].TTLEligible {
			t.Errorf("expected TTLEligible=true when MaxAgeDays=0")
		}
		if targets[0].EligibleBytes != targets[0].SizeBytes {
			t.Errorf("expected EligibleBytes == SizeBytes (%d), got %d", targets[0].SizeBytes, targets[0].EligibleBytes)
		}
	})

	t.Run("exclude paths", func(t *testing.T) {
		targets, err := c.Scan(context.Background(), domain.EcosystemRules{
			Enabled:      true,
			MaxAgeDays:   0,
			ExcludePaths: []string{"recent-app/**"},
		})
		if err != nil {
			t.Fatalf("Scan error: %v", err)
		}
		if len(targets) != 0 {
			t.Errorf("expected 0 targets when excluded, got %d", len(targets))
		}
	})
}

func TestNextJSCleaner_Scan_SymlinkAndNoCache(t *testing.T) {
	tempDir := t.TempDir()
	targetApp := filepath.Join(tempDir, "real-app")
	cacheDir := filepath.Join(targetApp, ".next", "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "data"), []byte("data"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	symlinkDir := filepath.Join(tempDir, "symlink-app")
	if err := os.Symlink(targetApp, symlinkDir); err != nil {
		t.Fatalf("symlink failed: %v", err)
	}

	// App without cache directory
	noCacheApp := filepath.Join(tempDir, "no-cache-app", ".next", "server")
	if err := os.MkdirAll(noCacheApp, 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(noCacheApp, "index.js"), []byte("server"), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	c := New([]string{tempDir})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}

	// Only 1 target from real-app/.next/cache.
	// Symlink is skipped, and no-cache-app/.next has no cache dir so it emits 0 targets.
	if len(targets) != 1 {
		t.Fatalf("expected exactly 1 target, got %d", len(targets))
	}
	if targets[0].Path != cacheDir {
		t.Errorf("target.Path = %q, want %q", targets[0].Path, cacheDir)
	}
}

func TestNextJSCleaner_Scan_NonExistentRoot(t *testing.T) {
	c := New([]string{"/non/existent/root/path/12345"})
	_, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Errorf("expected error scanning non-existent root, got nil")
	}
}

func TestNextJSCleaner_Clean_Error(t *testing.T) {
	tempDir := t.TempDir()
	readOnlyDir := filepath.Join(tempDir, "protected")
	if err := os.MkdirAll(filepath.Join(readOnlyDir, "child"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	// Make parent dir read-only so child cannot be removed
	if err := os.Chmod(readOnlyDir, 0o555); err != nil {
		t.Fatalf("chmod failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(readOnlyDir, 0o755)
	})

	c := New(nil)
	targets := []domain.Target{
		{Path: filepath.Join(readOnlyDir, "child")},
	}
	if err := c.Clean(context.Background(), targets); err == nil {
		t.Errorf("expected error removing child from read-only directory, got nil")
	}
}
