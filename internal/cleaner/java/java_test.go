package java

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestScan_GradleBuild(t *testing.T) {
	tmp := t.TempDir()

	// project1: has build.gradle + build/ → should be found
	p1 := filepath.Join(tmp, "project1")
	os.MkdirAll(filepath.Join(p1, "build"), 0o755)
	os.WriteFile(filepath.Join(p1, "build.gradle"), []byte("apply plugin: 'java'"), 0o644)
	os.WriteFile(filepath.Join(p1, "build", "output.jar"), []byte("data"), 0o644)

	// project2: has build/ but no gradle file → should NOT be found
	p2 := filepath.Join(tmp, "project2")
	os.MkdirAll(filepath.Join(p2, "build"), 0o755)
	os.WriteFile(filepath.Join(p2, "build", "output.jar"), []byte("data"), 0o644)

	c := New([]string{tmp}).(*javaCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Path == filepath.Join(p1, "build") {
			found = true
			if tgt.Category != "gradle-build" {
				t.Errorf("expected category gradle-build, got %s", tgt.Category)
			}
		}
		if tgt.Path == filepath.Join(p2, "build") {
			t.Error("project2/build should NOT be found (no build.gradle)")
		}
	}
	if !found {
		t.Error("project1/build should be found")
	}
}

func TestScan_GradleKts(t *testing.T) {
	tmp := t.TempDir()

	p := filepath.Join(tmp, "kts-project")
	os.MkdirAll(filepath.Join(p, "build"), 0o755)
	os.WriteFile(filepath.Join(p, "build.gradle.kts"), []byte("plugins { java }"), 0o644)
	os.WriteFile(filepath.Join(p, "build", "output.jar"), []byte("data"), 0o644)

	c := New([]string{tmp}).(*javaCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Path == filepath.Join(p, "build") {
			found = true
			if tgt.Category != "gradle-build" {
				t.Errorf("expected category gradle-build, got %s", tgt.Category)
			}
		}
	}
	if !found {
		t.Error("kts-project/build should be found (has build.gradle.kts)")
	}
}

func TestScan_MavenTarget(t *testing.T) {
	tmp := t.TempDir()

	// maven-proj: pom.xml + target/ → should be found
	maven := filepath.Join(tmp, "maven-proj")
	os.MkdirAll(filepath.Join(maven, "target"), 0o755)
	os.WriteFile(filepath.Join(maven, "pom.xml"), []byte("<project/>"), 0o644)
	os.WriteFile(filepath.Join(maven, "target", "app.jar"), []byte("data"), 0o644)

	// rust-proj: Cargo.toml + target/ → should NOT be found
	rust := filepath.Join(tmp, "rust-proj")
	os.MkdirAll(filepath.Join(rust, "target"), 0o755)
	os.WriteFile(filepath.Join(rust, "Cargo.toml"), []byte("[package]"), 0o644)
	os.WriteFile(filepath.Join(rust, "target", "binary"), []byte("data"), 0o644)

	// both: pom.xml + Cargo.toml + target/ → should NOT be found (Cargo.toml present = Rust owns)
	both := filepath.Join(tmp, "both")
	os.MkdirAll(filepath.Join(both, "target"), 0o755)
	os.WriteFile(filepath.Join(both, "pom.xml"), []byte("<project/>"), 0o644)
	os.WriteFile(filepath.Join(both, "Cargo.toml"), []byte("[package]"), 0o644)
	os.WriteFile(filepath.Join(both, "target", "app.jar"), []byte("data"), 0o644)

	c := New([]string{tmp}).(*javaCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundMaven := false
	for _, tgt := range targets {
		if tgt.Path == filepath.Join(maven, "target") {
			foundMaven = true
			if tgt.Category != "maven-target" {
				t.Errorf("expected category maven-target, got %s", tgt.Category)
			}
		}
		if tgt.Path == filepath.Join(rust, "target") {
			t.Error("rust-proj/target should NOT be claimed by java cleaner")
		}
		if tgt.Path == filepath.Join(both, "target") {
			t.Error("both/target should NOT be claimed by java cleaner (Cargo.toml present)")
		}
	}
	if !foundMaven {
		t.Error("maven-proj/target should be found")
	}
}

func TestScan_GlobalCaches(t *testing.T) {
	tmp := t.TempDir()
	gradleDir := filepath.Join(tmp, "gradle")
	mavenDir := filepath.Join(tmp, "maven")

	t.Setenv("GRADLE_USER_HOME", gradleDir)
	t.Setenv("HOME", tmp) // mavenHome uses UserHomeDir

	os.MkdirAll(filepath.Join(gradleDir, "caches", "modules-2"), 0o755)
	os.MkdirAll(filepath.Join(gradleDir, "wrapper", "dists"), 0o755)
	os.MkdirAll(filepath.Join(mavenDir, "repository"), 0o755)
	os.WriteFile(filepath.Join(gradleDir, "caches", "modules-2", "cache.bin"), []byte("data"), 0o644)
	os.WriteFile(filepath.Join(gradleDir, "wrapper", "dists", "gradle.zip"), []byte("data"), 0o644)

	// For maven, we need ~/.m2/repository
	os.MkdirAll(filepath.Join(tmp, ".m2", "repository"), 0o755)
	os.WriteFile(filepath.Join(tmp, ".m2", "repository", "artifact.jar"), []byte("data"), 0o644)

	c := New([]string{}).(*javaCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	categories := make(map[string]bool)
	for _, tgt := range targets {
		categories[tgt.Category] = true
	}

	if !categories["gradle-cache"] {
		t.Error("expected gradle-cache target")
	}
	if !categories["gradle-wrapper"] {
		t.Error("expected gradle-wrapper target")
	}
}

func TestScan_EmptyScanPaths(t *testing.T) {
	t.Setenv("GRADLE_USER_HOME", "/nonexistent-gradle-home")
	t.Setenv("HOME", "/nonexistent-home-for-test")

	c := New([]string{}).(*javaCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected no targets, got %d", len(targets))
	}
}
