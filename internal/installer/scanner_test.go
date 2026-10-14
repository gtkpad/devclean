package installer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/installer"
	"github.com/gtkpad/devclean/internal/safety"
)

func TestScanner_MaxDepthLimiter(t *testing.T) {
	tmp := t.TempDir()

	// Level 1: tmp/setup1.dmg (depth 1 relative to tmp)
	f1 := filepath.Join(tmp, "setup1.dmg")
	_ = os.WriteFile(f1, []byte("dmg1"), 0o644)

	// Level 2: tmp/sub1/setup2.dmg (depth 2)
	sub1 := filepath.Join(tmp, "sub1")
	_ = os.Mkdir(sub1, 0o755)
	f2 := filepath.Join(sub1, "setup2.dmg")
	_ = os.WriteFile(f2, []byte("dmg2"), 0o644)

	// Level 3: tmp/sub1/sub2/setup3.dmg (depth 3 - should be skipped when MaxDepth = 2)
	sub2 := filepath.Join(sub1, "sub2")
	_ = os.Mkdir(sub2, 0o755)
	f3 := filepath.Join(sub2, "setup3.dmg")
	_ = os.WriteFile(f3, []byte("dmg3"), 0o644)

	// Level 4: tmp/sub1/sub2/sub3/setup4.dmg (depth 4)
	sub3 := filepath.Join(sub2, "sub3")
	_ = os.Mkdir(sub3, 0o755)
	f4 := filepath.Join(sub3, "setup4.dmg")
	_ = os.WriteFile(f4, []byte("dmg4"), 0o644)

	// Make all files older than 24h
	oldTime := time.Now().Add(-48 * time.Hour)
	for _, p := range []string{f1, f2, f3, f4} {
		_ = os.Chtimes(p, oldTime, oldTime)
	}

	opts := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   24 * time.Hour,
		MinSize:  0,
	}

	s := installer.NewScanner(opts, nil)
	candidates, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	found := make(map[string]bool)
	for _, c := range candidates {
		found[filepath.Base(c.Path)] = true
	}

	if !found["setup1.dmg"] {
		t.Errorf("expected setup1.dmg (level 1) to be found")
	}
	if !found["setup2.dmg"] {
		t.Errorf("expected setup2.dmg (level 2) to be found")
	}
	if found["setup3.dmg"] {
		t.Errorf("setup3.dmg (level 3) should NOT be found with MaxDepth=2")
	}
	if found["setup4.dmg"] {
		t.Errorf("setup4.dmg (level 4) should NOT be found with MaxDepth=2")
	}
}

func TestScanner_SymlinkSkipped(t *testing.T) {
	tmp := t.TempDir()

	// External dir outside scan path
	externalDir := t.TempDir()
	extFile := filepath.Join(externalDir, "external.dmg")
	_ = os.WriteFile(extFile, []byte("ext-dmg"), 0o644)
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(extFile, oldTime, oldTime)

	// Symlink to external file
	symlinkFile := filepath.Join(tmp, "link.dmg")
	if err := os.Symlink(extFile, symlinkFile); err != nil {
		t.Skip("symlinks not supported on this environment")
	}

	// Symlink to external dir
	symlinkDir := filepath.Join(tmp, "linkdir")
	_ = os.Symlink(externalDir, symlinkDir)

	opts := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   0,
		MinSize:  0,
	}

	s := installer.NewScanner(opts, nil)
	candidates, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	for _, c := range candidates {
		if c.Path == symlinkFile || filepath.Base(c.Path) == "external.dmg" {
			t.Errorf("symlink target %s should be skipped", c.Path)
		}
	}
}

func TestScanner_BlacklistProtected(t *testing.T) {
	tmp := t.TempDir()

	// Simulated git repo inside downloads: tmp/.git/something.exe
	gitDir := filepath.Join(tmp, ".git")
	_ = os.Mkdir(gitDir, 0o755)
	gitExe := filepath.Join(gitDir, "hook.exe")
	_ = os.WriteFile(gitExe, []byte("hook"), 0o644)
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(gitExe, oldTime, oldTime)

	bl := safety.NewBlacklist(nil)
	opts := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 3,
		MinAge:   0,
		MinSize:  0,
	}

	s := installer.NewScanner(opts, bl)
	candidates, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	for _, c := range candidates {
		if filepath.Base(c.Path) == "hook.exe" {
			t.Errorf("blacklisted .git entry hook.exe was detected")
		}
	}
}

func TestScanner_MacAppDirectory(t *testing.T) {
	tmp := t.TempDir()

	appDir := filepath.Join(tmp, "SuperApp.app")
	contentsDir := filepath.Join(appDir, "Contents", "MacOS")
	_ = os.MkdirAll(contentsDir, 0o755)

	binFile := filepath.Join(contentsDir, "app_bin")
	_ = os.WriteFile(binFile, make([]byte, 5000), 0o755)

	resFile := filepath.Join(appDir, "Contents", "Resources.car")
	_ = os.WriteFile(resFile, make([]byte, 10000), 0o644)

	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(appDir, oldTime, oldTime)

	opts := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   24 * time.Hour,
		MinSize:  1000,
	}

	s := installer.NewScanner(opts, nil)
	candidates, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(candidates) != 1 {
		t.Fatalf("expected 1 candidate for .app directory, got %d", len(candidates))
	}

	cand := candidates[0]
	if cand.Path != appDir {
		t.Errorf("got path %s, want %s", cand.Path, appDir)
	}
	if cand.Type != installer.TypeDirectApp {
		t.Errorf("got type %s, want %s", cand.Type, installer.TypeDirectApp)
	}
	// Size should be sum of 5000 + 10000 = 15000
	if cand.SizeBytes != 15000 {
		t.Errorf("got size %d, want 15000", cand.SizeBytes)
	}
}

func TestScanner_MinAgeFilter(t *testing.T) {
	tmp := t.TempDir()

	recentFile := filepath.Join(tmp, "recent.dmg")
	_ = os.WriteFile(recentFile, []byte("recent-dmg"), 0o644)
	// Modified 1 hour ago
	_ = os.Chtimes(recentFile, time.Now().Add(-1*time.Hour), time.Now().Add(-1*time.Hour))

	oldFile := filepath.Join(tmp, "old.dmg")
	_ = os.WriteFile(oldFile, []byte("old-dmg"), 0o644)
	// Modified 48 hours ago
	_ = os.Chtimes(oldFile, time.Now().Add(-48*time.Hour), time.Now().Add(-48*time.Hour))

	// 1. Default min-age filter (24h)
	optsDefault := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   24 * time.Hour,
	}
	sDefault := installer.NewScanner(optsDefault, nil)
	candsDefault, err := sDefault.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(candsDefault) != 1 || filepath.Base(candsDefault[0].Path) != "old.dmg" {
		t.Errorf("expected only old.dmg with default 24h filter, got %v", candsDefault)
	}

	// 2. Explicit --min-age 0 includes both
	optsZero := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   0,
	}
	sZero := installer.NewScanner(optsZero, nil)
	candsZero, err := sZero.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(candsZero) != 2 {
		t.Errorf("expected both files with --min-age 0, got %d", len(candsZero))
	}
}

func TestScanner_MinSizeFilter(t *testing.T) {
	tmp := t.TempDir()

	smallFile := filepath.Join(tmp, "small.pkg")
	_ = os.WriteFile(smallFile, make([]byte, 50), 0o644)

	bigFile := filepath.Join(tmp, "big.pkg")
	_ = os.WriteFile(bigFile, make([]byte, 5000), 0o644)

	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(smallFile, oldTime, oldTime)
	_ = os.Chtimes(bigFile, oldTime, oldTime)

	opts := installer.Options{
		Paths:    []string{tmp},
		MaxDepth: 2,
		MinAge:   0,
		MinSize:  1000, // 1000 bytes minimum
	}

	s := installer.NewScanner(opts, nil)
	candidates, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(candidates) != 1 || filepath.Base(candidates[0].Path) != "big.pkg" {
		t.Errorf("expected only big.pkg with min-size 1000, got %v", candidates)
	}
}

func TestScanner_ContextCanceled(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "setup.dmg")
	_ = os.WriteFile(f, []byte("data"), 0o644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	opts := installer.Options{Paths: []string{tmp}}
	s := installer.NewScanner(opts, nil)
	_, err := s.Scan(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestScanner_DotfilesIgnored(t *testing.T) {
	tmp := t.TempDir()
	hiddenFile := filepath.Join(tmp, ".hidden.dmg")
	_ = os.WriteFile(hiddenFile, []byte("hidden"), 0o644)
	oldTime := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(hiddenFile, oldTime, oldTime)

	opts := installer.Options{
		Paths:   []string{tmp},
		MinAge:  0,
		MinSize: 0,
	}
	s := installer.NewScanner(opts, nil)
	cands, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	for _, c := range cands {
		if filepath.Base(c.Path) == ".hidden.dmg" {
			t.Errorf("dotfile .hidden.dmg should be ignored")
		}
	}
}

func TestScanner_NonexistentPathSkipped(t *testing.T) {
	opts := installer.Options{
		Paths: []string{"/path/that/definitely/does/not/exist/12345"},
	}
	s := installer.NewScanner(opts, nil)
	cands, err := s.Scan(context.Background())
	if err != nil {
		t.Fatalf("expected nil error for nonexistent path, got %v", err)
	}
	if len(cands) != 0 {
		t.Errorf("expected 0 candidates, got %d", len(cands))
	}
}
