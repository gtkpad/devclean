package macos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

type macosCleaner struct {
	exec    cleaner.CommandExecutor
	homeDir string
	goos    string
}

// New creates a macOS cleaner with default command executor and user home directory.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a macOS cleaner with custom executor (for hermetic testing).
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &macosCleaner{
		exec:    exec,
		homeDir: home,
		goos:    runtime.GOOS,
	}
}

// NewWithDirAndExecutor allows explicit home directory injection for tests.
func NewWithDirAndExecutor(homeDir string, exec cleaner.CommandExecutor) domain.Cleaner {
	return &macosCleaner{
		exec:    exec,
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

func (c *macosCleaner) Name() string {
	return "macos"
}

func (c *macosCleaner) IsInstalled() bool {
	return c.goos == "darwin" || c.homeDir != ""
}

func (c *macosCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled {
		return nil, nil
	}

	var allTargets []domain.Target

	// 1. User Essentials: Caches (D-05, D-06)
	if userTargets, err := scanUserCaches(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, userTargets...)
	}

	// 2. User Essentials: Logs (D-06)
	if logTargets, err := scanUserLogs(ctx, c.homeDir); err == nil {
		allTargets = append(allTargets, logTargets...)
	}

	// 3. User Essentials: Trash (~/.Trash) (D-01, D-02)
	trashTarget := scanTrash(c.homeDir)
	if trashTarget.ID != "" {
		allTargets = append(allTargets, trashTarget)
	}

	// 4. Apple Daemons: 8 canonical daemons (D-13)
	if daemonTargets, err := scanDaemons(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, daemonTargets...)
	}

	// 5. Group Containers: positive allowlist (D-08)
	if groupTargets, err := scanGroupContainers(ctx, c.homeDir); err == nil {
		allTargets = append(allTargets, groupTargets...)
	}

	// 6. Application Support Leaves & Spotify (D-07)
	if appSupportTargets, err := scanApplicationSupport(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, appSupportTargets...)
	}

	// 7. System Logs, Diagnostic Reports & Code Sign Clones (D-09..D-12)
	if sysTargets, err := scanSystemLogsAndReports(ctx, c.exec, c.homeDir); err == nil {
		allTargets = append(allTargets, sysTargets...)
	}

	return allTargets, nil
}

func (c *macosCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Stage 1 guard: reject ReviewOnly targets
		if t.ReviewOnly {
			continue
		}

		// Dedicated Trash purger (D-03)
		if t.Category == "macos-trash" {
			if err := cleanTrash(ctx, t.Path); err != nil {
				return err
			}
			continue
		}

		// Pre-deletion safety verification: Symlink Guard & Ownership (D-16)
		info, err := os.Lstat(t.Path)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			continue // Do not follow or remove symlinked root
		}
		if !isOwnedByCurrentUser(info) {
			continue // Refuse to delete items owned by other users
		}
		if safety.IsCoreBlacklisted(t.Path) || IsWhitelisted(t.Path) {
			continue
		}

		// Pre-deletion Process Guard revalidation (D-14)
		bundleID := resolveTargetBundleID(t)
		running, uncertain := IsBundleProcessRunning(ctx, c.exec, bundleID)
		if running || uncertain {
			continue // Abort deletion if process started post-scan!
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return fmt.Errorf("cleaning %s: %w", t.Path, err)
		}
	}
	return nil
}

func resolveTargetBundleID(t domain.Target) string {
	if t.Category == "macos-spotify-cache" {
		return "com.spotify.client"
	}
	if t.ID == "macos-code-sign-clones" {
		return "com.google.Chrome.code_sign_clone"
	}
	if strings.HasPrefix(t.ID, "macos-cache-") {
		return strings.TrimPrefix(t.ID, "macos-cache-")
	}
	if strings.HasPrefix(t.ID, "macos-log-") {
		return strings.TrimPrefix(t.ID, "macos-log-")
	}
	if strings.HasPrefix(t.ID, "macos-daemon-") {
		daemonID := strings.TrimPrefix(t.ID, "macos-daemon-")
		for _, d := range canonicalDaemons {
			if d.ID == daemonID {
				return d.BundleID
			}
		}
		return daemonID
	}
	return filepath.Base(t.Path)
}
