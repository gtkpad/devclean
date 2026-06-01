package dotnet

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/platform"
)

func TestDotNetCleaner_Name(t *testing.T) {
	c := New(platform.Paths{})
	if c.Name() != "dotnet" {
		t.Errorf("expected 'dotnet', got %q", c.Name())
	}
}

func TestDotNetCleaner_Scan_AllCaches(t *testing.T) {
	nuget := t.TempDir()
	http := t.TempDir()
	temp := t.TempDir()

	writeFile(t, filepath.Join(nuget, "pkg/lib.dll"), 1000)
	writeFile(t, filepath.Join(http, "cache/response"), 500)
	writeFile(t, filepath.Join(temp, "tmp/build"), 200)

	// Set old mod times
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(nuget, oldTime, oldTime)
	_ = os.Chtimes(http, oldTime, oldTime)
	_ = os.Chtimes(temp, oldTime, oldTime)

	paths := platform.Paths{
		DotNetNuGet: nuget,
		DotNetHTTP:  http,
		DotNetTemp:  temp,
	}
	c := New(paths)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 3 {
		t.Fatalf("expected 3 targets, got %d", len(targets))
	}
	for _, tgt := range targets {
		if tgt.Risk != domain.RiskLow {
			t.Errorf("%s: expected low risk, got %s", tgt.Category, tgt.Risk)
		}
	}
}

func TestDotNetCleaner_Scan_NonExistentPaths(t *testing.T) {
	paths := platform.Paths{
		DotNetNuGet: "/nonexistent/nuget",
		DotNetHTTP:  "/nonexistent/http",
		DotNetTemp:  "/nonexistent/temp",
	}
	c := New(paths)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets for non-existent paths, got %d", len(targets))
	}
}

func TestDotNetCleaner_Scan_ExcludePaths(t *testing.T) {
	nuget := t.TempDir()
	writeFile(t, filepath.Join(nuget, "pkg.dll"), 100)
	oldTime := time.Now().AddDate(0, 0, -60)
	_ = os.Chtimes(nuget, oldTime, oldTime)

	paths := platform.Paths{DotNetNuGet: nuget}
	c := New(paths)
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{nuget},
	}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with exclude, got %d", len(targets))
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
