package cleaner_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner/bun"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/homebrew"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/pnpm"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/cleaner/xcode"
	"github.com/gtkpad/devclean/internal/platform"
)

func TestCleaners_IsInstalled_NoBinaryInPath_CacheOnDisk(t *testing.T) {
	// Isolate PATH so no toolchain binaries are found
	emptyBinDir := t.TempDir()
	t.Setenv("PATH", emptyBinDir)

	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	t.Run("dotnet", func(t *testing.T) {
		nugetPath := filepath.Join(t.TempDir(), "nuget")
		if err := os.MkdirAll(nugetPath, 0755); err != nil {
			t.Fatal(err)
		}
		paths := platform.Paths{
			DotNetNuGet: nugetPath,
		}
		c := dotnet.New(paths, nil)
		if !c.IsInstalled() {
			t.Errorf("dotnet cleaner should report IsInstalled() == true when NuGet cache exists on disk without dotnet binary in PATH")
		}

		// When cache does not exist
		emptyPaths := platform.Paths{
			DotNetNuGet: filepath.Join(t.TempDir(), "nonexistent"),
		}
		cEmpty := dotnet.New(emptyPaths, nil)
		if cEmpty.IsInstalled() {
			t.Errorf("dotnet cleaner should report IsInstalled() == false when binary is absent and cache does not exist")
		}
	})

	t.Run("golang", func(t *testing.T) {
		modCache := filepath.Join(t.TempDir(), "pkg", "mod")
		if err := os.MkdirAll(modCache, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GOMODCACHE", modCache)
		c := golang.New()
		if !c.IsInstalled() {
			t.Errorf("golang cleaner should report IsInstalled() == true when GOMODCACHE exists on disk without go binary in PATH")
		}

		t.Setenv("GOMODCACHE", filepath.Join(t.TempDir(), "nonexistent"))
		t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "nonexistent"))
		t.Setenv("GOPATH", filepath.Join(t.TempDir(), "nonexistent"))
		cEmpty := golang.New()
		if cEmpty.IsInstalled() {
			t.Errorf("golang cleaner should report IsInstalled() == false when binary is absent and caches do not exist")
		}
	})

	t.Run("pnpm", func(t *testing.T) {
		pnpmStore := filepath.Join(fakeHome, "Library", "pnpm", "store")
		if runtime.GOOS != "darwin" {
			pnpmStore = filepath.Join(fakeHome, ".local", "share", "pnpm", "store")
		}
		if err := os.MkdirAll(pnpmStore, 0755); err != nil {
			t.Fatal(err)
		}
		c := pnpm.New()
		if !c.IsInstalled() {
			t.Errorf("pnpm cleaner should report IsInstalled() == true when store exists on disk without pnpm binary in PATH")
		}
	})

	t.Run("homebrew", func(t *testing.T) {
		brewCache := filepath.Join(fakeHome, "Library", "Caches", "Homebrew")
		if runtime.GOOS != "darwin" {
			brewCache = filepath.Join(fakeHome, ".cache", "Homebrew")
		}
		if err := os.MkdirAll(brewCache, 0755); err != nil {
			t.Fatal(err)
		}
		c := homebrew.New()
		if !c.IsInstalled() {
			t.Errorf("homebrew cleaner should report IsInstalled() == true when cache exists on disk without brew binary in PATH")
		}
	})

	t.Run("xcode", func(t *testing.T) {
		if runtime.GOOS != "darwin" {
			t.Skip("xcode cleaner is darwin-specific")
		}
		derivedData := filepath.Join(fakeHome, "Library", "Developer", "Xcode", "DerivedData")
		if err := os.MkdirAll(derivedData, 0755); err != nil {
			t.Fatal(err)
		}
		c := xcode.New()
		if !c.IsInstalled() {
			t.Errorf("xcode cleaner should report IsInstalled() == true when DerivedData exists on disk without xcrun in PATH")
		}
	})

	t.Run("rust", func(t *testing.T) {
		cargoHome := filepath.Join(t.TempDir(), ".cargo")
		cargoRegistry := filepath.Join(cargoHome, "registry")
		if err := os.MkdirAll(cargoRegistry, 0755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("CARGO_HOME", cargoHome)
		c := rust.New(nil)
		if !c.IsInstalled() {
			t.Errorf("rust cleaner should report IsInstalled() == true when CARGO_HOME registry exists on disk without cargo in PATH")
		}
	})

	t.Run("python", func(t *testing.T) {
		uvCache := filepath.Join(t.TempDir(), "uv-cache")
		if err := os.MkdirAll(uvCache, 0755); err != nil {
			t.Fatal(err)
		}
		c := python.New(uvCache, nil)
		if !c.IsInstalled() {
			t.Errorf("python cleaner should report IsInstalled() == true when uv cache exists on disk without python in PATH")
		}
	})

	t.Run("bun", func(t *testing.T) {
		bunCache := filepath.Join(t.TempDir(), "bun-cache")
		if err := os.MkdirAll(bunCache, 0755); err != nil {
			t.Fatal(err)
		}
		c := bun.NewWithPath(bunCache)
		if !c.IsInstalled() {
			t.Errorf("bun cleaner should report IsInstalled() == true when cache exists on disk without bun in PATH")
		}
	})

	t.Run("java", func(t *testing.T) {
		gradleCache := filepath.Join(fakeHome, ".gradle", "caches")
		if err := os.MkdirAll(gradleCache, 0755); err != nil {
			t.Fatal(err)
		}
		c := java.New(nil)
		if !c.IsInstalled() {
			t.Errorf("java cleaner should report IsInstalled() == true when .gradle/caches exists on disk without java/mvn/gradle in PATH")
		}
	})
}
