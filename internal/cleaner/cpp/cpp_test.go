package cpp

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestCppCleaner_Name(t *testing.T) {
	c := New([]string{})
	if c.Name() != "cpp" {
		t.Fatalf("expected name 'cpp', got %q", c.Name())
	}
}

func TestCppCleaner_IsInstalled(t *testing.T) {
	c := New([]string{})
	// Standard dev environment should have gcc/clang/make
	if !c.IsInstalled() {
		t.Log("Note: No C/C++ compiler or tools found in default PATH")
	}

	// Test fallback to conan directory when PATH is empty
	tmpHome := t.TempDir()
	t.Setenv("PATH", "")
	t.Setenv("HOME", tmpHome)
	t.Setenv("CONAN_USER_HOME", "")
	t.Setenv("CONAN_HOME", "")

	cIsolated := New([]string{})
	if cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be false when PATH is empty and no conan dirs exist")
	}

	// Create .conan directory
	conanDir := filepath.Join(tmpHome, ".conan")
	if err := os.MkdirAll(conanDir, 0o755); err != nil {
		t.Fatalf("failed to create .conan dir: %v", err)
	}

	if !cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be true when ~/.conan exists")
	}

	// Remove .conan and create .conan2 directory
	_ = os.RemoveAll(conanDir)
	conan2Dir := filepath.Join(tmpHome, ".conan2")
	if err := os.MkdirAll(conan2Dir, 0o755); err != nil {
		t.Fatalf("failed to create .conan2 dir: %v", err)
	}

	if !cIsolated.IsInstalled() {
		t.Error("expected IsInstalled to be true when ~/.conan2 exists")
	}
}

func TestCppCleaner_Scan_CMakeBuild_WithCache(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "project_with_cache")
	buildDir := filepath.Join(proj, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(buildDir, "CMakeCache.txt"), []byte("# CMake cache"), 0o644); err != nil {
		t.Fatalf("failed to write CMakeCache.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "main.o"), []byte("object code"), 0o644); err != nil {
		t.Fatalf("failed to write main.o: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var found *domain.Target
	for i := range targets {
		if targets[i].Path == buildDir {
			found = &targets[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("expected buildDir %s to be found, but targets were: %+v", buildDir, targets)
	}
	if found.Category != "cmake-build" {
		t.Errorf("expected category 'cmake-build', got %q", found.Category)
	}
	if found.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium', got %q", found.Risk)
	}
	if found.SizeBytes <= 0 {
		t.Errorf("expected size > 0, got %d", found.SizeBytes)
	}
	if found.ID == "" {
		t.Error("expected non-empty target ID")
	}
}

func TestCppCleaner_Scan_CMakeBuild_WithSiblingLists(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "project_with_lists")
	buildDir := filepath.Join(proj, "cmake-build-debug")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}

	// Sibling CMakeLists.txt in project root, but no CMakeCache.txt in buildDir
	if err := os.WriteFile(filepath.Join(proj, "CMakeLists.txt"), []byte("cmake_minimum_required(VERSION 3.10)"), 0o644); err != nil {
		t.Fatalf("failed to write CMakeLists.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "lib.a"), []byte("library bytes"), 0o644); err != nil {
		t.Fatalf("failed to write lib.a: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var found *domain.Target
	for i := range targets {
		if targets[i].Path == buildDir {
			found = &targets[i]
			break
		}
	}

	if found == nil {
		t.Fatalf("expected buildDir %s to be found, but targets were: %+v", buildDir, targets)
	}
	if found.Category != "cmake-build" {
		t.Errorf("expected category 'cmake-build', got %q", found.Category)
	}
	if found.Risk != domain.RiskMedium {
		t.Errorf("expected risk 'medium', got %q", found.Risk)
	}
}

func TestCppCleaner_Scan_IgnoreGenericBuildWithoutCMake(t *testing.T) {
	tmp := t.TempDir()
	genericProj := filepath.Join(tmp, "generic_project")
	buildDir := filepath.Join(genericProj, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}

	// No CMakeLists.txt and no CMakeCache.txt
	if err := os.WriteFile(filepath.Join(genericProj, "Makefile"), []byte("all:\n"), 0o644); err != nil {
		t.Fatalf("failed to write Makefile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "artifact.bin"), []byte("bytes"), 0o644); err != nil {
		t.Fatalf("failed to write artifact: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildDir {
			t.Fatalf("generic build directory %s should NOT have been claimed by cpp cleaner", buildDir)
		}
	}
}

func TestCppCleaner_Scan_VcpkgInstalled(t *testing.T) {
	tmp := t.TempDir()

	// Proj 1: vcpkg_installed with vcpkg.json
	proj1 := filepath.Join(tmp, "proj_vcpkg_json")
	vcpkgDir1 := filepath.Join(proj1, "vcpkg_installed")
	if err := os.MkdirAll(vcpkgDir1, 0o755); err != nil {
		t.Fatalf("failed to create vcpkgDir1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj1, "vcpkg.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("failed to write vcpkg.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vcpkgDir1, "vcpkg.lib"), []byte("lib data"), 0o644); err != nil {
		t.Fatalf("failed to write vcpkg.lib: %v", err)
	}

	// Proj 2: vcpkg_installed with CMakeLists.txt (no vcpkg.json)
	proj2 := filepath.Join(tmp, "proj_cmakelists")
	vcpkgDir2 := filepath.Join(proj2, "vcpkg_installed")
	if err := os.MkdirAll(vcpkgDir2, 0o755); err != nil {
		t.Fatalf("failed to create vcpkgDir2: %v", err)
	}
	if err := os.WriteFile(filepath.Join(proj2, "CMakeLists.txt"), []byte("project(foo)"), 0o644); err != nil {
		t.Fatalf("failed to write CMakeLists.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vcpkgDir2, "vcpkg2.lib"), []byte("lib data 2"), 0o644); err != nil {
		t.Fatalf("failed to write vcpkg2.lib: %v", err)
	}

	// Proj 3: vcpkg_installed without any manifest (orphan/unrelated)
	proj3 := filepath.Join(tmp, "proj_unrelated")
	vcpkgDir3 := filepath.Join(proj3, "vcpkg_installed")
	if err := os.MkdirAll(vcpkgDir3, 0o755); err != nil {
		t.Fatalf("failed to create vcpkgDir3: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vcpkgDir3, "orphan.lib"), []byte("orphan"), 0o644); err != nil {
		t.Fatalf("failed to write orphan: %v", err)
	}

	c := New([]string{tmp})
	rules := domain.EcosystemRules{Enabled: true, MaxAgeDays: 30}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var found1, found2, found3 bool
	for _, target := range targets {
		if target.Path == vcpkgDir1 {
			found1 = true
			if target.Category != "vcpkg-installed" {
				t.Errorf("proj1: expected category 'vcpkg-installed', got %q", target.Category)
			}
			if target.Risk != domain.RiskMedium {
				t.Errorf("proj1: expected risk 'medium', got %q", target.Risk)
			}
		}
		if target.Path == vcpkgDir2 {
			found2 = true
			if target.Category != "vcpkg-installed" {
				t.Errorf("proj2: expected category 'vcpkg-installed', got %q", target.Category)
			}
			if target.Risk != domain.RiskMedium {
				t.Errorf("proj2: expected risk 'medium', got %q", target.Risk)
			}
		}
		if target.Path == vcpkgDir3 {
			found3 = true
		}
	}

	if !found1 {
		t.Errorf("expected vcpkgDir1 %s to be found", vcpkgDir1)
	}
	if !found2 {
		t.Errorf("expected vcpkgDir2 %s to be found", vcpkgDir2)
	}
	if found3 {
		t.Errorf("orphan vcpkgDir3 %s should NOT have been claimed", vcpkgDir3)
	}
}

func TestCppCleaner_Scan_GlobalCaches(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Conan 1 cache: ~/.conan/data
	conan1Data := filepath.Join(tmpHome, ".conan", "data")
	if err := os.MkdirAll(conan1Data, 0o755); err != nil {
		t.Fatalf("failed to create conan1Data: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conan1Data, "pkg.tgz"), []byte("conan1 pkg"), 0o644); err != nil {
		t.Fatalf("failed to write conan1 pkg: %v", err)
	}

	// Conan 2 cache: ~/.conan2/p
	conan2P := filepath.Join(tmpHome, ".conan2", "p")
	if err := os.MkdirAll(conan2P, 0o755); err != nil {
		t.Fatalf("failed to create conan2P: %v", err)
	}
	if err := os.WriteFile(filepath.Join(conan2P, "pkg2.tgz"), []byte("conan2 pkg"), 0o644); err != nil {
		t.Fatalf("failed to write conan2 pkg: %v", err)
	}

	// vcpkg cache: test both mac and linux paths
	var vcpkgCache string
	if runtime.GOOS == "darwin" {
		vcpkgCache = filepath.Join(tmpHome, "Library", "Caches", "vcpkg")
	} else {
		vcpkgCache = filepath.Join(tmpHome, ".cache", "vcpkg")
	}
	if err := os.MkdirAll(vcpkgCache, 0o755); err != nil {
		t.Fatalf("failed to create vcpkgCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vcpkgCache, "binary.zip"), []byte("vcpkg zip"), 0o644); err != nil {
		t.Fatalf("failed to write vcpkg zip: %v", err)
	}

	c := New([]string{}) // No scanPaths needed for global caches
	rules := domain.EcosystemRules{Enabled: true}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var foundConan1, foundConan2, foundVcpkg bool
	for _, target := range targets {
		switch target.Category {
		case "conan-cache":
			foundConan1 = true
			if target.Risk != domain.RiskLow {
				t.Errorf("conan-cache: expected RiskLow, got %v", target.Risk)
			}
			if target.SizeBytes <= 0 {
				t.Errorf("conan-cache: expected size > 0, got %d", target.SizeBytes)
			}
		case "conan2-cache":
			foundConan2 = true
			if target.Risk != domain.RiskLow {
				t.Errorf("conan2-cache: expected RiskLow, got %v", target.Risk)
			}
			if target.SizeBytes <= 0 {
				t.Errorf("conan2-cache: expected size > 0, got %d", target.SizeBytes)
			}
		case "vcpkg-cache":
			foundVcpkg = true
			if target.Risk != domain.RiskLow {
				t.Errorf("vcpkg-cache: expected RiskLow, got %v", target.Risk)
			}
			if target.SizeBytes <= 0 {
				t.Errorf("vcpkg-cache: expected size > 0, got %d", target.SizeBytes)
			}
		}
	}

	if !foundConan1 {
		t.Errorf("expected conan-cache to be found in targets: %+v", targets)
	}
	if !foundConan2 {
		t.Errorf("expected conan2-cache to be found in targets: %+v", targets)
	}
	if !foundVcpkg {
		t.Errorf("expected vcpkg-cache to be found in targets: %+v", targets)
	}
}

func TestCppCleaner_Clean_ReadOnlyChmod(t *testing.T) {
	tmp := t.TempDir()
	readOnlyDir := filepath.Join(tmp, "conan_readonly_pkg")
	subDir := filepath.Join(readOnlyDir, "sub")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	f1 := filepath.Join(readOnlyDir, "file1.a")
	f2 := filepath.Join(subDir, "file2.a")
	for _, f := range []string{f1, f2} {
		if err := os.WriteFile(f, []byte("package data"), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", f, err)
		}
	}

	// Cleanup hook to avoid locking TempDir if test fails
	t.Cleanup(func() {
		_ = filepath.Walk(readOnlyDir, func(p string, info os.FileInfo, err error) error {
			if err == nil {
				_ = os.Chmod(p, 0o777)
			}
			return nil
		})
	})

	// Make files 0444 and dirs 0555
	for _, f := range []string{f1, f2} {
		_ = os.Chmod(f, 0o444)
	}
	_ = os.Chmod(subDir, 0o555)
	_ = os.Chmod(readOnlyDir, 0o555)

	target := domain.Target{
		ID:       "test-ro",
		Name:     "read-only-conan",
		Path:     readOnlyDir,
		Category: "conan-cache",
		Risk:     domain.RiskLow,
	}

	c := New([]string{})
	if err := c.Clean(context.Background(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed on read-only directory: %v", err)
	}

	if _, err := os.Stat(readOnlyDir); !os.IsNotExist(err) {
		t.Errorf("expected readOnlyDir to be removed, but stat returned: %v", err)
	}
}

func TestCppCleaner_Scan_SymlinkAndExclusions(t *testing.T) {
	tmp := t.TempDir()
	proj := filepath.Join(tmp, "my_project")
	buildDir := filepath.Join(proj, "build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatalf("failed to create buildDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildDir, "CMakeCache.txt"), []byte("cache"), 0o644); err != nil {
		t.Fatalf("failed to write cache: %v", err)
	}

	// Symlink pointing to build
	linkDir := filepath.Join(tmp, "link_to_proj")
	if err := os.Symlink(proj, linkDir); err != nil {
		t.Skip("symlinks not supported")
	}

	c := New([]string{tmp})
	// Test exclusion rule
	rules := domain.EcosystemRules{
		Enabled:      true,
		MaxAgeDays:   30,
		ExcludePaths: []string{"my_project/**"},
	}
	targets, err := c.Scan(context.Background(), rules)
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	for _, target := range targets {
		if target.Path == buildDir {
			t.Errorf("buildDir %s should have been excluded by rule", buildDir)
		}
	}
}

func TestCppCleaner_Scan_GitSkipped(t *testing.T) {
	tmp := t.TempDir()
	gitDir := filepath.Join(tmp, ".git")
	buildInGit := filepath.Join(gitDir, "build")
	if err := os.MkdirAll(buildInGit, 0o755); err != nil {
		t.Fatalf("failed to create buildInGit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(buildInGit, "CMakeCache.txt"), []byte("cache"), 0o644); err != nil {
		t.Fatalf("failed to write CMakeCache: %v", err)
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

func TestCppCleaner_Scan_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := New([]string{t.TempDir()})
	_, err := c.Scan(ctx, domain.EcosystemRules{Enabled: true})
	if err == nil {
		t.Error("expected error for canceled context, got nil")
	}
}

func TestCppCleaner_Clean_Error(t *testing.T) {
	tmp := t.TempDir()
	parentDir := filepath.Join(tmp, "locked_parent")
	childDir := filepath.Join(parentDir, "target")
	if err := os.MkdirAll(childDir, 0o755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	t.Cleanup(func() {
		_ = os.Chmod(parentDir, 0o777)
	})

	// Make parent dir read-only so removing childDir fails
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

