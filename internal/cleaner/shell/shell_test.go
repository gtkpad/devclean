package shell

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
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

func TestShellCleaner_Scan_MiseSeparateTargets(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	miseCache := filepath.Join(tempHome, "Library", "Caches", "mise")
	miseDownloads := filepath.Join(tempHome, ".local", "share", "mise", "downloads")

	if err := os.MkdirAll(miseCache, 0755); err != nil {
		t.Fatalf("failed to create miseCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(miseCache, "cache.dat"), []byte("cached runtime data"), 0644); err != nil {
		t.Fatalf("failed to write miseCache file: %v", err)
	}

	if err := os.MkdirAll(miseDownloads, 0755); err != nil {
		t.Fatalf("failed to create miseDownloads: %v", err)
	}
	if err := os.WriteFile(filepath.Join(miseDownloads, "node.tar.gz"), []byte("downloaded runtime archive"), 0644); err != nil {
		t.Fatalf("failed to write miseDownloads file: %v", err)
	}

	c := New()
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	var foundCache, foundDownloads bool
	for _, tgt := range targets {
		if tgt.Category == "shell-mise-cache" {
			foundCache = true
			if tgt.Name != "mise cache" {
				t.Errorf("expected name 'mise cache', got %q", tgt.Name)
			}
			if tgt.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow for mise cache, got %v", tgt.Risk)
			}
			if tgt.SizeBytes <= 0 {
				t.Errorf("expected SizeBytes > 0, got %d", tgt.SizeBytes)
			}
		}
		if tgt.Category == "shell-mise-downloads" {
			foundDownloads = true
			if tgt.Name != "mise downloads" {
				t.Errorf("expected name 'mise downloads', got %q", tgt.Name)
			}
			if tgt.Risk != domain.RiskLow {
				t.Errorf("expected RiskLow for mise downloads, got %v", tgt.Risk)
			}
			if tgt.SizeBytes <= 0 {
				t.Errorf("expected SizeBytes > 0, got %d", tgt.SizeBytes)
			}
		}
	}

	if !foundCache {
		t.Errorf("expected target with category 'shell-mise-cache' was not found")
	}
	if !foundDownloads {
		t.Errorf("expected target with category 'shell-mise-downloads' was not found")
	}
}

func TestShellCleaner_RiskLevels(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	dirs := []struct {
		relPath  string
		category string
		wantRisk domain.RiskLevel
	}{
		{filepath.Join("Library", "Caches", "mise"), "shell-mise-cache", domain.RiskLow},
		{filepath.Join(".local", "share", "mise", "downloads"), "shell-mise-downloads", domain.RiskLow},
		{filepath.Join("Library", "Caches", "gh"), "shell-gh-cache", domain.RiskLow},
		{filepath.Join(".cache", "pre-commit"), "shell-pre-commit-cache", domain.RiskMedium},
		{filepath.Join(".oh-my-zsh", "cache"), "shell-omz-cache", domain.RiskLow},
	}

	for _, d := range dirs {
		p := filepath.Join(tempHome, d.relPath)
		if err := os.MkdirAll(p, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d.relPath, err)
		}
		if err := os.WriteFile(filepath.Join(p, "sentinel.txt"), []byte("some cache bytes"), 0644); err != nil {
			t.Fatalf("failed to write sentinel in %s: %v", d.relPath, err)
		}
	}

	c := New()
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	foundRisks := make(map[string]domain.RiskLevel)
	for _, tgt := range targets {
		foundRisks[tgt.Category] = tgt.Risk
	}

	for _, d := range dirs {
		risk, ok := foundRisks[d.category]
		if !ok {
			t.Errorf("target category %q not found in Scan results", d.category)
			continue
		}
		if risk != d.wantRisk {
			t.Errorf("category %q: expected risk %v, got %v", d.category, d.wantRisk, risk)
		}
	}
}

func TestShellCleaner_NonInterferenceWithShellRC(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	// Create sensitive shell RC and history files with specific content
	sensitiveFiles := map[string]string{
		".zshrc":        "export PATH=$HOME/bin:$PATH\nalias ll='ls -la'\n",
		".bashrc":       "alias gs='git status'\n",
		".profile":      "export ENV=prod\n",
		".zsh_history":  ": 1600000000:0;echo sensitive_secret_command\n",
		".bash_history": "echo secret_password\n",
	}

	initialModTimes := make(map[string]int64)
	for name, content := range sensitiveFiles {
		p := filepath.Join(tempHome, name)
		if err := os.WriteFile(p, []byte(content), 0600); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s failed: %v", name, err)
		}
		initialModTimes[name] = info.ModTime().UnixNano()
	}

	// Also create a valid cache target so Scan() has something to return
	miseCache := filepath.Join(tempHome, "Library", "Caches", "mise")
	if err := os.MkdirAll(miseCache, 0755); err != nil {
		t.Fatalf("failed to create miseCache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(miseCache, "cache.dat"), []byte("cache data"), 0644); err != nil {
		t.Fatalf("failed to write miseCache file: %v", err)
	}

	c := New()
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan returned unexpected error: %v", err)
	}

	// Assert that none of the targets refer to any sensitive files
	for _, tgt := range targets {
		for name := range sensitiveFiles {
			if filepath.Base(tgt.Path) == name || tgt.Path == filepath.Join(tempHome, name) {
				t.Fatalf("VIOLATION: sensitive file %q inventoried as cleanup target: %+v", name, tgt)
			}
		}
	}

	// Clean the scanned targets
	if err := c.Clean(t.Context(), targets); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify all sensitive files still exist, have unchanged content, and unchanged timestamps
	for name, expectedContent := range sensitiveFiles {
		p := filepath.Join(tempHome, name)
		content, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("sensitive file %s was deleted or inaccessible: %v", name, err)
		}
		if string(content) != expectedContent {
			t.Fatalf("sensitive file %s content was modified! got %q, want %q", name, string(content), expectedContent)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatalf("stat %s failed: %v", name, err)
		}
		if info.ModTime().UnixNano() != initialModTimes[name] {
			t.Fatalf("sensitive file %s was touched (modtime modified)", name)
		}
	}
}

