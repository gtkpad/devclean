package node

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func setupFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	oldTime := time.Now().AddDate(0, 0, -60)
	recentTime := time.Now().Add(-1 * time.Hour)

	// project-a: old node_modules + old lock
	mkdirAll(t, filepath.Join(root, "project-a", "node_modules", "some-pkg"))
	touchFile(t, filepath.Join(root, "project-a", "package-lock.json"))
	os.Chtimes(filepath.Join(root, "project-a", "node_modules"), oldTime, oldTime)
	os.Chtimes(filepath.Join(root, "project-a", "package-lock.json"), oldTime, oldTime)

	// project-b: recent node_modules + recent lock
	mkdirAll(t, filepath.Join(root, "project-b", "node_modules", "another-pkg"))
	touchFile(t, filepath.Join(root, "project-b", "yarn.lock"))
	os.Chtimes(filepath.Join(root, "project-b", "node_modules"), recentTime, recentTime)
	os.Chtimes(filepath.Join(root, "project-b", "yarn.lock"), recentTime, recentTime)

	// project-c: old node_modules + recent lock
	mkdirAll(t, filepath.Join(root, "project-c", "node_modules", "pkg"))
	touchFile(t, filepath.Join(root, "project-c", "pnpm-lock.yaml"))
	os.Chtimes(filepath.Join(root, "project-c", "node_modules"), oldTime, oldTime)
	os.Chtimes(filepath.Join(root, "project-c", "pnpm-lock.yaml"), recentTime, recentTime)

	// nested: outer node_modules with inner nested (should NOT be discovered separately)
	mkdirAll(t, filepath.Join(root, "nested", "outer", "node_modules", ".package", "node_modules", "inner-pkg"))
	os.Chtimes(filepath.Join(root, "nested", "outer", "node_modules"), oldTime, oldTime)

	// excluded: should be excluded via glob
	mkdirAll(t, filepath.Join(root, "excluded", "project-d", "node_modules", "excluded-pkg"))
	os.Chtimes(filepath.Join(root, "excluded", "project-d", "node_modules"), oldTime, oldTime)

	// symlink-dir -> project-a (should be skipped)
	os.Symlink(filepath.Join(root, "project-a"), filepath.Join(root, "symlink-dir"))

	return root
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func touchFile(t *testing.T, path string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
}

func TestNodeCleaner_Name(t *testing.T) {
	c := New(nil)
	if c.Name() != "node" {
		t.Errorf("expected 'node', got %q", c.Name())
	}
}

func TestNodeCleaner_IsInstalled(t *testing.T) {
	c := New(nil)
	if !c.IsInstalled() {
		t.Error("expected IsInstalled to always return true")
	}
}

func TestNodeCleaner_Scan_Discovery(t *testing.T) {
	root := setupFixture(t)
	c := New([]string{root})

	rules := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find: project-a, project-b, project-c, nested/outer, excluded/project-d
	// symlink-dir should be skipped
	// nested inner node_modules should NOT be found (SkipDir after outer)
	expectedPaths := map[string]bool{
		filepath.Join(root, "project-a", "node_modules"):             true,
		filepath.Join(root, "project-b", "node_modules"):             true,
		filepath.Join(root, "project-c", "node_modules"):             true,
		filepath.Join(root, "nested", "outer", "node_modules"):       true,
		filepath.Join(root, "excluded", "project-d", "node_modules"): true,
	}

	if len(targets) != len(expectedPaths) {
		t.Errorf("expected %d targets, got %d", len(expectedPaths), len(targets))
		for _, tgt := range targets {
			t.Logf("  found: %s", tgt.Path)
		}
	}

	for _, tgt := range targets {
		if !expectedPaths[tgt.Path] {
			t.Errorf("unexpected target: %s", tgt.Path)
		}
		// Verify nested inner was NOT discovered
		nestedInner := filepath.Join(root, "nested", "outer", "node_modules", ".package", "node_modules")
		if tgt.Path == nestedInner {
			t.Error("nested node_modules should not be discovered (SkipDir)")
		}
	}
}

func TestNodeCleaner_Scan_ExcludeGlobs(t *testing.T) {
	root := setupFixture(t)
	c := New([]string{root})

	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{"excluded/**"},
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tgt := range targets {
		if filepath.Base(filepath.Dir(tgt.Path)) == "project-d" {
			t.Errorf("excluded/project-d/node_modules should be excluded, got: %s", tgt.Path)
		}
	}

	// Should have 4 targets (project-a, project-b, project-c, nested/outer)
	if len(targets) != 4 {
		t.Errorf("expected 4 targets with exclude, got %d", len(targets))
		for _, tgt := range targets {
			t.Logf("  found: %s", tgt.Path)
		}
	}
}

func TestNodeCleaner_Scan_SymlinkSkipped(t *testing.T) {
	root := setupFixture(t)
	c := New([]string{root})

	rules := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// symlink-dir points to project-a, should not produce a duplicate target
	projectAPath := filepath.Join(root, "project-a", "node_modules")
	count := 0
	for _, tgt := range targets {
		if tgt.Path == projectAPath {
			count++
		}
	}
	if count > 1 {
		t.Errorf("project-a/node_modules appeared %d times (symlink was followed)", count)
	}
}

func TestNodeCleaner_Scan_RiskClassification(t *testing.T) {
	root := setupFixture(t)
	c := New([]string{root})

	rules := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	riskMap := make(map[string]domain.RiskLevel)
	for _, tgt := range targets {
		riskMap[tgt.Path] = tgt.Risk
	}

	tests := []struct {
		name     string
		path     string
		expected domain.RiskLevel
	}{
		{"old nm + old lock", filepath.Join(root, "project-a", "node_modules"), domain.RiskLow},
		{"recent nm + recent lock", filepath.Join(root, "project-b", "node_modules"), domain.RiskHigh},
		{"old nm + recent lock", filepath.Join(root, "project-c", "node_modules"), domain.RiskMedium},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := riskMap[tc.path]
			if !ok {
				t.Fatalf("target not found: %s", tc.path)
			}
			if got != tc.expected {
				t.Errorf("expected risk %s, got %s", tc.expected, got)
			}
		})
	}
}

func TestNodeCleaner_Scan_EmptyScanPaths(t *testing.T) {
	c := New([]string{})

	rules := domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for empty scan paths, got %d", len(targets))
	}
}

func TestNodeCleaner_Clean(t *testing.T) {
	root := t.TempDir()
	nmPath := filepath.Join(root, "test-project", "node_modules", "pkg")
	mkdirAll(t, nmPath)

	c := New(nil)
	targets := []domain.Target{
		{Path: filepath.Join(root, "test-project", "node_modules")},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "test-project", "node_modules")); !os.IsNotExist(err) {
		t.Error("node_modules should have been removed")
	}
}
