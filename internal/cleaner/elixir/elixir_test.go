package elixir

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestElixirCleaner_Name(t *testing.T) {
	c := New([]string{})
	if c.Name() != "elixir" {
		t.Fatalf("expected name 'elixir', got %q", c.Name())
	}
}

func TestElixirCleaner_IsInstalled(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("HEX_HOME", "")

	// 1. Isolated environment: no mix/elixir/iex, no mix.exs, no hex cache
	cIsolated := New([]string{})
	if cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be false in isolated environment")
	}

	// 2. ScanPaths containing a mix.exs
	projDir := filepath.Join(tmpHome, "workspace", "phoenix_app")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("failed to create projDir: %v", err)
	}
	mixExs := filepath.Join(projDir, "mix.exs")
	if err := os.WriteFile(mixExs, []byte("defmodule PhoenixApp.MixProject do\n  use Mix.Project\nend\n"), 0o644); err != nil {
		t.Fatalf("failed to write mix.exs: %v", err)
	}

	cWithScanPaths := New([]string{filepath.Join(tmpHome, "workspace")})
	if !cWithScanPaths.IsInstalled() {
		t.Error("expected IsInstalled to be true when mix.exs is present in scanPaths")
	}

	// 3. Global Hex packages cache exists (~/.hex/packages)
	hexDir := filepath.Join(tmpHome, ".hex", "packages")
	if err := os.MkdirAll(hexDir, 0o755); err != nil {
		t.Fatalf("failed to create hexDir: %v", err)
	}

	cWithCacheOnly := New([]string{})
	if !cWithCacheOnly.IsInstalled() {
		t.Error("expected IsInstalled to be true when ~/.hex/packages exists")
	}

	// 4. Custom HEX_HOME environment variable override
	customHex := filepath.Join(tmpHome, "custom_hex", "packages")
	if err := os.MkdirAll(customHex, 0o755); err != nil {
		t.Fatalf("failed to create customHex: %v", err)
	}
	_ = os.RemoveAll(filepath.Join(tmpHome, ".hex"))
	t.Setenv("HEX_HOME", filepath.Join(tmpHome, "custom_hex"))

	cWithCustomHex := New([]string{})
	if !cWithCustomHex.IsInstalled() {
		t.Error("expected IsInstalled to be true when $HEX_HOME/packages exists")
	}
}

func TestElixirCleaner_Scan_WithMixExs(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_elixir_app")
	buildDir := filepath.Join(proj, "_build")
	depsDir := filepath.Join(proj, "deps")

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create _build dir: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		t.Fatalf("failed to create deps dir: %v", err)
	}

	// Create sibling mix.exs and mix.lock in parent dir
	mixExs := filepath.Join(proj, "mix.exs")
	if err := os.WriteFile(mixExs, []byte("defmodule App.MixProject do\nend\n"), 0o644); err != nil {
		t.Fatalf("failed to write mix.exs: %v", err)
	}
	mixLock := filepath.Join(proj, "mix.lock")
	if err := os.WriteFile(mixLock, []byte("%{}\n"), 0o644); err != nil {
		t.Fatalf("failed to write mix.lock: %v", err)
	}

	// Create dummy compiled artifacts and dependency files
	if err := os.WriteFile(filepath.Join(buildDir, "dev.beam"), []byte("beam bytecode"), 0o644); err != nil {
		t.Fatalf("failed to write dev.beam: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depsDir, "jason.tar"), []byte("downloaded dep"), 0o644); err != nil {
		t.Fatalf("failed to write jason.tar: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundBuild, foundDeps *domain.Target
	for i := range targets {
		if targets[i].Path == buildDir {
			foundBuild = &targets[i]
		}
		if targets[i].Path == depsDir {
			foundDeps = &targets[i]
		}
	}

	// Verify _build target (RiskLow)
	if foundBuild == nil {
		t.Fatalf("expected _build target %s to be found", buildDir)
	}
	if foundBuild.Category != "elixir-build" {
		t.Errorf("expected category 'elixir-build', got %q", foundBuild.Category)
	}
	if foundBuild.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low' for _build, got %q", foundBuild.Risk)
	}
	if foundBuild.SizeBytes <= 0 {
		t.Errorf("expected size > 0 for _build, got %d", foundBuild.SizeBytes)
	}
	if foundBuild.ID == "" {
		t.Error("expected non-empty ID for _build target")
	}
	if foundBuild.Name != "my_elixir_app/_build" {
		t.Errorf("expected name 'my_elixir_app/_build', got %q", foundBuild.Name)
	}

	// Verify deps target (RiskMedium)
	if foundDeps == nil {
		t.Fatalf("expected deps target %s to be found", depsDir)
	}
	if foundDeps.Category != "elixir-deps" {
		t.Errorf("expected category 'elixir-deps', got %q", foundDeps.Category)
	}
	if foundDeps.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium' for deps, got %q", foundDeps.Risk)
	}
	if foundDeps.SizeBytes <= 0 {
		t.Errorf("expected size > 0 for deps, got %d", foundDeps.SizeBytes)
	}
	if foundDeps.ID == "" {
		t.Error("expected non-empty ID for deps target")
	}
	if foundDeps.Name != "my_elixir_app/deps" {
		t.Errorf("expected name 'my_elixir_app/deps', got %q", foundDeps.Name)
	}
}

func TestElixirCleaner_Scan_IgnoreWithoutMixExs(t *testing.T) {
	tmp := t.TempDir()
	genericProj := filepath.Join(tmp, "generic_c_project")
	buildDir := filepath.Join(genericProj, "_build")
	depsDir := filepath.Join(genericProj, "deps")

	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create _build: %v", err)
	}
	if err := os.MkdirAll(depsDir, 0o755); err != nil {
		t.Fatalf("failed to create deps: %v", err)
	}

	// No mix.exs - only a generic Makefile
	if err := os.WriteFile(filepath.Join(genericProj, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "main.o"), []byte("object code"), 0o644); err != nil {
		t.Fatalf("failed to write main.o: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depsDir, "external.h"), []byte("// header"), 0o644); err != nil {
		t.Fatalf("failed to write external.h: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildDir {
			t.Fatalf("_build directory %s should NOT have been claimed without sibling mix.exs", buildDir)
		}
		if target.Path == depsDir {
			t.Fatalf("deps directory %s should NOT have been claimed without sibling mix.exs", depsDir)
		}
	}
}

func TestElixirCleaner_Scan_GlobalHexCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Test default ~/.hex/packages
	defaultHexPackages := filepath.Join(tmpHome, ".hex", "packages")
	if err := os.MkdirAll(defaultHexPackages, 0o755); err != nil {
		t.Fatalf("failed to create defaultHexPackages: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultHexPackages, "phoenix-1.7.0.tar"), []byte("hex package data"), 0o644); err != nil {
		t.Fatalf("failed to write hex package: %v", err)
	}

	c := New([]string{})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundDefault *domain.Target
	for i := range targets {
		if targets[i].Path == defaultHexPackages {
			foundDefault = &targets[i]
			break
		}
	}

	if foundDefault == nil {
		t.Fatalf("expected default Hex cache target %s to be found", defaultHexPackages)
	}
	if foundDefault.Category != "hex-packages" {
		t.Errorf("expected category 'hex-packages', got %q", foundDefault.Category)
	}
	if foundDefault.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundDefault.Risk)
	}
	if foundDefault.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundDefault.SizeBytes)
	}
	if foundDefault.Name != "Hex Packages Cache" {
		t.Errorf("expected target name 'Hex Packages Cache', got %q", foundDefault.Name)
	}

	// Test custom $HEX_HOME/packages override
	customHexDir := filepath.Join(tmpHome, "custom_hex")
	customHexPackages := filepath.Join(customHexDir, "packages")
	if err := os.MkdirAll(customHexPackages, 0o755); err != nil {
		t.Fatalf("failed to create customHexPackages: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customHexPackages, "ecto-3.10.0.tar"), []byte("ecto data"), 0o644); err != nil {
		t.Fatalf("failed to write custom hex package: %v", err)
	}
	t.Setenv("HEX_HOME", customHexDir)

	targets, err = c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundCustom *domain.Target
	for i := range targets {
		if targets[i].Path == customHexPackages {
			foundCustom = &targets[i]
			break
		}
	}

	if foundCustom == nil {
		t.Fatalf("expected custom Hex cache target %s to be found", customHexPackages)
	}
	if foundCustom.Category != "hex-packages" {
		t.Errorf("expected category 'hex-packages', got %q", foundCustom.Category)
	}
	if foundCustom.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundCustom.Risk)
	}
}

func TestElixirCleaner_Clean(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target_elixir_to_remove")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "beam.bin"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	c := New([]string{})
	target := domain.Target{
		ID:       "elixir-test-id",
		Name:     "target",
		Path:     targetDir,
		Category: "elixir-build",
		Risk:     domain.RiskLow,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("expected targetDir to be removed, got err: %v", err)
	}
}

func TestElixirCleaner_Clean_Error(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "locked_parent")
	childDir := filepath.Join(parentDir, "_build")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(parentDir, 0o777)
	})

	if err := os.Chmod(parentDir, 0o555); err != nil {
		t.Fatalf("failed to chmod parent: %v", err)
	}

	c := New([]string{})
	target := domain.Target{Path: childDir}
	err := c.Clean(context.Background(), []domain.Target{target})
	if err == nil {
		t.Error("expected error when removing child of read-only parent directory, got nil")
	}
}

func TestElixirCleaner_Scan_SymlinkAndExclusions(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_elixir_proj")
	buildDir := filepath.Join(proj, "_build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "mix.exs"), []byte("defmodule Proj.MixProject do\nend\n"), 0o644); err != nil {
		t.Fatalf("failed to write mix.exs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "dummy"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	// Symlink pointing to proj
	linkDir := filepath.Join(tmp, "link_to_proj")
	if err := os.Symlink(proj, linkDir); err != nil {
		t.Skip("symlinks not supported")
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{"my_elixir_proj/**"},
	}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildDir {
			t.Errorf("buildDir %s should have been excluded by pattern", buildDir)
		}
	}
}

func TestElixirCleaner_Scan_GitSkipped(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	buildInGit := filepath.Join(gitDir, "_build")
	if err := os.MkdirAll(buildInGit, 0o755); err != nil {
		t.Fatalf("failed to create buildInGit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "mix.exs"), []byte("git mix.exs\n"), 0o644); err != nil {
		t.Fatalf("failed to write mix.exs: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildInGit {
			t.Errorf(".git internal _build dir should have been skipped")
		}
	}
}

func TestElixirCleaner_Scan_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New([]string{t.TempDir()})
	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
