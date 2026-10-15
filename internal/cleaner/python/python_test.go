package python

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestPythonCleaner_Name(t *testing.T) {
	c := New("/tmp/uv", nil)
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

	c := New(dir, nil)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	var uvTargets []domain.Target
	for _, tgt := range targets {
		if tgt.Category == "uv-cache" {
			uvTargets = append(uvTargets, tgt)
		}
	}
	if len(uvTargets) != 1 {
		t.Fatalf("expected 1 uv-cache target, got %d", len(uvTargets))
	}
	if uvTargets[0].SizeBytes != 1200 {
		t.Errorf("expected 1200 bytes, got %d", uvTargets[0].SizeBytes)
	}
	if uvTargets[0].Risk != domain.RiskLow {
		t.Errorf("expected low risk, got %s", uvTargets[0].Risk)
	}
}

func TestPythonCleaner_Scan_NonExistent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	c := New("/nonexistent/uv/cache", nil)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}
}

func TestPythonCleaner_Scan_RecentCacheReportedNotEligible(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "data"), 100)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	c := New(dir, nil)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target for recent cache, got %d", len(targets))
	}
	if targets[0].TTLEligible {
		t.Errorf("expected TTLEligible = false for recent uv cache")
	}
	if targets[0].EligibleBytes != 0 {
		t.Errorf("expected EligibleBytes = 0 for recent uv cache, got %d", targets[0].EligibleBytes)
	}
	if targets[0].SizeBytes <= 0 {
		t.Errorf("expected SizeBytes > 0, got %d", targets[0].SizeBytes)
	}
}

func TestPythonCleaner_Scan_ExcludePaths(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "data"), 100)
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(dir, oldTime, oldTime)

	t.Setenv("HOME", t.TempDir())
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	c := New(dir, nil)
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{dir},
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with exclude, got %d", len(targets))
	}
}

func TestPythonCleaner_IsInstalled_CacheExists(t *testing.T) {
	dir := t.TempDir()
	c := New(dir, nil)
	if !c.IsInstalled() {
		t.Error("expected IsInstalled=true when cache dir exists")
	}
}

func TestScan_PipCache(t *testing.T) {
	tmp := t.TempDir()
	pipDir := filepath.Join(tmp, "pip")
	os.MkdirAll(pipDir, 0o755)
	writeFile(t, filepath.Join(pipDir, "wheels", "pkg.whl"), 500)

	t.Setenv("PIP_CACHE_DIR", pipDir)
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	c := New("/nonexistent", nil)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Category == "pip-cache" {
			found = true
			if tgt.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow, got %s", tgt.Risk)
			}
		}
	}
	if !found {
		t.Error("expected pip-cache target")
	}
}

func TestScan_CondaPkgs(t *testing.T) {
	tmp := t.TempDir()
	pkgsDir := filepath.Join(tmp, "pkgs")
	os.MkdirAll(pkgsDir, 0o755)
	writeFile(t, filepath.Join(pkgsDir, "numpy-1.0/data"), 1000)

	t.Setenv("CONDA_PREFIX", tmp)
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")

	c := New("/nonexistent", nil)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Category == "conda-pkgs" {
			found = true
			if tgt.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow, got %s", tgt.Risk)
			}
		}
	}
	if !found {
		t.Error("expected conda-pkgs target")
	}
}

func TestScan_OrphanedVenv(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	// Orphan: .venv with no manifest
	orphan := filepath.Join(tmp, "orphan-project")
	os.MkdirAll(filepath.Join(orphan, ".venv", "lib"), 0o755)
	writeFile(t, filepath.Join(orphan, ".venv", "pyvenv.cfg"), 50)

	// Active: .venv with recent pyproject.toml
	active := filepath.Join(tmp, "active-project")
	os.MkdirAll(filepath.Join(active, ".venv", "lib"), 0o755)
	writeFile(t, filepath.Join(active, ".venv", "pyvenv.cfg"), 50)
	writeFile(t, filepath.Join(active, "pyproject.toml"), 100)

	// Stale: venv with old requirements.txt
	stale := filepath.Join(tmp, "stale-project")
	os.MkdirAll(filepath.Join(stale, "venv", "lib"), 0o755)
	writeFile(t, filepath.Join(stale, "venv", "pyvenv.cfg"), 50)
	writeFile(t, filepath.Join(stale, "requirements.txt"), 50)
	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(filepath.Join(stale, "requirements.txt"), oldTime, oldTime)

	c := New("/nonexistent", []string{tmp})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	venvPaths := make(map[string]domain.RiskLevel)
	for _, tgt := range targets {
		if tgt.Category == "python-venv-orphan" {
			venvPaths[tgt.Path] = tgt.Risk
		}
	}

	orphanVenv := filepath.Join(orphan, ".venv")
	if risk, ok := venvPaths[orphanVenv]; !ok {
		t.Error("orphan-project/.venv should be found")
	} else if risk != domain.RiskLow {
		t.Errorf("orphan venv should be RiskLow, got %s", risk)
	}

	activeVenv := filepath.Join(active, ".venv")
	if _, ok := venvPaths[activeVenv]; ok {
		t.Error("active-project/.venv should NOT be found (has recent manifest)")
	}

	staleVenv := filepath.Join(stale, "venv")
	if risk, ok := venvPaths[staleVenv]; !ok {
		t.Error("stale-project/venv should be found")
	} else if risk != domain.RiskMedium {
		t.Errorf("stale venv should be RiskMedium, got %s", risk)
	}
}

func TestScan_Pycache(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PIP_CACHE_DIR", "/nonexistent")
	t.Setenv("CONDA_PREFIX", "/nonexistent")

	// Create __pycache__ directories
	writeFile(t, filepath.Join(tmp, "myproject", "src", "__pycache__", "module.cpython-311.pyc"), 200)
	writeFile(t, filepath.Join(tmp, "myproject", "tests", "__pycache__", "test.cpython-311.pyc"), 150)

	c := New("/nonexistent", []string{tmp})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Category == "python-pycache" {
			found = true
			if tgt.SizeBytes != 350 {
				t.Errorf("expected aggregated size 350, got %d", tgt.SizeBytes)
			}
			if tgt.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow, got %s", tgt.Risk)
			}
		}
	}
	if !found {
		t.Error("expected python-pycache target")
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
