package shell

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

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

func TestShellCleaner_Scan_ZshCompletionAggregated(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	files := map[string]int{
		".zcompdump-mac-5.9": 100,
		".zcompdump-mac-5.8": 200,
		".zcompdump":         50,
	}

	var expectedTotalSize int64
	for name, size := range files {
		p := filepath.Join(tempHome, name)
		data := make([]byte, size)
		if err := os.WriteFile(p, data, 0644); err != nil {
			t.Fatalf("failed to create dump file %s: %v", name, err)
		}
		expectedTotalSize += int64(size)
	}

	c := New()
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	var compdumpTargets []domain.Target
	for _, tgt := range targets {
		if tgt.Path == "shell://zsh/compdump" {
			compdumpTargets = append(compdumpTargets, tgt)
		}
	}

	if len(compdumpTargets) != 1 {
		t.Fatalf("expected exactly 1 'shell://zsh/compdump' target, got %d", len(compdumpTargets))
	}

	tgt := compdumpTargets[0]
	if tgt.ID != "shell-zsh-compdump" {
		t.Errorf("expected ID 'shell-zsh-compdump', got %q", tgt.ID)
	}
	if tgt.Name != "Zsh completion cache" {
		t.Errorf("expected Name 'Zsh completion cache', got %q", tgt.Name)
	}
	if tgt.Category != "shell-zsh-compdump" {
		t.Errorf("expected Category 'shell-zsh-compdump', got %q", tgt.Category)
	}
	if tgt.Risk != domain.RiskLow {
		t.Errorf("expected RiskLow, got %v", tgt.Risk)
	}
	if tgt.SizeBytes != expectedTotalSize {
		t.Errorf("expected SizeBytes %d, got %d", expectedTotalSize, tgt.SizeBytes)
	}
}

func TestShellCleaner_Clean_ZshCompletionAggregated(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	dumpFiles := []string{
		".zcompdump-mac-5.9",
		".zcompdump-mac-5.8",
		".zcompdump",
	}

	for _, name := range dumpFiles {
		p := filepath.Join(tempHome, name)
		if err := os.WriteFile(p, []byte("dump contents"), 0644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
	}

	c := New()
	target := domain.Target{
		ID:       "shell-zsh-compdump",
		Name:     "Zsh completion cache",
		Path:     "shell://zsh/compdump",
		Category: "shell-zsh-compdump",
	}

	if err := c.Clean(t.Context(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	for _, name := range dumpFiles {
		p := filepath.Join(tempHome, name)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("expected file %s to be deleted, but it still exists (err=%v)", name, err)
		}
	}
}

func TestShellCleaner_Clean_PhysicalCaches(t *testing.T) {
	tempHome := t.TempDir()
	ghPath := filepath.Join(tempHome, "Library", "Caches", "gh")
	if err := os.MkdirAll(ghPath, 0755); err != nil {
		t.Fatalf("failed to create ghPath: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ghPath, "token_cache.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	c := New()
	target := domain.Target{
		ID:       "test-gh",
		Name:     "GitHub CLI cache",
		Path:     ghPath,
		Category: "shell-gh-cache",
	}

	if err := c.Clean(t.Context(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(ghPath); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, but stat returned err=%v", ghPath, err)
	}
}

func TestShellCleaner_Clean_SymlinkProtection(t *testing.T) {
	tempHome := t.TempDir()
	realCache := filepath.Join(tempHome, "real_cache_dir")
	if err := os.MkdirAll(realCache, 0755); err != nil {
		t.Fatalf("failed to create realCache: %v", err)
	}
	canaryFile := filepath.Join(realCache, "canary.txt")
	if err := os.WriteFile(canaryFile, []byte("vital safe data"), 0644); err != nil {
		t.Fatalf("failed to write canary: %v", err)
	}

	symlinkPath := filepath.Join(tempHome, "symlink_cache")
	if err := os.Symlink(realCache, symlinkPath); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	c := New()
	target := domain.Target{
		ID:       "test-symlink",
		Name:     "Symlink Target",
		Path:     symlinkPath,
		Category: "shell-symlink",
	}

	if err := c.Clean(t.Context(), []domain.Target{target}); err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	// Verify canary in real dir was NOT removed!
	if _, err := os.Stat(canaryFile); err != nil {
		t.Fatalf("VIOLATION: symlink target contents were deleted! canary stat err=%v", err)
	}
}

func TestShellCleaner_IsInstalled_EnvVars(t *testing.T) {
	mockExec := cleaner.NewMockExecutor()
	for _, b := range []string{"mise", "gh", "pre-commit", "zsh"} {
		mockExec.Errors["lookpath:"+b] = errors.New("not found")
	}

	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	// 1. PRE_COMMIT_HOME
	preCommitDir := t.TempDir()
	t.Setenv("PRE_COMMIT_HOME", preCommitDir)
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("ZDOTDIR", "")

	c := NewWithExecutor(mockExec)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when PRE_COMMIT_HOME exists")
	}

	// 2. ZSH_CACHE_DIR
	t.Setenv("PRE_COMMIT_HOME", "")
	zshCacheDir := t.TempDir()
	t.Setenv("ZSH_CACHE_DIR", zshCacheDir)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when ZSH_CACHE_DIR exists")
	}

	// 3. XDG_DATA_HOME
	t.Setenv("ZSH_CACHE_DIR", "")
	xdgDataDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", xdgDataDir)
	_ = os.MkdirAll(filepath.Join(xdgDataDir, "mise", "downloads"), 0755)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when XDG_DATA_HOME/mise/downloads exists")
	}

	// 4. XDG_CACHE_HOME
	t.Setenv("XDG_DATA_HOME", "")
	xdgCacheDir := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", xdgCacheDir)
	_ = os.MkdirAll(filepath.Join(xdgCacheDir, "gh"), 0755)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when XDG_CACHE_HOME/gh exists")
	}

	// 5. ZDOTDIR compdump
	t.Setenv("XDG_CACHE_HOME", "")
	zdotDir := t.TempDir()
	t.Setenv("ZDOTDIR", zdotDir)
	_ = os.WriteFile(filepath.Join(zdotDir, ".zcompdump-custom"), []byte("data"), 0644)
	if !c.IsInstalled() {
		t.Errorf("expected IsInstalled == true when ZDOTDIR/.zcompdump* exists")
	}
}

func TestShellCleaner_Scan_CustomEnvDirs(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)

	customPreCommit := t.TempDir()
	customZshCache := t.TempDir()
	customXdgData := t.TempDir()
	customXdgCache := t.TempDir()

	t.Setenv("PRE_COMMIT_HOME", customPreCommit)
	t.Setenv("ZSH_CACHE_DIR", customZshCache)
	t.Setenv("XDG_DATA_HOME", customXdgData)
	t.Setenv("XDG_CACHE_HOME", customXdgCache)
	t.Setenv("ZDOTDIR", "")

	_ = os.WriteFile(filepath.Join(customPreCommit, "repo.dat"), []byte("commit hook"), 0644)
	_ = os.WriteFile(filepath.Join(customZshCache, "omz.dat"), []byte("omz cache data"), 0644)
	_ = os.MkdirAll(filepath.Join(customXdgData, "mise", "downloads"), 0755)
	_ = os.WriteFile(filepath.Join(customXdgData, "mise", "downloads", "rust.tar.gz"), []byte("rust tar"), 0644)
	_ = os.MkdirAll(filepath.Join(customXdgCache, "gh"), 0755)
	_ = os.WriteFile(filepath.Join(customXdgCache, "gh", "hosts.yml"), []byte("hosts"), 0644)

	c := New()
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	categories := make(map[string]bool)
	for _, tgt := range targets {
		categories[tgt.Category] = true
	}

	for _, expectedCat := range []string{
		"shell-pre-commit-cache",
		"shell-omz-cache",
		"shell-mise-downloads",
		"shell-gh-cache",
	} {
		if !categories[expectedCat] {
			t.Errorf("expected category %q not found in scan results with custom env dirs", expectedCat)
		}
	}
}

func TestShellCleaner_Scan_ZDOTDIR_And_TTL(t *testing.T) {
	tempHome := t.TempDir()
	zdotDir := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", zdotDir)
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	// Create a dump file in ZDOTDIR modified 45 days ago
	oldFile := filepath.Join(zdotDir, ".zcompdump-old")
	if err := os.WriteFile(oldFile, []byte("old dump"), 0644); err != nil {
		t.Fatalf("failed to write old dump file: %v", err)
	}
	oldTime := time.Now().AddDate(0, 0, -45)
	_ = os.Chtimes(oldFile, oldTime, oldTime)

	c := New()
	// Scan with MaxAgeDays = 30 -> should be TTLEligible = true
	targets, err := c.Scan(t.Context(), domain.EcosystemRules{MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 target, got %d", len(targets))
	}
	if !targets[0].TTLEligible {
		t.Errorf("expected TTLEligible == true for 45-day-old file with MaxAgeDays=30")
	}

	// Now create a new dump file in HOME modified now
	newFile := filepath.Join(tempHome, ".zcompdump-new")
	if err := os.WriteFile(newFile, []byte("fresh dump"), 0644); err != nil {
		t.Fatalf("failed to write fresh dump file: %v", err)
	}

	targets, err = c.Scan(t.Context(), domain.EcosystemRules{MaxAgeDays: 30})
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 aggregated target, got %d", len(targets))
	}
	// Because fresh dump was touched now, mostRecent is not before cutoff -> TTLEligible = false
	if targets[0].TTLEligible {
		t.Errorf("expected TTLEligible == false when fresh dump file is present")
	}
}

func TestShellCleaner_Scan_Exclusions(t *testing.T) {
	tempHome := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", "")
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	miseCache := filepath.Join(tempHome, "Library", "Caches", "mise")
	ghCache := filepath.Join(tempHome, "Library", "Caches", "gh")
	_ = os.MkdirAll(miseCache, 0755)
	_ = os.WriteFile(filepath.Join(miseCache, "data"), []byte("data"), 0644)
	_ = os.MkdirAll(ghCache, 0755)
	_ = os.WriteFile(filepath.Join(ghCache, "data"), []byte("data"), 0644)
	_ = os.WriteFile(filepath.Join(tempHome, ".zcompdump"), []byte("dump"), 0644)

	c := New()

	// 1. Exclude virtual target
	targets, _ := c.Scan(t.Context(), domain.EcosystemRules{
		ExcludePaths: []string{"shell://zsh/compdump", "**/Library/Caches/gh"},
	})
	for _, tgt := range targets {
		if tgt.Path == "shell://zsh/compdump" {
			t.Errorf("expected shell://zsh/compdump to be excluded")
		}
		if tgt.Category == "shell-gh-cache" {
			t.Errorf("expected gh cache to be excluded via pattern")
		}
	}
}

func TestShellCleaner_Clean_BlacklistAndZDOTDIR(t *testing.T) {
	tempHome := t.TempDir()
	zdotDir := t.TempDir()
	t.Setenv("HOME", tempHome)
	t.Setenv("ZDOTDIR", zdotDir)
	t.Setenv("PRE_COMMIT_HOME", "")
	t.Setenv("ZSH_CACHE_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	// Create dump in ZDOTDIR
	zdotDump := filepath.Join(zdotDir, ".zcompdump-test")
	_ = os.WriteFile(zdotDump, []byte("zdot dump"), 0644)

	// Create fake .ssh dir that should NEVER be removed by Clean
	sshDir := filepath.Join(tempHome, ".ssh")
	_ = os.MkdirAll(sshDir, 0700)
	_ = os.WriteFile(filepath.Join(sshDir, "id_rsa"), []byte("secret"), 0600)

	c := New()
	// Clean virtual target with ZDOTDIR set and a blacklisted target
	err := c.Clean(t.Context(), []domain.Target{
		{Path: "shell://zsh/compdump"},
		{Path: sshDir}, // Core blacklisted (.ssh)
	})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(zdotDump); !os.IsNotExist(err) {
		t.Errorf("expected ZDOTDIR dump file to be deleted")
	}
	if _, err := os.Stat(sshDir); os.IsNotExist(err) {
		t.Errorf("VIOLATION: .ssh dir was deleted by Clean")
	}
}



