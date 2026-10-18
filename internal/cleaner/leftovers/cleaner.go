package leftovers

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

// leftoversCleaner coordinates LaunchAgent and Application Leftover discovery.
// It implements domain.Cleaner with an inviolable Clean() defensive refusal guard.
type leftoversCleaner struct {
	exec             cleaner.CommandExecutor
	homeDir          string
	goos             string
	appRoots         []string
	launchAgentsDirs []string
	libraryDir       string
}

// New creates a leftovers cleaner with default executor and system paths.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a leftovers cleaner with custom executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return NewWithConfig(exec, home, runtime.GOOS, nil, nil, "")
}

// NewWithConfig creates a leftovers cleaner with injectable directory roots
// and environment parameters enabling hermetic testing across Darwin and Linux (D-14, D-15).
func NewWithConfig(
	exec cleaner.CommandExecutor,
	homeDir, goos string,
	appRoots, launchAgentsDirs []string,
	libraryDir string,
) domain.Cleaner {
	if appRoots == nil {
		appRoots = []string{
			"/Applications",
			"/System/Applications",
			filepath.Join(homeDir, "Applications"),
		}
	}
	if launchAgentsDirs == nil {
		launchAgentsDirs = []string{
			filepath.Join(homeDir, "Library", "LaunchAgents"),
			"/Library/LaunchAgents",
		}
	}
	if libraryDir == "" {
		libraryDir = filepath.Join(homeDir, "Library")
	}

	return &leftoversCleaner{
		exec:             exec,
		homeDir:          homeDir,
		goos:             goos,
		appRoots:         appRoots,
		launchAgentsDirs: launchAgentsDirs,
		libraryDir:       libraryDir,
	}
}

// Name returns the cleaner ecosystem identifier.
func (c *leftoversCleaner) Name() string {
	return "leftovers"
}

// IsInstalled returns true if running on macOS or if a valid home directory is provided.
func (c *leftoversCleaner) IsInstalled() bool {
	return c.goos == "darwin" || c.homeDir != ""
}

// Scan aggregates stale LaunchAgents and application leftovers across configured roots (LEFT-01, LEFT-03).
func (c *leftoversCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled {
		return nil, nil
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var allTargets []domain.Target

	// 1. Scan LaunchAgents (LEFT-01, LEFT-02)
	launchTargets, err := scanLaunchAgents(ctx, c.launchAgentsDirs)
	if err == nil {
		allTargets = append(allTargets, launchTargets...)
	}

	// 2. Index installed applications and discover leftovers (LEFT-03)
	appIndex, err := scanInstalledApps(ctx, c.appRoots)
	if err == nil {
		leftoverTargets, err := scanAppLeftovers(ctx, c.libraryDir, appIndex)
		if err == nil {
			allTargets = append(allTargets, leftoverTargets...)
		}
	}

	return allTargets, nil
}

// Clean implements the inviolable defensive refusal guard (LEFT-04, D-13).
// Leftovers and LaunchAgents must NEVER be deleted automatically in v2.3.
// This method unconditionally refuses physical deletion, ensuring zero mutations
// even under direct test invocations or coordinator bypass.
func (c *leftoversCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	// Defensive refusal guard (LEFT-04, D-13):
	// Leftovers and LaunchAgents must NEVER be deleted automatically in v2.3.
	// This method unconditionally refuses physical deletion, ensuring zero mutations
	// even under direct test invocations or coordinator bypass.
	return nil
}
