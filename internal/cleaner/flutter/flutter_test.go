package flutter

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestFlutterCleaner_Name(t *testing.T) {
	c := New([]string{})
	if c.Name() != "flutter" {
		t.Fatalf("expected name 'flutter', got %q", c.Name())
	}
}

func TestFlutterCleaner_IsInstalled(t *testing.T) {
	// Isolated environment without flutter/dart tools or pub cache
	tmpHome := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("PUB_CACHE", "")

	cIsolated := New([]string{})
	if cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be false when PATH is empty and no pub-cache exists")
	}

	// Create ~/.pub-cache directory
	pubCacheDir := filepath.Join(tmpHome, ".pub-cache")
	if err := os.MkdirAll(pubCacheDir, 0o755); err != nil {
		t.Fatalf("failed to create .pub-cache dir: %v", err)
	}

	if !cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be true when ~/.pub-cache exists")
	}

	// Test PUB_CACHE environment variable override
	customCache := filepath.Join(tmpHome, "custom-pub-cache")
	if err := os.MkdirAll(customCache, 0o755); err != nil {
		t.Fatalf("failed to create custom-pub-cache dir: %v", err)
	}
	_ = os.RemoveAll(pubCacheDir)
	t.Setenv("PUB_CACHE", customCache)

	if !cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be true when $PUB_CACHE exists")
	}
}

func TestFlutterCleaner_Scan_WithPubspec(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "flutter_app")
	dartToolDir := filepath.Join(proj, ".dart_tool")
	buildDir := filepath.Join(proj, "build")

	if err := os.MkdirAll(dartToolDir, 0o755); err != nil {
		t.Fatalf("failed to create .dart_tool: %v", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create build dir: %v", err)
	}

	// Create pubspec.yaml and pubspec.lock in parent dir
	pubspec := filepath.Join(proj, "pubspec.yaml")
	if err := os.WriteFile(pubspec, []byte("name: flutter_app\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec.yaml: %v", err)
	}
	pubspecLock := filepath.Join(proj, "pubspec.lock")
	if err := os.WriteFile(pubspecLock, []byte("# lockfile\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec.lock: %v", err)
	}

	// Create dummy files inside .dart_tool and build
	if err := os.WriteFile(filepath.Join(dartToolDir, "package_config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write dart_tool file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "app.apk"), []byte("binary data"), 0o644); err != nil {
		t.Fatalf("failed to write build file: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundDartTool, foundBuild *domain.Target
	for i := range targets {
		if targets[i].Path == dartToolDir {
			foundDartTool = &targets[i]
		}
		if targets[i].Path == buildDir {
			foundBuild = &targets[i]
		}
	}

	if foundDartTool == nil {
		t.Fatalf("expected .dart_tool target %s to be found", dartToolDir)
	}
	if foundDartTool.Category != "flutter-dart-tool" {
		t.Errorf("expected category 'flutter-dart-tool', got %q", foundDartTool.Category)
	}
	if foundDartTool.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low' for .dart_tool, got %q", foundDartTool.Risk)
	}
	if foundDartTool.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundDartTool.SizeBytes)
	}
	if foundDartTool.ID == "" {
		t.Error("expected non-empty ID for .dart_tool target")
	}

	if foundBuild == nil {
		t.Fatalf("expected build target %s to be found", buildDir)
	}
	if foundBuild.Category != "flutter-build" {
		t.Errorf("expected category 'flutter-build', got %q", foundBuild.Category)
	}
	if foundBuild.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium' for build, got %q", foundBuild.Risk)
	}
	if foundBuild.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundBuild.SizeBytes)
	}
	if foundBuild.ID == "" {
		t.Error("expected non-empty ID for build target")
	}
}

func TestFlutterCleaner_Scan_IgnoreWithoutPubspec(t *testing.T) {
	tmp := t.TempDir()
	genericProj := filepath.Join(tmp, "generic_project")
	dartToolDir := filepath.Join(genericProj, ".dart_tool")
	buildDir := filepath.Join(genericProj, "build")

	if err := os.MkdirAll(dartToolDir, 0o755); err != nil {
		t.Fatalf("failed to create .dart_tool: %v", err)
	}
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create build: %v", err)
	}

	// No pubspec.yaml - only a generic Makefile
	if err := os.WriteFile(filepath.Join(genericProj, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dartToolDir, "dummy"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write dummy: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "output.bin"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write output.bin: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == dartToolDir {
			t.Fatalf(".dart_tool directory %s should NOT have been claimed without pubspec.yaml", dartToolDir)
		}
		if target.Path == buildDir {
			t.Fatalf("build directory %s should NOT have been claimed without pubspec.yaml", buildDir)
		}
	}
}

func TestFlutterCleaner_Scan_GlobalPubCache(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Test with $PUB_CACHE custom path
	customPubCache := filepath.Join(tmpHome, "custom_pub_cache")
	if err := os.MkdirAll(customPubCache, 0o755); err != nil {
		t.Fatalf("failed to create custom_pub_cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customPubCache, "cached_package.tar"), []byte("cache data"), 0o644); err != nil {
		t.Fatalf("failed to write cache file: %v", err)
	}
	t.Setenv("PUB_CACHE", customPubCache)

	c := New([]string{}) // No scanPaths needed for global cache
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundGlobal *domain.Target
	for i := range targets {
		if targets[i].Path == customPubCache {
			foundGlobal = &targets[i]
			break
		}
	}

	if foundGlobal == nil {
		t.Fatalf("expected global pub cache target %s to be found", customPubCache)
	}
	if foundGlobal.Category != "pub-cache" {
		t.Errorf("expected category 'pub-cache', got %q", foundGlobal.Category)
	}
	if foundGlobal.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundGlobal.Risk)
	}
	if foundGlobal.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", foundGlobal.SizeBytes)
	}

	// Test default ~/.pub-cache fallback when $PUB_CACHE is empty
	t.Setenv("PUB_CACHE", "")
	defaultPubCache := filepath.Join(tmpHome, ".pub-cache")
	if err := os.MkdirAll(defaultPubCache, 0o755); err != nil {
		t.Fatalf("failed to create .pub-cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(defaultPubCache, "hosted.tar"), []byte("hosted"), 0o644); err != nil {
		t.Fatalf("failed to write hosted.tar: %v", err)
	}

	targets, err = c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundDefault *domain.Target
	for i := range targets {
		if targets[i].Path == defaultPubCache {
			foundDefault = &targets[i]
			break
		}
	}

	if foundDefault == nil {
		t.Fatalf("expected default pub cache target %s to be found", defaultPubCache)
	}
	if foundDefault.Category != "pub-cache" {
		t.Errorf("expected category 'pub-cache', got %q", foundDefault.Category)
	}
	if foundDefault.Risk != domain.RiskLow {
		t.Errorf("expected risk 'low', got %q", foundDefault.Risk)
	}
}

func TestFlutterCleaner_Clean(t *testing.T) {
	tmp := t.TempDir()
	targetDir := filepath.Join(tmp, "target_to_remove")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	c := New([]string{})
	target := domain.Target{
		ID:       "test-id",
		Name:     "target",
		Path:     targetDir,
		Category: "flutter-build",
		Risk:     domain.RiskMedium,
	}

	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(targetDir); !os.IsNotExist(err) {
		t.Errorf("expected targetDir to be removed, got err: %v", err)
	}
}

func TestFlutterCleaner_Clean_Error(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "locked_parent")
	childDir := filepath.Join(parentDir, "target")
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

func TestFlutterCleaner_Scan_SymlinkAndExclusions(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_flutter_proj")
	buildDir := filepath.Join(proj, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj, "pubspec.yaml"), []byte("name: my_flutter_proj\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "output"), []byte("data"), 0o644); err != nil {
		t.Fatalf("failed to write build file: %v", err)
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
		ExcludePaths: []string{"my_flutter_proj/**"},
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

func TestFlutterCleaner_Scan_GitSkipped(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	buildInGit := filepath.Join(gitDir, "build")
	if err := os.MkdirAll(buildInGit, 0o755); err != nil {
		t.Fatalf("failed to create buildInGit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "pubspec.yaml"), []byte("name: git\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildInGit {
			t.Errorf(".git internal build dir should have been skipped")
		}
	}
}

func TestFlutterCleaner_Scan_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New([]string{t.TempDir()})
	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}
