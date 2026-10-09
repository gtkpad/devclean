package intelligence

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFindGitDir_InGitRepo(t *testing.T) {
	// This test assumes it's running inside the devclean git repo
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	got := findGitDir(wd)
	if got == "" {
		t.Skip("not running inside a git repo")
	}

	if filepath.Base(got) != ".git" {
		t.Errorf("findGitDir() = %q, want path ending in .git", got)
	}
}

func TestFindGitDir_NoRepo(t *testing.T) {
	tmp := t.TempDir()
	got := findGitDir(tmp)
	if got != "" {
		t.Errorf("findGitDir(tempdir) = %q, want empty", got)
	}
}

func TestGitActivityScore_NoGitDir(t *testing.T) {
	tmp := t.TempDir()
	score, err := GitActivityScore(context.Background(), tmp, 180*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0.0 {
		t.Errorf("GitActivityScore(no git) = %v, want 0.0", score)
	}
}

func TestGitActivityScore_InRepo(t *testing.T) {
	// This file is in the repo, so it should return a score
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	if findGitDir(wd) == "" {
		t.Skip("not running inside a git repo")
	}

	score, err := GitActivityScore(context.Background(), wd, 180*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Score should be between 0 and 1
	if score < 0.0 || score > 1.0 {
		t.Errorf("GitActivityScore() = %v, want [0.0, 1.0]", score)
	}
}

func TestAgeToActivityScore(t *testing.T) {
	maxAge := 180 * 24 * time.Hour

	tests := []struct {
		name string
		t    time.Time
		want float64
	}{
		{"just now", time.Now(), 0.0},
		{"half max", time.Now().Add(-90 * 24 * time.Hour), 0.5},
		{"at max", time.Now().Add(-maxAge), 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ageToActivityScore(tt.t, maxAge)
			if diff := got - tt.want; diff > 0.02 || diff < -0.02 {
				t.Errorf("ageToActivityScore() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

func TestFindGitRepo_WorktreePointer(t *testing.T) {
	root := t.TempDir()
	metaDir := filepath.Join(root, "main-repo", ".git", "worktrees", "wt1")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(root, "wt1-worktree")
	if err := os.MkdirAll(filepath.Join(wtDir, "sub", "dir"), 0755); err != nil {
		t.Fatal(err)
	}

	gitFile := filepath.Join(wtDir, ".git")
	if err := os.WriteFile(gitFile, []byte(fmt.Sprintf("gitdir: %s\n", metaDir)), 0644); err != nil {
		t.Fatal(err)
	}

	workTreeDir, gitDir := findGitRepo(filepath.Join(wtDir, "sub", "dir"))
	if workTreeDir != wtDir {
		t.Errorf("findGitRepo() workTreeDir = %q, want %q", workTreeDir, wtDir)
	}
	if gitDir != metaDir {
		t.Errorf("findGitRepo() gitDir = %q, want %q", gitDir, metaDir)
	}

	if got := findGitDir(filepath.Join(wtDir, "sub", "dir")); got != metaDir {
		t.Errorf("findGitDir() = %q, want %q", got, metaDir)
	}
}

func TestFindGitRepo_RelativeWorktreePointer(t *testing.T) {
	root := t.TempDir()
	metaDir := filepath.Join(root, "repo", ".git", "worktrees", "feat")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}

	wtDir := filepath.Join(root, "repo", "feat-wt")
	if err := os.MkdirAll(wtDir, 0755); err != nil {
		t.Fatal(err)
	}

	relPath, err := filepath.Rel(wtDir, metaDir)
	if err != nil {
		t.Fatal(err)
	}

	gitFile := filepath.Join(wtDir, ".git")
	if err := os.WriteFile(gitFile, []byte(fmt.Sprintf("gitdir: %s\n", relPath)), 0644); err != nil {
		t.Fatal(err)
	}

	workTreeDir, gitDir := findGitRepo(wtDir)
	if workTreeDir != wtDir {
		t.Errorf("findGitRepo() workTreeDir = %q, want %q", workTreeDir, wtDir)
	}
	if gitDir != metaDir {
		t.Errorf("findGitRepo() gitDir = %q, want %q", gitDir, metaDir)
	}
}

func TestGitActivityScore_IgnoredDirFallback(t *testing.T) {
	repoDir := t.TempDir()

	runGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v, out: %s", args, err, string(out))
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")

	gitignorePath := filepath.Join(repoDir, ".gitignore")
	if err := os.WriteFile(gitignorePath, []byte("node_modules/\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", ".gitignore")

	dateStr := time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339)
	commitCmd := exec.Command("git", "commit", "-m", "initial commit", "--date", dateStr)
	commitCmd.Dir = repoDir
	commitCmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit failed: %v, out: %s", err, string(out))
	}

	ignoredDir := filepath.Join(repoDir, "node_modules")
	if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ignoredDir, "pkg.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	maxAge := 180 * 24 * time.Hour
	score, err := GitActivityScore(context.Background(), ignoredDir, maxAge)
	if err != nil {
		t.Fatalf("GitActivityScore failed: %v", err)
	}

	// Score must be active (> 0.0) derived from repo's 10-day-old commit, and safe/active (< 1.0)
	if score <= 0.0 || score >= 1.0 {
		t.Errorf("GitActivityScore(ignoredDir) = %v, want (0.0, 1.0) based on repo commit fallback", score)
	}

	// Validate gitLastCommitTime directly
	lastCommit, err := gitLastCommitTime(context.Background(), repoDir, ignoredDir)
	if err != nil {
		t.Fatalf("gitLastCommitTime failed: %v", err)
	}
	if lastCommit.IsZero() {
		t.Error("expected non-zero lastCommit time for ignored directory via fallback")
	}
}

