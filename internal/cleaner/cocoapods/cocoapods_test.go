package cocoapods

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestCocoaPodsCleaner_Name(t *testing.T) {
	c := New([]string{})
	if c.Name() != "cocoapods" {
		t.Fatalf("expected name 'cocoapods', got %q", c.Name())
	}
}

func TestCocoaPodsCleaner_IsInstalled(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("XDG_CACHE_HOME", "")

	// 1. Isolated environment: no pod executable, no scanPaths, no cache
	cIsolated := New([]string{})
	if cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be false in isolated environment")
	}

	// 2. ScanPaths containing a Podfile
	projDir := filepath.Join(tmpHome, "workspace", "my_app")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatalf("failed to create projDir: %v", err)
	}
	podfile := filepath.Join(projDir, "Podfile")
	if err := os.WriteFile(podfile, []byte("platform :ios, '14.0'\n"), 0o644); err != nil {
		t.Fatalf("failed to write Podfile: %v", err)
	}

	cWithScanPaths := New([]string{filepath.Join(tmpHome, "workspace")})
	if !cWithScanPaths.IsInstalled() {
		t.Error("expected IsInstalled to be true when Podfile is present in scanPaths")
	}

	// 3. Global cache directory exists
	cacheDir := filepath.Join(tmpHome, ".cache", "CocoaPods")
	if runtime.GOOS == "darwin" {
		cacheDir = filepath.Join(tmpHome, "Library", "Caches", "CocoaPods")
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("failed to create cacheDir: %v", err)
	}

	cWithCacheOnly := New([]string{})
	if !cWithCacheOnly.IsInstalled() {
		t.Error("expected IsInstalled to be true when global CocoaPods cache exists")
	}
}

func TestCocoaPodsCleaner_Scan_WithPodfile(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "ios_app")
	podsDir := filepath.Join(proj, "Pods")

	if err := os.MkdirAll(podsDir, 0o755); err != nil {
		t.Fatalf("failed to create Pods dir: %v", err)
	}

	// Create sibling Podfile and Podfile.lock in parent dir
	podfile := filepath.Join(proj, "Podfile")
	if err := os.WriteFile(podfile, []byte("target 'ios_app' do\n  pod 'AFNetworking'\nend\n"), 0o644); err != nil {
		t.Fatalf("failed to write Podfile: %v", err)
	}
	podfileLock := filepath.Join(proj, "Podfile.lock")
	if err := os.WriteFile(podfileLock, []byte("PODS:\n  - AFNetworking (4.0.0)\n"), 0o644); err != nil {
		t.Fatalf("failed to write Podfile.lock: %v", err)
	}

	// Create dummy dependency files inside Pods/
	if err := os.WriteFile(filepath.Join(podsDir, "Manifest.lock"), []byte("manifest"), 0o644); err != nil {
		t.Fatalf("failed to write Manifest.lock: %v", err)
	}
	subPodDir := filepath.Join(podsDir, "AFNetworking")
	if err := os.MkdirAll(subPodDir, 0o755); err != nil {
		t.Fatalf("failed to create subPodDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subPodDir, "AFNetworking.m"), []byte("// code"), 0o644); err != nil {
		t.Fatalf("failed to write pod source: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundPods *domain.Target
	for i := range targets {
		if targets[i].Path == podsDir {
			foundPods = &targets[i]
			break
		}
	}

	if foundPods == nil {
		t.Fatalf("expected Pods target %s to be found", podsDir)
	}
	if foundPods.Category != "cocoapods-pods" {
		t.Errorf("expected category 'cocoapods-pods', got %q", foundPods.Category)
	}
	if foundPods.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium' for Pods, got %q", foundPods.Risk)
	}
	if foundPods.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundPods.SizeBytes)
	}
	if foundPods.ID == "" {
		t.Error("expected non-empty ID for Pods target")
	}
	if foundPods.Name != "ios_app/Pods" {
		t.Errorf("expected target name 'ios_app/Pods', got %q", foundPods.Name)
	}
}

func TestCocoaPodsCleaner_Scan_IgnoreWithoutPodfile(t *testing.T) {
	tmp := t.TempDir()
	genericProj := filepath.Join(tmp, "kubernetes_setup")
	podsDir := filepath.Join(genericProj, "Pods")

	if err := os.MkdirAll(podsDir, 0o755); err != nil {
		t.Fatalf("failed to create Pods: %v", err)
	}

	// No Podfile - only a generic k8s manifest
	if err := os.WriteFile(filepath.Join(genericProj, "deployment.yaml"), []byte("apiVersion: apps/v1\n"), 0o644); err != nil {
		t.Fatalf("failed to write deployment.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(podsDir, "pod-spec.yaml"), []byte("kind: Pod\n"), 0o644); err != nil {
		t.Fatalf("failed to write pod-spec.yaml: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == podsDir {
			t.Fatalf("Pods directory %s should NOT have been claimed without sibling Podfile", podsDir)
		}
	}
}

func TestCocoaPodsCleaner_Scan_GlobalCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Test macOS Library/Caches/CocoaPods
	macCache := filepath.Join(tmpHome, "Library", "Caches", "CocoaPods")
	if err := os.MkdirAll(macCache, 0o755); err != nil {
		t.Fatalf("failed to create macCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(macCache, "cache_entry.bin"), []byte("cached pod data"), 0o644); err != nil {
		t.Fatalf("failed to write cache entry: %v", err)
	}

	// Test Linux ~/.cache/CocoaPods or $XDG_CACHE_HOME/CocoaPods
	xdgHome := filepath.Join(tmpHome, "custom_xdg")
	xdgCache := filepath.Join(xdgHome, "CocoaPods")
	if err := os.MkdirAll(xdgCache, 0o755); err != nil {
		t.Fatalf("failed to create xdgCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(xdgCache, "linux_cache.bin"), []byte("linux data"), 0o644); err != nil {
		t.Fatalf("failed to write linux cache entry: %v", err)
	}
	t.Setenv("XDG_CACHE_HOME", xdgHome)

	c := New([]string{})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundMac, foundXDG *domain.Target
	for i := range targets {
		if targets[i].Path == macCache {
			foundMac = &targets[i]
		}
		if targets[i].Path == xdgCache {
			foundXDG = &targets[i]
		}
	}

	if runtime.GOOS == "darwin" && foundMac == nil {
		t.Fatalf("expected macOS CocoaPods cache target %s to be found", macCache)
	}
	if foundMac != nil {
		if foundMac.Category != "cocoapods-cache" {
			t.Errorf("expected category 'cocoapods-cache', got %q", foundMac.Category)
		}
		if foundMac.Risk != domain.RiskLow {
			t.Errorf("expected risk 'low', got %q", foundMac.Risk)
		}
		if foundMac.SizeBytes <= 0 {
			t.Errorf("expected size > 0, got %d", foundMac.SizeBytes)
		}
	}

	if foundXDG == nil {
		t.Fatalf("expected XDG CocoaPods cache target %s to be found", xdgCache)
	}
	if foundXDG.Category != "cocoapods-cache" {
		t.Errorf("expected category 'cocoapods-cache', got %q", foundXDG.Category)
	}
	if foundXDG.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundXDG.Risk)
	}
	if foundXDG.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundXDG.SizeBytes)
	}
}

func TestCocoaPodsCleaner_Clean(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target_pods_to_remove")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	c := New([]string{})
	target := domain.Target{
		ID:       "cp-test-id",
		Name:     "target",
		Path:     targetDir,
		Category: "cocoapods-pods",
		Risk:     domain.RiskMedium,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("expected targetDir to be removed, got err: %v", err)
	}
}

func TestCocoaPodsCleaner_Clean_Error(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "locked_parent")
	childDir := filepath.Join(parentDir, "Pods")
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

func TestCocoaPodsCleaner_Scan_SymlinkAndExclusions(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_cocoa_proj")
	podsDir := filepath.Join(proj, "Pods")
	if err := os.MkdirAll(podsDir, 0o755); err != nil {
		t.Fatalf("failed to create podsDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "Podfile"), []byte("target 'my_cocoa_proj'\n"), 0o644); err != nil {
		t.Fatalf("failed to write Podfile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(podsDir, "dummy"), []byte("data"), 0o644); err != nil {
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
		ExcludePaths: []string{"my_cocoa_proj/**"},
	}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == podsDir {
			t.Errorf("podsDir %s should have been excluded by pattern", podsDir)
		}
	}
}

func TestCocoaPodsCleaner_Scan_GitSkipped(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	podsInGit := filepath.Join(gitDir, "Pods")
	if err := os.MkdirAll(podsInGit, 0o755); err != nil {
		t.Fatalf("failed to create podsInGit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "Podfile"), []byte("git podfile\n"), 0o644); err != nil {
		t.Fatalf("failed to write Podfile: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == podsInGit {
			t.Errorf(".git internal Pods dir should have been skipped")
		}
	}
}

func TestCocoaPodsCleaner_Scan_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New([]string{t.TempDir()})
	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
