package golang

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestGoCleaner_Name(t *testing.T) {
	c := New()
	if c.Name() != "go" {
		t.Errorf("expected 'go', got %q", c.Name())
	}
}

func TestGoCleaner_IsInstalled(t *testing.T) {
	c := New()
	// Go should be installed in the test environment
	if !c.IsInstalled() {
		t.Skip("go not in PATH")
	}
}

func TestGoCleaner_Scan_WithFakeCaches(t *testing.T) {
	modDir := t.TempDir()
	buildDir := t.TempDir()

	// Create old files
	writeOldFile(t, filepath.Join(modDir, "pkg/mod/example.com@v1/go.mod"), 500)
	writeOldFile(t, filepath.Join(buildDir, "cache/ab/cdef"), 300)

	t.Setenv("GOMODCACHE", modDir)
	t.Setenv("GOCACHE", buildDir)

	// Set mod time to 90 days ago
	oldTime := time.Now().AddDate(0, 0, -90)
	_ = os.Chtimes(modDir, oldTime, oldTime)
	_ = os.Chtimes(buildDir, oldTime, oldTime)

	c := New()
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.SizeBytes == 0 {
			t.Errorf("target %s has 0 size", tgt.Category)
		}
		if tgt.Risk != domain.RiskLow {
			t.Errorf("target %s: expected low risk, got %s", tgt.Category, tgt.Risk)
		}
	}
}

func TestGoCleaner_Scan_RecentCacheFiltered(t *testing.T) {
	modDir := t.TempDir()
	writeOldFile(t, filepath.Join(modDir, "pkg/file.go"), 100)
	// Don't set old mod time — it stays recent

	t.Setenv("GOMODCACHE", modDir)
	t.Setenv("GOCACHE", "")

	c := New()
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for recent cache, got %d", len(targets))
	}
}

func TestGoCleaner_Scan_NonExistentPath(t *testing.T) {
	t.Setenv("GOMODCACHE", "/nonexistent/gomod/cache")
	t.Setenv("GOCACHE", "/nonexistent/go/cache")

	c := New()
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for non-existent paths, got %d", len(targets))
	}
}

func TestGoCleaner_Clean_ReadOnlyFiles(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "readonly.go")
	if err := os.WriteFile(file, []byte("test"), 0o444); err != nil {
		t.Fatal(err)
	}

	c := New()
	err := c.Clean([]domain.Target{{Path: dir}})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestGoCleaner_Scan_ExcludePaths(t *testing.T) {
	modDir := t.TempDir()
	writeOldFile(t, filepath.Join(modDir, "file.go"), 100)

	oldTime := time.Now().AddDate(0, 0, -90)
	_ = os.Chtimes(modDir, oldTime, oldTime)

	t.Setenv("GOMODCACHE", modDir)
	t.Setenv("GOCACHE", "")

	c := New()
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{modDir},
	}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with excluded path, got %d", len(targets))
	}
}

func writeOldFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}
