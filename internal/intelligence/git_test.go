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

func setupGitRepoHelper(t *testing.T, defaultBranch string) (string, func(args ...string)) {
	t.Helper()
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
	runGit("config", "user.email", "tester@example.com")
	runGit("config", "user.name", "Tester")
	if defaultBranch != "" {
		runGit("checkout", "-b", defaultBranch)
	}
	return repoDir, runGit
}

func commitHelper(t *testing.T, repoDir string, msg string, daysAgo int) {
	t.Helper()
	dateStr := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour).Format(time.RFC3339)
	cmd := exec.Command("git", "commit", "-m", msg, "--date", dateStr)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE="+dateStr,
		"GIT_COMMITTER_DATE="+dateStr,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("commit %q failed: %v, out: %s", msg, err, string(out))
	}
}

func TestGit_FindGitRepoLocal(t *testing.T) {
	repoDir, _ := setupGitRepoHelper(t, "main")

	subDir := filepath.Join(repoDir, "cmd", "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	workTree, gitDir := FindGitRepoLocal(subDir)
	if workTree != repoDir {
		t.Errorf("FindGitRepoLocal(%q) workTree = %q, want %q", subDir, workTree, repoDir)
	}
	expectedGitDir := filepath.Join(repoDir, ".git")
	if gitDir != expectedGitDir {
		t.Errorf("FindGitRepoLocal(%q) gitDir = %q, want %q", subDir, gitDir, expectedGitDir)
	}

	nonGitDir := t.TempDir()
	workTree, gitDir = FindGitRepoLocal(nonGitDir)
	if workTree != "" || gitDir != "" {
		t.Errorf("FindGitRepoLocal(nonGit) = (%q, %q), want empty strings", workTree, gitDir)
	}
}

func TestGit_GetRepoLastCommitTime(t *testing.T) {
	repoDir, runGit := setupGitRepoHelper(t, "main")

	testFile := filepath.Join(repoDir, "file.txt")
	if err := os.WriteFile(testFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "file.txt")
	commitHelper(t, repoDir, "initial", 5)

	ts, err := GetRepoLastCommitTime(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("GetRepoLastCommitTime failed: %v", err)
	}
	if ts.IsZero() {
		t.Fatal("expected non-zero commit time")
	}

	expectedAge := 5 * 24 * time.Hour
	actualAge := time.Since(ts)
	diff := actualAge - expectedAge
	if diff < -time.Minute || diff > time.Minute {
		t.Errorf("GetRepoLastCommitTime() age = %v, want ~%v", actualAge, expectedAge)
	}

	// Non-git directory
	nonGitDir := t.TempDir()
	tsNonGit, err := GetRepoLastCommitTime(context.Background(), nonGitDir)
	if err != nil {
		t.Fatalf("GetRepoLastCommitTime(nonGit) error = %v, want nil", err)
	}
	if !tsNonGit.IsZero() {
		t.Errorf("GetRepoLastCommitTime(nonGit) = %v, want zero time", tsNonGit)
	}
}

func TestGit_IsWorkingTreeClean(t *testing.T) {
	repoDir, runGit := setupGitRepoHelper(t, "main")

	testFile := filepath.Join(repoDir, "tracked.txt")
	if err := os.WriteFile(testFile, []byte("v1"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "tracked.txt")
	commitHelper(t, repoDir, "commit tracked", 1)

	// Clean working tree
	clean, err := IsWorkingTreeClean(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean failed: %v", err)
	}
	if !clean {
		t.Error("expected clean working tree to return true")
	}

	// Untracked file
	untrackedFile := filepath.Join(repoDir, "untracked.txt")
	if err := os.WriteFile(untrackedFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	clean, err = IsWorkingTreeClean(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean failed: %v", err)
	}
	if clean {
		t.Error("expected untracked file to make working tree dirty (false)")
	}

	// Remove untracked file, modify tracked file
	_ = os.Remove(untrackedFile)
	if err := os.WriteFile(testFile, []byte("v2"), 0644); err != nil {
		t.Fatal(err)
	}
	clean, err = IsWorkingTreeClean(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean failed: %v", err)
	}
	if clean {
		t.Error("expected modified file to make working tree dirty (false)")
	}

	// Stage the modified file
	runGit("add", "tracked.txt")
	clean, err = IsWorkingTreeClean(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean failed: %v", err)
	}
	if clean {
		t.Error("expected staged changes to make working tree dirty (false)")
	}

	// Commit staged file -> clean again
	commitHelper(t, repoDir, "commit v2", 0)
	clean, err = IsWorkingTreeClean(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsWorkingTreeClean failed: %v", err)
	}
	if !clean {
		t.Error("expected working tree to be clean after commit")
	}
}

func TestGit_IsBranchMerged(t *testing.T) {
	// Case 1: On main branch directly
	repoDir, runGit := setupGitRepoHelper(t, "main")
	testFile := filepath.Join(repoDir, "main.txt")
	_ = os.WriteFile(testFile, []byte("main content"), 0644)
	runGit("add", "main.txt")
	commitHelper(t, repoDir, "initial main commit", 10)

	merged, err := IsBranchMerged(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsBranchMerged on main branch failed: %v", err)
	}
	if !merged {
		t.Error("expected HEAD on main branch to be considered merged")
	}

	// Case 2: Create feature branch with unmerged commit
	runGit("checkout", "-b", "feature-x")
	featFile := filepath.Join(repoDir, "feat.txt")
	_ = os.WriteFile(featFile, []byte("feature content"), 0644)
	runGit("add", "feat.txt")
	commitHelper(t, repoDir, "feature commit", 2)

	merged, err = IsBranchMerged(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsBranchMerged on feature branch failed: %v", err)
	}
	if merged {
		t.Error("expected unmerged feature branch to return false")
	}

	// Case 3: Merge feature branch into main
	runGit("checkout", "main")
	runGit("merge", "--no-ff", "feature-x", "-m", "merge feature-x")

	// Switch back to feature-x: now feature-x is ancestor of main!
	runGit("checkout", "feature-x")
	merged, err = IsBranchMerged(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("IsBranchMerged on merged feature branch failed: %v", err)
	}
	if !merged {
		t.Error("expected merged feature branch to return true")
	}

	// Case 4: Repository using master instead of main
	repoMaster, runMaster := setupGitRepoHelper(t, "master")
	_ = os.WriteFile(filepath.Join(repoMaster, "file.txt"), []byte("data"), 0644)
	runMaster("add", "file.txt")
	commitHelper(t, repoMaster, "master commit", 5)

	merged, err = IsBranchMerged(context.Background(), repoMaster)
	if err != nil {
		t.Fatalf("IsBranchMerged on master failed: %v", err)
	}
	if !merged {
		t.Error("expected HEAD on master branch to be considered merged")
	}

	// Case 5: Repository without main or master
	repoDev, runDev := setupGitRepoHelper(t, "dev")
	_ = os.WriteFile(filepath.Join(repoDev, "file.txt"), []byte("data"), 0644)
	runDev("add", "file.txt")
	commitHelper(t, repoDev, "dev commit", 5)

	_, err = IsBranchMerged(context.Background(), repoDev)
	if err == nil {
		t.Error("expected error when neither main nor master branch exists")
	}
}

