package node

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
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

	// Filter only node_modules targets (ignore global caches)
	var nodeTargets []domain.Target
	for _, tgt := range targets {
		if tgt.Category == "node" {
			nodeTargets = append(nodeTargets, tgt)
		}
	}

	if len(nodeTargets) != len(expectedPaths) {
		t.Errorf("expected %d node_modules targets, got %d", len(expectedPaths), len(nodeTargets))
		for _, tgt := range nodeTargets {
			t.Logf("  found: %s", tgt.Path)
		}
	}

	for _, tgt := range nodeTargets {
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
		if tgt.Category == "node" && filepath.Base(filepath.Dir(tgt.Path)) == "project-d" {
			t.Errorf("excluded/project-d/node_modules should be excluded, got: %s", tgt.Path)
		}
	}

	// Should have 4 node_modules targets (project-a, project-b, project-c, nested/outer)
	var nodeCount int
	for _, tgt := range targets {
		if tgt.Category == "node" {
			nodeCount++
		}
	}
	if nodeCount != 4 {
		t.Errorf("expected 4 node_modules targets with exclude, got %d", nodeCount)
		for _, tgt := range targets {
			if tgt.Category == "node" {
				t.Logf("  found: %s", tgt.Path)
			}
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
	// Override HOME and cache env vars to avoid finding real global caches
	tmp := t.TempDir() // empty dir with no caches
	t.Setenv("HOME", tmp)
	t.Setenv("PNPM_HOME", filepath.Join(tmp, "pnpm"))
	t.Setenv("YARN_CACHE_FOLDER", filepath.Join(tmp, "yarn"))
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
		for _, tgt := range targets {
			t.Logf("  found: %s (category: %s)", tgt.Path, tgt.Category)
		}
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

func TestNodeCleaner_Clean_DirectFilesystem(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, "puppeteer-cache")
	mkdirAll(t, filepath.Join(cacheDir, "chrome-mac"))
	touchFile(t, filepath.Join(cacheDir, "chrome-mac", "browser.bin"))

	c := New(nil)
	targets := []domain.Target{
		{Path: cacheDir, Category: "node-puppeteer"},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("expected cacheDir to be removed, but it still exists")
	}
}

func TestNodeCleaner_Clean_SymlinkProtection(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Outside directory with valuable files
	valuableDir := filepath.Join(tempDir, "valuable-code")
	mkdirAll(t, valuableDir)
	secretPath := filepath.Join(valuableDir, "secret.txt")
	if err := os.WriteFile(secretPath, []byte("super-secret"), 0o644); err != nil {
		t.Fatalf("failed to write secret file: %v", err)
	}

	// 2. Symlink pretending to be cache target pointing to outside valuable directory
	symlinkCache := filepath.Join(tempDir, "fake-cache-symlink")
	if err := os.Symlink(valuableDir, symlinkCache); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// 3. Blacklisted directory (.git)
	gitDir := filepath.Join(tempDir, ".git")
	mkdirAll(t, gitDir)
	touchFile(t, filepath.Join(gitDir, "config"))

	c := New(nil)
	targets := []domain.Target{
		{Path: symlinkCache, Category: "node-puppeteer"},
		{Path: gitDir, Category: "node"},
	}

	if err := c.Clean(context.Background(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify valuable directory and its contents are untouched
	if _, err := os.Stat(secretPath); err != nil {
		t.Errorf("valuable file was deleted or affected: %v", err)
	}
	data, err := os.ReadFile(secretPath)
	if err != nil || string(data) != "super-secret" {
		t.Errorf("valuable file data was corrupted: %v", err)
	}

	// Verify symlink itself was skipped (not removed or traversed)
	if _, err := os.Lstat(symlinkCache); err != nil {
		t.Errorf("symlink target was not skipped safely: %v", err)
	}

	// Verify blacklisted directory was skipped
	if _, err := os.Stat(filepath.Join(gitDir, "config")); err != nil {
		t.Errorf("blacklisted .git directory was deleted: %v", err)
	}
}

func TestScan_GlobalCaches(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// npm cache
	npmCache := filepath.Join(tmp, ".npm", "_cacache")
	os.MkdirAll(npmCache, 0o755)
	os.WriteFile(filepath.Join(npmCache, "data"), []byte("cache"), 0o644)

	// yarn cache
	yarnDir := filepath.Join(tmp, "yarn-cache")
	t.Setenv("YARN_CACHE_FOLDER", yarnDir)
	os.MkdirAll(yarnDir, 0o755)
	os.WriteFile(filepath.Join(yarnDir, "pkg.tgz"), []byte("data"), 0o644)

	c := New([]string{})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	categories := make(map[string]bool)
	for _, tgt := range targets {
		categories[tgt.Category] = true
		if tgt.Risk != domain.RiskLow {
			t.Errorf("global cache %s should have RiskLow, got %s", tgt.Category, tgt.Risk)
		}
	}

	if !categories["npm-cache"] {
		t.Error("expected npm-cache target")
	}
	if !categories["yarn-cache"] {
		t.Error("expected yarn-cache target")
	}
	if categories["pnpm-store"] {
		t.Error("unexpected pnpm-store target in node cleaner")
	}
	if categories["bun-cache"] {
		t.Error("unexpected bun-cache target in node cleaner")
	}
}

func TestScan_FrameworkOutput(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", "/nonexistent-home-for-test")

	// Nuxt app
	nuxtApp := filepath.Join(tmp, "nuxt-app")
	os.MkdirAll(filepath.Join(nuxtApp, ".nuxt"), 0o755)
	os.WriteFile(filepath.Join(nuxtApp, "nuxt.config.ts"), []byte("export default {}"), 0o644)
	os.WriteFile(filepath.Join(nuxtApp, ".nuxt", "data"), []byte("cached"), 0o644)

	c := New([]string{tmp})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	categories := make(map[string]bool)
	for _, tgt := range targets {
		categories[tgt.Category] = true
	}

	if !categories["nuxt-cache"] {
		t.Error("expected nuxt-cache target")
	}
	if categories["nextjs-cache"] {
		t.Error("unexpected nextjs-cache target in node cleaner")
	}
}

func TestScan_DistAncestorDetection(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", "/nonexistent-home-for-test")

	// Monorepo: dist/ with ancestor package.json 2 levels up
	mono := filepath.Join(tmp, "monorepo")
	os.MkdirAll(mono, 0o755)
	os.WriteFile(filepath.Join(mono, "package.json"), []byte("{}"), 0o644)
	os.MkdirAll(filepath.Join(mono, "packages", "ui", "dist"), 0o755)
	os.WriteFile(filepath.Join(mono, "packages", "ui", "dist", "bundle.js"), []byte("code"), 0o644)

	// Standalone: dist/ with sibling package.json
	standalone := filepath.Join(tmp, "standalone")
	os.MkdirAll(filepath.Join(standalone, "dist"), 0o755)
	os.WriteFile(filepath.Join(standalone, "package.json"), []byte("{}"), 0o644)
	os.WriteFile(filepath.Join(standalone, "dist", "bundle.js"), []byte("code"), 0o644)

	// Orphan: dist/ with no package.json anywhere
	orphan := filepath.Join(tmp, "orphan")
	os.MkdirAll(filepath.Join(orphan, "dist"), 0o755)
	os.WriteFile(filepath.Join(orphan, "dist", "output.bin"), []byte("data"), 0o644)

	c := New([]string{tmp})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	distPaths := make(map[string]bool)
	for _, tgt := range targets {
		if tgt.Category == "node-dist" {
			distPaths[tgt.Path] = true
		}
	}

	monoDist := filepath.Join(mono, "packages", "ui", "dist")
	standaloneDist := filepath.Join(standalone, "dist")
	orphanDist := filepath.Join(orphan, "dist")

	if !distPaths[monoDist] {
		t.Error("monorepo/packages/ui/dist should be found (ancestor package.json)")
	}
	if !distPaths[standaloneDist] {
		t.Error("standalone/dist should be found (sibling package.json)")
	}
	if distPaths[orphanDist] {
		t.Error("orphan/dist should NOT be found (no package.json)")
	}
}

func TestScan_DistExclusion(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", "/nonexistent-home-for-test")

	// C project: dist/ with sibling Makefile → NOT a JS project
	cproj := filepath.Join(tmp, "cproject")
	os.MkdirAll(filepath.Join(cproj, "dist"), 0o755)
	os.WriteFile(filepath.Join(cproj, "Makefile"), []byte("all:"), 0o644)
	os.WriteFile(filepath.Join(cproj, "package.json"), []byte("{}"), 0o644) // even with package.json
	os.WriteFile(filepath.Join(cproj, "dist", "binary"), []byte("bin"), 0o644)

	// Python project: dist/ with sibling setup.py → NOT a JS project
	pyproj := filepath.Join(tmp, "pyproject")
	os.MkdirAll(filepath.Join(pyproj, "dist"), 0o755)
	os.WriteFile(filepath.Join(pyproj, "setup.py"), []byte("setup()"), 0o644)
	os.WriteFile(filepath.Join(pyproj, "package.json"), []byte("{}"), 0o644) // even with package.json
	os.WriteFile(filepath.Join(pyproj, "dist", "wheel.whl"), []byte("whl"), 0o644)

	c := New([]string{tmp})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, tgt := range targets {
		if tgt.Category == "node-dist" {
			t.Errorf("should not find dist/ in non-JS project: %s", tgt.Path)
		}
	}
}

func TestNodeCleaner_Scan_GlobalCaches_AutomationAndNpm(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// Clean out any custom env vars that might leak from host environment
	t.Setenv("PUPPETEER_CACHE_DIR", "")
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", "")
	t.Setenv("CYPRESS_CACHE_FOLDER", "")

	dirs := map[string]string{
		"npm-cache":       filepath.Join(tempHome, ".npm", "_cacache"),
		"npx-cache":       filepath.Join(tempHome, ".npm", "_npx"),
		"npm-logs":        filepath.Join(tempHome, ".npm", "_logs"),
		"corepack-cache":  filepath.Join(tempHome, "Library", "Caches", "node", "corepack"),
		"node-puppeteer":  filepath.Join(tempHome, ".cache", "puppeteer"),
		"node-playwright": filepath.Join(tempHome, "Library", "Caches", "ms-playwright"),
		"node-cypress":    filepath.Join(tempHome, "Library", "Caches", "Cypress"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("failed to mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "sentinel.txt"), []byte("payload"), 0o644); err != nil {
			t.Fatalf("failed to write sentinel in %s: %v", dir, err)
		}
	}

	c := New([]string{})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	foundCategories := make(map[string]domain.Target)
	for _, tgt := range targets {
		foundCategories[tgt.Category] = tgt
	}

	for cat, dir := range dirs {
		tgt, exists := foundCategories[cat]
		if !exists {
			t.Errorf("expected target category %q to be discovered", cat)
			continue
		}
		if tgt.Path != dir {
			t.Errorf("category %q: expected path %q, got %q", cat, dir, tgt.Path)
		}
		if tgt.SizeBytes <= 0 {
			t.Errorf("category %q: expected SizeBytes > 0, got %d", cat, tgt.SizeBytes)
		}
		if !tgt.TTLEligible {
			t.Errorf("category %q: expected TTLEligible true with MaxAgeDays=0", cat)
		}
		if tgt.EligibleBytes != tgt.SizeBytes {
			t.Errorf("category %q: expected EligibleBytes == SizeBytes (%d), got %d", cat, tgt.SizeBytes, tgt.EligibleBytes)
		}
		if tgt.ID == "" || len(tgt.ID) != 12 {
			t.Errorf("category %q: expected 12-char hex ID, got %q", cat, tgt.ID)
		}
	}

	// Test environment overrides
	t.Run("environment overrides", func(t *testing.T) {
		customPup := filepath.Join(tempHome, "custom-puppeteer")
		customPlay := filepath.Join(tempHome, "custom-playwright")
		customCyp := filepath.Join(tempHome, "custom-cypress")

		t.Setenv("PUPPETEER_CACHE_DIR", customPup)
		t.Setenv("PLAYWRIGHT_BROWSERS_PATH", customPlay)
		t.Setenv("CYPRESS_CACHE_FOLDER", customCyp)

		for _, dir := range []string{customPup, customPlay, customCyp} {
			_ = os.MkdirAll(dir, 0o755)
			_ = os.WriteFile(filepath.Join(dir, "sentinel.txt"), []byte("override"), 0o644)
		}

		targetsOverride, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 0})
		if err != nil {
			t.Fatalf("Scan() with overrides failed: %v", err)
		}

		overridePaths := make(map[string]string)
		for _, tgt := range targetsOverride {
			overridePaths[tgt.Category] = tgt.Path
		}

		if overridePaths["node-puppeteer"] != customPup {
			t.Errorf("expected PUPPETEER_CACHE_DIR override %q, got %q", customPup, overridePaths["node-puppeteer"])
		}
		if overridePaths["node-playwright"] != customPlay {
			t.Errorf("expected PLAYWRIGHT_BROWSERS_PATH override %q, got %q", customPlay, overridePaths["node-playwright"])
		}
		if overridePaths["node-cypress"] != customCyp {
			t.Errorf("expected CYPRESS_CACHE_FOLDER override %q, got %q", customCyp, overridePaths["node-cypress"])
		}
	})
}

func TestNodeCleaner_RiskLevels_Automation(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	dirs := map[string]string{
		"npm-cache":       filepath.Join(tempHome, ".npm", "_cacache"),
		"npx-cache":       filepath.Join(tempHome, ".npm", "_npx"),
		"npm-logs":        filepath.Join(tempHome, ".npm", "_logs"),
		"corepack-cache":  filepath.Join(tempHome, ".cache", "node", "corepack"),
		"node-puppeteer":  filepath.Join(tempHome, ".cache", "puppeteer"),
		"node-playwright": filepath.Join(tempHome, ".cache", "ms-playwright"),
		"node-cypress":    filepath.Join(tempHome, ".cache", "Cypress"),
	}

	for _, dir := range dirs {
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "sentinel.txt"), []byte("risk-test"), 0o644)
	}

	c := New([]string{})
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{Enabled: true, MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("Scan() failed: %v", err)
	}

	riskMap := make(map[string]domain.RiskLevel)
	for _, tgt := range targets {
		riskMap[tgt.Category] = tgt.Risk
	}

	expectedRisks := map[string]domain.RiskLevel{
		"node-puppeteer":  domain.RiskMedium,
		"node-playwright": domain.RiskMedium,
		"node-cypress":    domain.RiskLow,
		"npm-cache":       domain.RiskLow,
		"npx-cache":       domain.RiskLow,
		"npm-logs":        domain.RiskLow,
		"corepack-cache":  domain.RiskLow,
	}

	for cat, wantRisk := range expectedRisks {
		gotRisk, ok := riskMap[cat]
		if !ok {
			t.Errorf("category %q not discovered", cat)
			continue
		}
		if gotRisk != wantRisk {
			t.Errorf("category %q: expected risk %v, got %v", cat, wantRisk, gotRisk)
		}
	}
}

func TestNodeCleaner_Clean_PermissionHandling(t *testing.T) {
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

	c := New(nil)
	targets := []domain.Target{
		{Path: file, Category: "node"},
	}

	// Must not panic
	_ = c.Clean(context.Background(), targets)
}


