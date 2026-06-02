package dotnet

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/platform"
)

func TestDotNetCleaner_Name(t *testing.T) {
	c := New(platform.Paths{}, nil)
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
	c := New(paths, nil)
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
	c := New(paths, nil)
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
	c := New(paths, nil)
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

func TestDotNetCleaner_LocalScan_WithManifest(t *testing.T) {
	root := t.TempDir()

	// Create a .NET project with bin/ and obj/
	projectDir := filepath.Join(root, "MyApp")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "MyApp.csproj"), []byte("<Project/>"), 0o644)
	writeFile(t, filepath.Join(projectDir, "bin", "Debug", "app.dll"), 2048)
	writeFile(t, filepath.Join(projectDir, "obj", "project.assets.json"), 1024)

	c := New(platform.Paths{}, []string{root})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (bin + obj), got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.Category != "dotnet-build" {
			t.Errorf("expected category 'dotnet-build', got %q", tgt.Category)
		}
		if tgt.Risk != domain.RiskLow {
			t.Errorf("expected RiskLow, got %s", tgt.Risk)
		}
	}
}

func TestDotNetCleaner_LocalScan_WithoutManifest(t *testing.T) {
	root := t.TempDir()

	// Create bin/ and obj/ without any project manifest
	randomDir := filepath.Join(root, "random")
	os.MkdirAll(randomDir, 0o755)
	writeFile(t, filepath.Join(randomDir, "bin", "output.exe"), 4096)
	writeFile(t, filepath.Join(randomDir, "obj", "temp.cache"), 512)

	c := New(platform.Paths{}, []string{root})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (no manifest), got %d", len(targets))
	}
}

func TestDotNetCleaner_LocalScan_ExcludedPath(t *testing.T) {
	root := t.TempDir()

	projectDir := filepath.Join(root, "excluded-project")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "App.csproj"), []byte("<Project/>"), 0o644)
	writeFile(t, filepath.Join(projectDir, "bin", "app.dll"), 1024)

	c := New(platform.Paths{}, []string{root})
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   0,
		ExcludePaths: []string{"excluded-project/**"},
	}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets (excluded), got %d", len(targets))
	}
}

func TestDotNetCleaner_LocalScan_FsprojManifest(t *testing.T) {
	root := t.TempDir()

	// F# project
	projectDir := filepath.Join(root, "FSharpApp")
	os.MkdirAll(projectDir, 0o755)
	os.WriteFile(filepath.Join(projectDir, "FSharpApp.fsproj"), []byte("<Project/>"), 0o644)
	writeFile(t, filepath.Join(projectDir, "obj", "cache.json"), 512)

	c := New(platform.Paths{}, []string{root})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 0}

	targets, err := c.Scan(rules)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (obj), got %d", len(targets))
	}
	if targets[0].Category != "dotnet-build" {
		t.Errorf("expected category 'dotnet-build', got %q", targets[0].Category)
	}
}
