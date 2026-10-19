package browsers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestBrowsers_Constructors(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()

	c1 := NewWithExecutor(mockExec)
	if c1.Name() != "browsers" {
		t.Errorf("NewWithExecutor().Name() = %q, want 'browsers'", c1.Name())
	}

	tempHome := t.TempDir()
	c2 := NewWithHome(tempHome)
	if c2.Name() != "browsers" {
		t.Errorf("NewWithHome().Name() = %q, want 'browsers'", c2.Name())
	}

	c3 := NewWithExecutorAndHome(mockExec, tempHome)
	if c3.Name() != "browsers" {
		t.Errorf("NewWithExecutorAndHome().Name() = %q, want 'browsers'", c3.Name())
	}

	c4 := NewWithExecutorHomeAndOS(mockExec, tempHome, "darwin")
	if c4.Name() != "browsers" {
		t.Errorf("NewWithExecutorHomeAndOS().Name() = %q, want 'browsers'", c4.Name())
	}
	if c4.IsInstalled() {
		t.Errorf("IsInstalled() on empty dir = true, want false")
	}

	c5 := NewWithExecutorHomeAndOS(mockExec, "", "darwin")
	if c5.IsInstalled() {
		t.Errorf("IsInstalled() with empty home = true, want false")
	}
}

func TestBrowsers_IsSyscallPermission(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "nil error", err: nil, expected: false},
		{name: "generic error", err: errors.New("something went wrong"), expected: false},
		{name: "syscall EPERM", err: syscall.EPERM, expected: true},
		{name: "syscall EACCES", err: syscall.EACCES, expected: true},
		{name: "syscall ENOENT", err: syscall.ENOENT, expected: false},
		{name: "wrapped PathError with EPERM", err: &os.PathError{Op: "open", Path: "/protected", Err: syscall.EPERM}, expected: true},
		{name: "wrapped PathError with EACCES", err: &os.PathError{Op: "open", Path: "/protected", Err: syscall.EACCES}, expected: true},
		{name: "standard os.ErrPermission without errno", err: os.ErrPermission, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSyscallPermission(tt.err)
			if got != tt.expected {
				t.Errorf("isSyscallPermission(%v) = %v, want %v", tt.err, got, tt.expected)
			}
		})
	}
}

func TestBrowsers_ExtractBrowserIDFromURI(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"browser://chrome/caches", "chrome"},
		{"browser://brave/caches", "brave"},
		{"browser://safari/caches", "safari"},
		{"browser://firefox/profiles", "firefox"},
		{"browser://arc/scriptcache", "arc"},
		{"browser://edge", "edge"},
		{"invalid-uri", ""},
		{"file:///tmp/cache", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := extractBrowserIDFromURI(tt.uri)
		if got != tt.expected {
			t.Errorf("extractBrowserIDFromURI(%q) = %q, want %q", tt.uri, got, tt.expected)
		}
	}
}

func TestBrowsers_CleanProfiles_ErrorBranches(t *testing.T) {
	tempHome := t.TempDir()

	// 1. Chromium clean: with allowlisted leaf as symlink (must be skipped by symlink guard)
	chromeSupport := filepath.Join(tempHome, "Library", "Application Support", "Google", "Chrome", "Default")
	_ = os.MkdirAll(chromeSupport, 0755)

	realDir := filepath.Join(tempHome, "real_target")
	_ = os.MkdirAll(realDir, 0755)
	sentinel := filepath.Join(realDir, "sentinel.txt")
	_ = os.WriteFile(sentinel, []byte("INVIOLABLE"), 0644)

	symlinkLeaf := filepath.Join(chromeSupport, "GPUCache")
	_ = os.Symlink(realDir, symlinkLeaf)

	bDef, ok := FindBrowserDef("chrome")
	if !ok {
		t.Fatalf("could not find chrome browser definition")
	}

	err := cleanChromiumBrowser(tempHome, "darwin", bDef)
	if err != nil {
		t.Fatalf("cleanChromiumBrowser returned error: %v", err)
	}

	// Symlink must not have been removed and real target must not have been touched
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel behind symlink was mutated: %v", err)
	}

	// Non-darwin OS with DarwinOnly browser definition returns nil early
	safariDef, _ := FindBrowserDef("safari")
	if err := cleanChromiumBrowser(tempHome, "linux", safariDef); err != nil {
		t.Errorf("cleanChromiumBrowser darwin-only on linux = %v, want nil", err)
	}

	// Browser definition with nil dirs returns nil early
	nilDef := BrowserDef{ID: "nil_dirs"}
	if err := cleanChromiumBrowser(tempHome, "darwin", nilDef); err != nil {
		t.Errorf("cleanChromiumBrowser nil dirs = %v, want nil", err)
	}

	// 2. Firefox clean: with nil dirs or allowlisted leaf as symlink
	if err := cleanFirefox(tempHome, "darwin", nilDef); err != nil {
		t.Errorf("cleanFirefox nil dirs = %v, want nil", err)
	}

	firefoxDef, ok := FindBrowserDef("firefox")
	if ok {
		firefoxSupport := filepath.Join(tempHome, "Library", "Application Support", "Firefox", "Profiles", "test.default")
		_ = os.MkdirAll(firefoxSupport, 0755)
		ffSymlink := filepath.Join(firefoxSupport, "startupCache")
		_ = os.Symlink(realDir, ffSymlink)

		if err := cleanFirefox(tempHome, "darwin", firefoxDef); err != nil {
			t.Fatalf("cleanFirefox returned error: %v", err)
		}
		if _, err := os.Stat(sentinel); err != nil {
			t.Fatalf("sentinel behind firefox symlink was mutated: %v", err)
		}
	}

	// 3. Safari clean: allowlisted leaf as symlink
	safariCache := filepath.Join(tempHome, "Library", "Caches", "com.apple.Safari")
	_ = os.MkdirAll(safariCache, 0755)
	safariSymlink := filepath.Join(safariCache, "WebKitCache")
	_ = os.Symlink(realDir, safariSymlink)

	if err := cleanSafari(tempHome); err != nil {
		t.Fatalf("cleanSafari returned error: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel behind safari symlink was mutated: %v", err)
	}
}

func TestBrowsers_ScanSafari_PermissionErrors(t *testing.T) {
	tempHome := t.TempDir()
	safariDef, ok := FindBrowserDef("safari")
	if !ok {
		t.Fatalf("safari def not found")
	}

	// 1. Non-existent cache dir returns nil cleanly
	targets, err := scanSafari(context.Background(), tempHome, "darwin", safariDef, false, time.Time{}, 0)
	if err != nil {
		t.Fatalf("scanSafari with nonexistent dir error = %v, want nil", err)
	}
	if len(targets) != 0 {
		t.Errorf("expected 0 targets, got %d", len(targets))
	}

	// 2. Canceled context returns ctx.Err()
	cacheRoot := safariDef.CacheDir(tempHome, "darwin")
	_ = os.MkdirAll(cacheRoot, 0755)
	_ = os.WriteFile(filepath.Join(cacheRoot, "WebKitCache"), []byte("data"), 0644)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = scanSafari(ctx, tempHome, "darwin", safariDef, false, time.Time{}, 0)
	if err == nil {
		t.Errorf("expected error on canceled context, got nil")
	}
}

func TestBrowsers_Clean_DefensiveRefusal_And_UnknownURI(t *testing.T) {
	tempHome := t.TempDir()
	mockExec := cleaner.NewMockExecutor()
	c := NewWithExecutorAndHome(mockExec, tempHome)

	// Target with unknown browser ID in virtual URI
	tUnknown := domain.Target{
		ID:         "unknown-browser",
		Path:       "browser://unknown-browser/caches",
		ReviewOnly: false,
	}

	// Target with malformed URI
	tMalformed := domain.Target{
		ID:         "malformed",
		Path:       "invalid-scheme://foo",
		ReviewOnly: false,
	}

	err := c.Clean(context.Background(), []domain.Target{tUnknown, tMalformed})
	if err != nil {
		t.Fatalf("Clean with unknown/malformed URIs returned error: %v", err)
	}

	// Canceled context in Clean returns error
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = c.Clean(ctx, []domain.Target{tUnknown})
	if err == nil {
		t.Fatalf("expected error on canceled context, got nil")
	}
}
