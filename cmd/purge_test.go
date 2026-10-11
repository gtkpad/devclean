package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupTestRepo(t *testing.T, dir string, branch string, daysAgo int, dirty bool) {
	t.Helper()
	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.email", "tester@example.com")
	runGit("config", "user.name", "Tester")
	if branch != "" {
		runGit("checkout", "-b", branch)
	}

	pkgJSON := filepath.Join(dir, "package.json")
	if err := os.WriteFile(pkgJSON, []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "package.json")

	dateStr := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)
	commitCmd := exec.Command("git", "commit", "-m", "init", "--date", dateStr)
	commitCmd.Dir = dir
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("commit failed: %v, out: %s", err, string(out))
	}

	// Create node_modules artifact
	nmDir := filepath.Join(dir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "dummy.js"), []byte("console.log('x')"), 0644); err != nil {
		t.Fatal(err)
	}

	if dirty {
		if err := os.WriteFile(filepath.Join(dir, "uncommitted.txt"), []byte("dirty content"), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func createPurgeConfig(t *testing.T, tmpDir string, scanDirs ...string) string {
	t.Helper()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	var scanPathsYAML strings.Builder
	for _, d := range scanDirs {
		scanPathsYAML.WriteString(fmt.Sprintf("  - %q\n", d))
	}
	content := fmt.Sprintf(`scan_paths:
%secosystems:
  docker:
    enabled: false
  go:
    enabled: false
  dotnet:
    enabled: false
  python:
    enabled: false
  rust:
    enabled: false
  java:
    enabled: false
  node:
    enabled: true
safety:
  trash_enabled: true
`, scanPathsYAML.String())
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestPurgeCmd_Filters(t *testing.T) {
	origConfigPath := configPath
	origQuiet := quiet
	origStdinReader := purgeStdinReader
	origIsTerminal := purgeIsTerminal

	defer func() {
		configPath = origConfigPath
		quiet = origQuiet
		purgeStdinReader = origStdinReader
		purgeIsTerminal = origIsTerminal
	}()

	t.Run("ActiveRepoDiscarded", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo-active")
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Active commit: 10 days ago (< 60 days default)
		setupTestRepo(t, repoDir, "main", 10, false)

		configPath = createPurgeConfig(t, tmpDir, repoDir)
		quiet = false

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--yes"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "Nenhum artefato elegível para purge encontrado.") {
			t.Errorf("expected no eligible targets message, got: %s", out)
		}

		// node_modules should not be deleted
		if _, err := os.Stat(filepath.Join(repoDir, "node_modules")); err != nil {
			t.Errorf("expected node_modules to remain intact in active repo: %v", err)
		}
	})

	t.Run("DirtyTreeSkippedOnMergedOnly", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo-dirty")
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Inactive commit (70 days ago), but dirty tree
		setupTestRepo(t, repoDir, "main", 70, true)

		configPath = createPurgeConfig(t, tmpDir, repoDir)
		quiet = false

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--merged-only", "--yes"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "Nenhum artefato elegível para purge encontrado.") {
			t.Errorf("expected dirty repo to be skipped, got: %s", out)
		}

		// node_modules should not be deleted
		if _, err := os.Stat(filepath.Join(repoDir, "node_modules")); err != nil {
			t.Errorf("expected node_modules to remain intact in dirty repo: %v", err)
		}
	})

	t.Run("InteractivePreviewAndCancel", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo-inactive")
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		// Inactive commit: 70 days ago
		setupTestRepo(t, repoDir, "main", 70, false)

		configPath = createPurgeConfig(t, tmpDir, repoDir)
		quiet = false
		purgeIsTerminal = func(fd int) bool { return true }
		purgeStdinReader = strings.NewReader("n\n")

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{}) // No --yes, no --dry-run

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "REPOSITORY") || !strings.Contains(out, "node_modules") {
			t.Errorf("expected preview table containing REPOSITORY and node_modules, got: %s", out)
		}
		if !strings.Contains(out, "Expurgo cancelado pelo usuário.") {
			t.Errorf("expected cancellation message, got: %s", out)
		}

		// node_modules must remain intact
		if _, err := os.Stat(filepath.Join(repoDir, "node_modules")); err != nil {
			t.Errorf("expected node_modules to remain intact after cancel: %v", err)
		}
	})

	t.Run("NonTTYRequiresYesFlag", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo-inactive")
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		setupTestRepo(t, repoDir, "main", 70, false)

		configPath = createPurgeConfig(t, tmpDir, repoDir)
		quiet = false
		purgeIsTerminal = func(fd int) bool { return false }

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{}) // No --yes in non-TTY

		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected error in non-TTY mode without --yes, got nil")
		}
		if !strings.Contains(err.Error(), "TTY") || !strings.Contains(err.Error(), "--yes") {
			t.Errorf("expected error mentioning TTY and --yes, got: %v", err)
		}
	})

	t.Run("DryRunPreviewWithoutDeletion", func(t *testing.T) {
		tmpDir := t.TempDir()
		repoDir := filepath.Join(tmpDir, "repo-inactive")
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			t.Fatal(err)
		}
		setupTestRepo(t, repoDir, "main", 70, false)

		configPath = createPurgeConfig(t, tmpDir, repoDir)
		quiet = false

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--dry-run"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error with --dry-run: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "REPOSITORY") || !strings.Contains(out, "node_modules") {
			t.Errorf("expected preview table with --dry-run, got: %s", out)
		}

		// node_modules must remain intact
		if _, err := os.Stat(filepath.Join(repoDir, "node_modules")); err != nil {
			t.Errorf("expected node_modules to remain intact in dry-run: %v", err)
		}
	})

	t.Run("NonGitDirectorySilentlyIgnored", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonGitDir := filepath.Join(tmpDir, "non-git-app")
		if err := os.MkdirAll(filepath.Join(nonGitDir, "node_modules"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nonGitDir, "package.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nonGitDir, "node_modules", "lib.js"), []byte("code"), 0644); err != nil {
			t.Fatal(err)
		}

		configPath = createPurgeConfig(t, tmpDir, nonGitDir)
		quiet = false

		cmd := newPurgeCmd()
		var outBuf bytes.Buffer
		cmd.SetOut(&outBuf)
		cmd.SetArgs([]string{"--yes"})

		if err := cmd.Execute(); err != nil {
			t.Fatalf("non-git directory caused unexpected error: %v", err)
		}

		out := outBuf.String()
		if !strings.Contains(out, "Nenhum artefato elegível para purge encontrado.") {
			t.Errorf("expected silent ignore of non-git folder, got: %s", out)
		}

		if _, err := os.Stat(filepath.Join(nonGitDir, "node_modules")); err != nil {
			t.Errorf("expected node_modules to remain intact in non-git dir: %v", err)
		}
	})

	t.Run("MutuallyExclusiveTrashPermanent", func(t *testing.T) {
		cmd := newPurgeCmd()
		cmd.SetArgs([]string{"--trash", "--permanent"})
		err := cmd.Execute()
		if err == nil {
			t.Fatal("expected mutually exclusive error, got nil")
		}
		if !strings.Contains(err.Error(), "--trash and --permanent are mutually exclusive") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
