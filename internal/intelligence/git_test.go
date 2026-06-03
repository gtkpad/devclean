package intelligence

import (
	"context"
	"os"
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
