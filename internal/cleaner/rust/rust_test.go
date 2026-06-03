package rust

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestScan_LocalTargets(t *testing.T) {
	tmp := t.TempDir()

	// project1: has Cargo.toml + target/ → should be found
	p1 := filepath.Join(tmp, "project1")
	os.MkdirAll(filepath.Join(p1, "target"), 0o755)
	os.WriteFile(filepath.Join(p1, "Cargo.toml"), []byte("[package]"), 0o644)
	os.WriteFile(filepath.Join(p1, "target", "dummy"), []byte("data"), 0o644)

	// project2: has target/ but no Cargo.toml → should NOT be found
	p2 := filepath.Join(tmp, "project2")
	os.MkdirAll(filepath.Join(p2, "target"), 0o755)
	os.WriteFile(filepath.Join(p2, "target", "dummy"), []byte("data"), 0o644)

	c := New([]string{tmp}).(*rustCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, tgt := range targets {
		if tgt.Path == filepath.Join(p1, "target") {
			found = true
			if tgt.Category != "cargo-target" {
				t.Errorf("expected category cargo-target, got %s", tgt.Category)
			}
		}
		if tgt.Path == filepath.Join(p2, "target") {
			t.Error("project2/target should NOT be found (no Cargo.toml)")
		}
	}
	if !found {
		t.Error("project1/target should be found")
	}
}

func TestScan_Disambiguation(t *testing.T) {
	tmp := t.TempDir()

	// both: has Cargo.toml + pom.xml + target/ → Rust cleaner SHOULD claim it
	both := filepath.Join(tmp, "both")
	os.MkdirAll(filepath.Join(both, "target"), 0o755)
	os.WriteFile(filepath.Join(both, "Cargo.toml"), []byte("[package]"), 0o644)
	os.WriteFile(filepath.Join(both, "pom.xml"), []byte("<project/>"), 0o644)
	os.WriteFile(filepath.Join(both, "target", "dummy"), []byte("data"), 0o644)

	// java-only: has pom.xml + target/ but no Cargo.toml → Rust cleaner should NOT claim it
	javaOnly := filepath.Join(tmp, "java-only")
	os.MkdirAll(filepath.Join(javaOnly, "target"), 0o755)
	os.WriteFile(filepath.Join(javaOnly, "pom.xml"), []byte("<project/>"), 0o644)
	os.WriteFile(filepath.Join(javaOnly, "target", "dummy"), []byte("data"), 0o644)

	c := New([]string{tmp}).(*rustCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundBoth := false
	for _, tgt := range targets {
		if tgt.Path == filepath.Join(both, "target") {
			foundBoth = true
		}
		if tgt.Path == filepath.Join(javaOnly, "target") {
			t.Error("java-only/target should NOT be claimed by rust cleaner")
		}
	}
	if !foundBoth {
		t.Error("both/target should be claimed by rust cleaner (has Cargo.toml)")
	}
}

func TestScan_GlobalCaches(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CARGO_HOME", tmp)

	// Create registry and git subdirs
	os.MkdirAll(filepath.Join(tmp, "registry", "cache"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "git", "db"), 0o755)
	os.WriteFile(filepath.Join(tmp, "registry", "cache", "crate"), []byte("data"), 0o644)
	os.WriteFile(filepath.Join(tmp, "git", "db", "repo"), []byte("data"), 0o644)

	c := New([]string{}).(*rustCleaner)
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

	if !categories["cargo-registry"] {
		t.Error("expected cargo-registry target")
	}
	if !categories["cargo-git"] {
		t.Error("expected cargo-git target")
	}
}

func TestScan_EmptyScanPaths(t *testing.T) {
	t.Setenv("CARGO_HOME", "/nonexistent-cargo-home-for-test")

	c := New([]string{}).(*rustCleaner)
	targets, err := c.Scan(context.Background(), domain.EcosystemRules{
		Enabled:    true,
		MaxAgeDays: 30,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected no targets for empty scan paths + nonexistent cargo home, got %d", len(targets))
	}
}

func TestIsInstalled_CargoHomeExists(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "bin"), 0o755)
	t.Setenv("CARGO_HOME", tmp)

	c := New([]string{}).(*rustCleaner)
	if !c.IsInstalled() {
		t.Error("expected IsInstalled=true when CARGO_HOME/bin exists")
	}
}
