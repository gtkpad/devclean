package python

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
)

func TestPythonCleaner_Name(t *testing.T) {
	c := New("/tmp/uv")
	if c.Name() != "python" {
		t.Errorf("expected 'python', got %q", c.Name())
	}
}

func TestPythonCleaner_Scan_CacheExists(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "wheels/pkg.whl"), 800)
	writeFile(t, filepath.Join(dir, "archives/pkg.tar.gz"), 400)

	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(dir, oldTime, oldTime)

	c := New(dir)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if targets[0].SizeBytes != 1200 {
		t.Errorf("expected 1200 bytes, got %d", targets[0].SizeBytes)
	}
	if targets[0].Category != "uv-cache" {
		t.Errorf("expected category 'uv-cache', got %q", targets[0].Category)
	}
	if targets[0].Risk != domain.RiskLow {
		t.Errorf("expected low risk, got %s", targets[0].Risk)
	}
}

func TestPythonCleaner_Scan_NonExistent(t *testing.T) {
	c := New("/nonexistent/uv/cache")
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

func TestPythonCleaner_Scan_RecentCacheFiltered(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "data"), 100)
	// Dir mod time is recent (just created)

	c := New(dir)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for recent cache, got %d", len(targets))
	}
}

func TestPythonCleaner_Scan_ExcludePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "data"), 100)
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(dir, oldTime, oldTime)

	c := New(dir)
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{dir},
	}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with exclude, got %d", len(targets))
	}
}

func TestPythonCleaner_IsInstalled_CacheExists(t *testing.T) {
	dir := t.TempDir()
	c := New(dir)
	if !c.IsInstalled() {
		t.Error("expected IsInstalled=true when cache dir exists")
	}
}

func writeFile(t *testing.T, path string, size int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}
