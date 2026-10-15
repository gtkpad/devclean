package shell

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
)

func TestShellCleaner_Name(t *testing.T) {
	c := New()
	if c.Name() != "shell" {
		t.Fatalf("expected cleaner name 'shell', got %q", c.Name())
	}
}

func TestShellCleaner_IsInstalled_LookPath(t *testing.T) {
	tests := []struct {
		name    string
		binName string
	}{
		{"mise in PATH", "mise"},
		{"gh in PATH", "gh"},
		{"pre-commit in PATH", "pre-commit"},
		{"zsh in PATH", "zsh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := cleaner.NewMockExecutor()
			// Mock all binaries as not found by default
			for _, b := range []string{"mise", "gh", "pre-commit", "zsh"} {
				mockExec.Errors["lookpath:"+b] = errors.New("not found")
			}
			// Enable only the test binary
			delete(mockExec.Errors, "lookpath:"+tt.binName)
			mockExec.Outputs["lookpath:"+tt.binName] = []byte("/bin/" + tt.binName)

			// Clean temp HOME to avoid local disk detection
			tempHome := t.TempDir()
			t.Setenv("HOME", tempHome)
			t.Setenv("ZDOTDIR", "")
			t.Setenv("PRE_COMMIT_HOME", "")
			t.Setenv("ZSH_CACHE_DIR", "")
			t.Setenv("XDG_DATA_HOME", "")
			t.Setenv("XDG_CACHE_HOME", "")

			c := NewWithExecutor(mockExec)
			if !c.IsInstalled() {
				t.Fatalf("expected IsInstalled() == true for %s in PATH", tt.binName)
			}
		})
	}
}

func TestShellCleaner_IsInstalled_DiskPresence(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	for _, b := range []string{"mise", "gh", "pre-commit", "zsh"} {
		mockExec.Errors["lookpath:"+b] = errors.New("not found")
	}

	cases := []struct {
		name      string
		setupPath string
		isFile    bool
	}{
		{"mise cache Darwin", filepath.Join("Library", "Caches", "mise"), false},
		{"mise cache XDG", filepath.Join(".cache", "mise"), false},
		{"mise downloads", filepath.Join(".local", "share", "mise", "downloads"), false},
		{"gh cache Darwin", filepath.Join("Library", "Caches", "gh"), false},
		{"gh cache XDG", filepath.Join(".cache", "gh"), false},
		{"pre-commit cache", filepath.Join(".cache", "pre-commit"), false},
		{"omz cache", filepath.Join(".oh-my-zsh", "cache"), false},
		{"zcompdump file", ".zcompdump-test-host-5.9", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tempHome := t.TempDir()
			t.Setenv("HOME", tempHome)
			t.Setenv("ZDOTDIR", "")
			t.Setenv("PRE_COMMIT_HOME", "")
			t.Setenv("ZSH_CACHE_DIR", "")
			t.Setenv("XDG_DATA_HOME", "")
			t.Setenv("XDG_CACHE_HOME", "")

			targetPath := filepath.Join(tempHome, tc.setupPath)
			if tc.isFile {
				if err := os.WriteFile(targetPath, []byte("dump data"), 0644); err != nil {
					t.Fatalf("failed to create sentinel file: %v", err)
				}
			} else {
				if err := os.MkdirAll(targetPath, 0755); err != nil {
					t.Fatalf("failed to create cache dir: %v", err)
				}
			}

			c := NewWithExecutor(mockExec)
			if !c.IsInstalled() {
				t.Fatalf("expected IsInstalled() == true when %s exists on disk", tc.setupPath)
			}
		})
	}
}

func TestShellCleaner_IsInstalled_Empty(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	for _, b := range []string{"mise", "gh", "pre-commit", "zsh"} {
		mockExec.Errors["lookpath:"+b] = errors.New("not found")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	c := NewWithExecutor(mockExec)
	if c.IsInstalled() {
		t.Fatalf("expected IsInstalled() == false when no binaries or cache dirs exist")
	}
}
