package cleaner_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/cleaner/node"
	"github.com/gtkpad/devclean/internal/cleaner/pnpm"
	"github.com/gtkpad/devclean/internal/cleaner/shell"
	"github.com/gtkpad/devclean/internal/cleaner/xcode"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestProviders_NoShellInjection(t *testing.T) {
	// Audit that all virtual and ecosystem cleaners execute subprocesses
	// strictly via exec.CommandContext direct token slices, never through
	// sh -c, /bin/sh, or bash -c interpolation (POL-04, D-16).
	cleanerDir := "."
	violations, err := cleaner.AuditNoShellInjection(cleanerDir)
	if err != nil {
		t.Fatalf("audit failed: %v", err)
	}
	if len(violations) > 0 {
		for _, v := range violations {
			t.Errorf("shell injection risk detected: %s", v)
		}
	}
}

func TestCrossCleaner_SymlinkProtection_Phase08(t *testing.T) {
	tempDir := t.TempDir()
	outsideDir := filepath.Join(tempDir, "outside_project")
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}
	canaryPath := filepath.Join(outsideDir, "canary.txt")
	if err := os.WriteFile(canaryPath, []byte("canary-secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	symlinkPath := filepath.Join(tempDir, "symlink_cache")
	if err := os.Symlink(outsideDir, symlinkPath); err != nil {
		t.Fatal(err)
	}

	cleaners := []struct {
		name    string
		cleaner domain.Cleaner
	}{
		{"node", node.New([]string{tempDir})},
		{"pnpm", pnpm.NewWithExecutor(cleaner.NewMockExecutor())},
		{"shell", shell.NewWithExecutor(cleaner.NewMockExecutor())},
		{"xcode", xcode.NewWithDirExecutorAndOS(tempDir, cleaner.NewMockExecutor(), "darwin")},
	}

	for _, tc := range cleaners {
		t.Run(tc.name, func(t *testing.T) {
			targets := []domain.Target{
				{
					ID:       tc.name + "-symlink",
					Name:     tc.name + " symlink",
					Path:     symlinkPath,
					Category: tc.name + "-cache",
				},
			}

			if err := tc.cleaner.Clean(context.Background(), targets); err != nil {
				t.Fatalf("[%s] Clean() error: %v", tc.name, err)
			}

			// Canary in outside directory must NEVER be touched
			if _, err := os.Stat(canaryPath); err != nil {
				t.Fatalf("[%s] outside canary was deleted: %v", tc.name, err)
			}
			data, err := os.ReadFile(canaryPath)
			if err != nil || string(data) != "canary-secret" {
				t.Fatalf("[%s] outside canary corrupted: %v", tc.name, err)
			}
			// Symlink itself must remain intact (not traversed or removed)
			if _, err := os.Lstat(symlinkPath); err != nil {
				t.Fatalf("[%s] symlink was not preserved: %v", tc.name, err)
			}
		})
	}
}

