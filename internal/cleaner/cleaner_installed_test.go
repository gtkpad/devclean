package cleaner_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner/ai"
	"github.com/gtkpad/devclean/internal/cleaner/bun"
	"github.com/gtkpad/devclean/internal/cleaner/cocoapods"
	"github.com/gtkpad/devclean/internal/cleaner/cpp"
	"github.com/gtkpad/devclean/internal/cleaner/docker"
	"github.com/gtkpad/devclean/internal/cleaner/dotnet"
	"github.com/gtkpad/devclean/internal/cleaner/elixir"
	"github.com/gtkpad/devclean/internal/cleaner/flutter"
	"github.com/gtkpad/devclean/internal/cleaner/golang"
	"github.com/gtkpad/devclean/internal/cleaner/homebrew"
	"github.com/gtkpad/devclean/internal/cleaner/java"
	"github.com/gtkpad/devclean/internal/cleaner/nextjs"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/pnpm"
	"github.com/gtkpad/devclean/internal/cleaner/python"
	"github.com/gtkpad/devclean/internal/cleaner/rust"
	"github.com/gtkpad/devclean/internal/cleaner/shell"
	"github.com/gtkpad/devclean/internal/cleaner/terraform"
	"github.com/gtkpad/devclean/internal/cleaner/xcode"
	"github.com/gtkpad/devclean/internal/domain"
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

func TestCleaners_IsInstalled_ManifestInScanPaths(t *testing.T) {
	// Isolate PATH and HOME so no toolchains or global caches are found
	emptyDir := t.TempDir()
	t.Setenv("PATH", emptyDir)
	t.Setenv("HOME", emptyDir)
	t.Setenv("CONAN_HOME", filepath.Join(emptyDir, "nonexistent"))
	t.Setenv("CONAN_USER_HOME", filepath.Join(emptyDir, "nonexistent"))
	t.Setenv("PUB_CACHE", filepath.Join(emptyDir, "nonexistent"))
	t.Setenv("TF_PLUGIN_CACHE_DIR", filepath.Join(emptyDir, "nonexistent"))

	scanRoot := t.TempDir()

	t.Run("cpp with CMakeLists.txt", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "cpp_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.10)"), 0644)

		c := cpp.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("cpp cleaner should report IsInstalled() == true when CMakeLists.txt exists in scanPaths")
		}

		cEmpty := cpp.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("cpp cleaner should report IsInstalled() == false when no tools, caches, or manifests exist")
		}
	})

	t.Run("flutter with pubspec.yaml", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "flutter_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "pubspec.yaml"), []byte("name: test_app"), 0644)

		c := flutter.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("flutter cleaner should report IsInstalled() == true when pubspec.yaml exists in scanPaths")
		}

		cEmpty := flutter.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("flutter cleaner should report IsInstalled() == false when no tools, caches, or manifests exist")
		}
	})

	t.Run("terraform with main.tf", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "tf_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "main.tf"), []byte("terraform {}"), 0644)

		c := terraform.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("terraform cleaner should report IsInstalled() == true when .tf exists in scanPaths")
		}

		cEmpty := terraform.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("terraform cleaner should report IsInstalled() == false when no tools, caches, or manifests exist")
		}
	})

	t.Run("cocoapods with Podfile", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "ios_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "Podfile"), []byte("target 'App' do\nend"), 0644)

		c := cocoapods.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("cocoapods cleaner should report IsInstalled() == true when Podfile exists in scanPaths")
		}

		cEmpty := cocoapods.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("cocoapods cleaner should report IsInstalled() == false when no tools, caches, or manifests exist")
		}
	})

	t.Run("elixir with mix.exs", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "ex_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "mix.exs"), []byte("defmodule Test.MixProject do\nend"), 0644)

		c := elixir.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("elixir cleaner should report IsInstalled() == true when mix.exs exists in scanPaths")
		}

		cEmpty := elixir.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("elixir cleaner should report IsInstalled() == false when no tools, caches, or manifests exist")
		}
	})

	t.Run("nextjs with next.config.js", func(t *testing.T) {
		projDir := filepath.Join(scanRoot, "web_proj")
		_ = os.MkdirAll(projDir, 0755)
		_ = os.WriteFile(filepath.Join(projDir, "next.config.js"), []byte("module.exports = {}"), 0644)

		c := nextjs.New([]string{scanRoot})
		if !c.IsInstalled() {
			t.Errorf("nextjs cleaner should report IsInstalled() == true when next.config.js exists in scanPaths")
		}

		cEmpty := nextjs.New([]string{emptyDir})
		if cEmpty.IsInstalled() {
			t.Errorf("nextjs cleaner should report IsInstalled() == false when no tools or manifests exist")
		}
	})
}

func TestCleaners_IsInstalled_InstantResponse(t *testing.T) {
	tempDir := t.TempDir()
	resolver := platform.New()
	paths := resolver.Resolve()

	allCleaners := []domain.Cleaner{
		docker.New(),
		golang.New(),
		node.New([]string{tempDir}),
		dotnet.New(paths, []string{tempDir}),
		python.New(paths.PythonUVCache, []string{tempDir}),
		rust.New([]string{tempDir}),
		java.New([]string{tempDir}),
		ai.New([]string{tempDir}),
		pnpm.New(),
		bun.New(),
		nextjs.New([]string{tempDir}),
		xcode.New(),
		homebrew.New(),
		cpp.New([]string{tempDir}),
		flutter.New([]string{tempDir}),
		terraform.New([]string{tempDir}),
		cocoapods.New([]string{tempDir}),
		elixir.New([]string{tempDir}),
		shell.New(),
	}

	// Warm-up pass to prime OS path lookups and socket initialization
	for _, c := range allCleaners {
		_ = c.IsInstalled()
	}

	start := time.Now()
	for _, c := range allCleaners {
		_ = c.IsInstalled()
	}
	elapsed := time.Since(start)

	if elapsed > 100*time.Millisecond {
		t.Errorf("IsInstalled() execution for all 19 cleaners took %v, want < 100ms", elapsed)
	}
}



