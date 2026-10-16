package dotnet

import (
	"context"
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

	targets, err := c.Scan(context.Background(), rules)
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

func TestDotNet_NuGetPackages_LockedToReviewOnly(t *testing.T) {
	for _, maxAge := range []int{0, 30} {
		t.Run(filepath.Join("MaxAge", string(rune('0'+maxAge))), func(t *testing.T) {
			nugetDir := t.TempDir()
			writeFile(t, filepath.Join(nugetDir, "Newtonsoft.Json", "13.0.1", "newtonsoft.json.13.0.1.nupkg"), 5000)

			oldTime := time.Now().AddDate(0, 0, -60)
			_ = os.Chtimes(nugetDir, oldTime, oldTime)

			paths := platform.Paths{
				DotNetNuGet: nugetDir,
			}
			c := New(paths, nil)
			rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: maxAge}

			targets, err := c.Scan(context.Background(), rules)
			if err != nil {
				t.Fatalf("Scan failed: %v", err)
			}
			if len(targets) != 1 {
				t.Fatalf("expected 1 target, got %d", len(targets))
			}

			tgt := targets[0]
			if tgt.Category != "nuget-packages" {
				t.Fatalf("expected category 'nuget-packages', got %q", tgt.Category)
			}
			if !tgt.ReviewOnly {
				t.Errorf("expected ReviewOnly == true, got false")
			}
			if tgt.ReviewReason != domain.ReviewReasonSharedStore {
				t.Errorf("expected ReviewReason == %q, got %q", domain.ReviewReasonSharedStore, tgt.ReviewReason)
			}
			if tgt.EligibleBytes != 0 {
				t.Errorf("expected EligibleBytes == 0, got %d", tgt.EligibleBytes)
			}
			if tgt.TTLEligible {
				t.Errorf("expected TTLEligible == false, got true")
			}
			if tgt.SizeBytes != 5000 {
				t.Errorf("expected SizeBytes == 5000, got %d", tgt.SizeBytes)
			}
		})
	}
}

func TestDotNet_NuGetHttpAndTemp_RemainTTLEligible(t *testing.T) {
	httpDir := t.TempDir()
	tempDir := t.TempDir()

	writeFile(t, filepath.Join(httpDir, "v3-cache", "item.dat"), 1200)
	writeFile(t, filepath.Join(tempDir, "NuGetScratch", "temp.dat"), 800)

	oldTime := time.Now().AddDate(0, 0, -45)
	_ = os.Chtimes(httpDir, oldTime, oldTime)
	_ = os.Chtimes(tempDir, oldTime, oldTime)

	paths := platform.Paths{
		DotNetHTTP: httpDir,
		DotNetTemp: tempDir,
	}
	c := New(paths, nil)
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}

	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets, got %d", len(targets))
	}

	for _, tgt := range targets {
		if tgt.ReviewOnly {
			t.Errorf("target %s: expected ReviewOnly == false, got true", tgt.Category)
		}
		if !tgt.TTLEligible {
			t.Errorf("target %s: expected TTLEligible == true, got false", tgt.Category)
		}
		if tgt.EligibleBytes != tgt.SizeBytes || tgt.EligibleBytes == 0 {
			t.Errorf("target %s: expected EligibleBytes == SizeBytes (%d), got %d", tgt.Category, tgt.SizeBytes, tgt.EligibleBytes)
		}
	}
}

func TestDotNet_Clean_DefensiveGuard_PreservesNuGet(t *testing.T) {
	testCases := []struct {
		name   string
		target func(nugetDir string) domain.Target
	}{
		{
			name: "Target marked ReviewOnly",
			target: func(nugetDir string) domain.Target {
				return domain.Target{
					ID:         "target-1",
					Path:       nugetDir,
					ReviewOnly: true,
				}
			},
		},
		{
			name: "Target with category nuget-packages",
			target: func(nugetDir string) domain.Target {
				return domain.Target{
					ID:       "target-2",
					Path:     nugetDir,
					Category: "nuget-packages",
				}
			},
		},
		{
			name: "Target matching DotNetNuGet path without flags",
			target: func(nugetDir string) domain.Target {
				return domain.Target{
					ID:   "target-3",
					Path: nugetDir,
				}
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			nugetDir := t.TempDir()
			sentinelPath := filepath.Join(nugetDir, "PackageA", "1.0.0", "PackageA.nupkg")
			writeFile(t, sentinelPath, 2048)

			infoBefore, err := os.Stat(sentinelPath)
			if err != nil {
				t.Fatalf("failed to stat sentinel before: %v", err)
			}

			paths := platform.Paths{
				DotNetNuGet: nugetDir,
			}
			c := New(paths, nil)

			err = c.Clean(context.Background(), []domain.Target{tc.target(nugetDir)})
			if err != nil {
				t.Fatalf("Clean failed: %v", err)
			}

			infoAfter, err := os.Stat(sentinelPath)
			if err != nil {
				t.Fatalf("sentinel file was deleted or unreadable: %v", err)
			}
			if infoAfter.Size() != infoBefore.Size() {
				t.Errorf("sentinel file size changed: before %d, after %d", infoBefore.Size(), infoAfter.Size())
			}
			if !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
				t.Errorf("sentinel file modTime changed: before %v, after %v", infoBefore.ModTime(), infoAfter.ModTime())
			}
		})
	}
}

func TestDotNet_IsInstalled(t *testing.T) {
	tempDir := t.TempDir()
	nugetDir := filepath.Join(tempDir, "nuget")
	_ = os.MkdirAll(nugetDir, 0o755)

	paths := platform.Paths{
		DotNetNuGet: nugetDir,
	}
	c := New(paths, nil)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when DotNetNuGet dir exists")
	}

	emptyPaths := platform.Paths{
		DotNetNuGet: filepath.Join(tempDir, "nonexistent1"),
		DotNetHTTP:  filepath.Join(tempDir, "nonexistent2"),
		DotNetTemp:  filepath.Join(tempDir, "nonexistent3"),
	}
	cEmpty := New(emptyPaths, nil)
	// If dotnet is in system path, it returns true, otherwise false without panic
	_ = cEmpty.IsInstalled()
}

func TestDotNet_Clean_EligibleAndSafety(t *testing.T) {
	tempDir := t.TempDir()
	cleanDir := filepath.Join(tempDir, "to_delete")
	writeFile(t, filepath.Join(cleanDir, "file.dll"), 512)

	outsideDir := filepath.Join(tempDir, "outside")
	writeFile(t, filepath.Join(outsideDir, "safe.dll"), 512)

	symlinkDir := filepath.Join(tempDir, "symlink_dir")
	_ = os.Symlink(outsideDir, symlinkDir)

	c := New(platform.Paths{}, nil)

	targets := []domain.Target{
		{
			ID:       "clean-me",
			Path:     cleanDir,
			Category: "dotnet-build",
		},
		{
			ID:       "symlink-target",
			Path:     symlinkDir,
			Category: "dotnet-build",
		},
		{
			ID:       "blacklisted-root",
			Path:     "/",
			Category: "dotnet-build",
		},
	}

	err := c.Clean(context.Background(), targets)
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// cleanDir must be deleted
	if _, err := os.Stat(cleanDir); !os.IsNotExist(err) {
		t.Errorf("expected cleanDir to be deleted")
	}

	// symlink target outsideDir must remain intact
	if _, err := os.Stat(filepath.Join(outsideDir, "safe.dll")); err != nil {
		t.Errorf("expected outsideDir to remain intact: %v", err)
	}
}


