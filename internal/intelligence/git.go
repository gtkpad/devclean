package intelligence

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// GitActivityScore returns a 0.0-1.0 score based on how recently the target
// was committed to in git. More recent activity = lower score (less safe to delete).
// Returns 0.0 (neutral) if path is not in a git repo.
func GitActivityScore(ctx context.Context, targetPath string, maxAge time.Duration) (float64, error) {
	gitDir := findGitDir(targetPath)
	if gitDir == "" {
		return 0.0, nil
	}

	repoDir := filepath.Dir(gitDir)

	// Fast proxy: check .git/ mtime. If recent (< 7 days), refine with git log.
	info, err := os.Stat(gitDir)
	if err != nil {
		return 0.0, nil
	}

	gitMtime := info.ModTime()
	if time.Since(gitMtime) > 7*24*time.Hour {
		// Repo hasn't had any activity in 7+ days — use mtime as proxy
		return ageToActivityScore(gitMtime, maxAge), nil
	}

	// Refine: get last commit time for this specific path
	lastCommit, err := gitLastCommitTime(ctx, repoDir, targetPath)
	if err != nil {
		return 0.0, nil
	}
	if lastCommit.IsZero() {
		return 0.0, nil
	}

	return ageToActivityScore(lastCommit, maxAge), nil
}

// ageToActivityScore converts a timestamp to a 0.0-1.0 activity score.
// More recent = lower score (active project, don't delete).
// Older = higher score (inactive, safe to delete).
func ageToActivityScore(t time.Time, maxAge time.Duration) float64 {
	if maxAge <= 0 {
		return 0.0
	}
	age := time.Since(t)
	score := float64(age) / float64(maxAge)
	return clamp(score)
}

// findGitDir walks up the directory tree looking for a .git directory.
func findGitDir(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}

	dir := abs
	for {
		candidate := filepath.Join(dir, ".git")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// gitLastCommitTime gets the last commit time for a path within a git repo.
// Uses `--` separator before path arg to prevent flag injection.
func gitLastCommitTime(ctx context.Context, repoDir, targetPath string) (time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	relPath, err := filepath.Rel(repoDir, targetPath)
	if err != nil {
		relPath = targetPath
	}

	// Use -- separator for safety (prevents path from being interpreted as flag)
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "log", "--format=%ct", "-1", "--", relPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return time.Time{}, nil
	}

	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return time.Time{}, nil
	}

	ts, err := strconv.ParseInt(output, 10, 64)
	if err != nil {
		return time.Time{}, nil
	}

	return time.Unix(ts, 0), nil
}
