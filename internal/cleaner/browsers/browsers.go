package browsers

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
)

type browsersCleaner struct {
	exec    cleaner.CommandExecutor
	homeDir string
	goos    string
}

// New creates a new browser cleaner with default system dependencies.
func New() domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &browsersCleaner{
		exec:    &cleaner.DefaultExecutor{},
		homeDir: home,
		goos:    runtime.GOOS,
	}
}

// NewWithExecutor creates a browser cleaner with a custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	home, _ := os.UserHomeDir()
	return &browsersCleaner{
		exec:    exec,
		homeDir: home,
		goos:    runtime.GOOS,
	}
}

// NewWithHome creates a browser cleaner with a custom home directory.
func NewWithHome(homeDir string) domain.Cleaner {
	return &browsersCleaner{
		exec:    &cleaner.DefaultExecutor{},
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

// NewWithExecutorAndHome creates a browser cleaner with custom executor and home directory.
func NewWithExecutorAndHome(exec cleaner.CommandExecutor, homeDir string) domain.Cleaner {
	return &browsersCleaner{
		exec:    exec,
		homeDir: homeDir,
		goos:    runtime.GOOS,
	}
}

// NewWithExecutorHomeAndOS creates a browser cleaner with custom executor, home directory, and OS.
func NewWithExecutorHomeAndOS(exec cleaner.CommandExecutor, homeDir, goos string) domain.Cleaner {
	return &browsersCleaner{
		exec:    exec,
		homeDir: homeDir,
		goos:    goos,
	}
}

// Name returns the identifier of the cleaner ("browsers").
func (c *browsersCleaner) Name() string {
	return "browsers"
}

// IsInstalled returns true if any supported browser AppSupport or Cache directory exists on disk.
func (c *browsersCleaner) IsInstalled() bool {
	if c.homeDir == "" {
		return false
	}
	for _, bDef := range GetBrowserDefinitions() {
		if bDef.DarwinOnly && c.goos != "darwin" {
			continue
		}
		if bDef.AppSupportDir != nil {
			appSupport := bDef.AppSupportDir(c.homeDir, c.goos)
			if appSupport != "" {
				if _, err := os.Stat(appSupport); err == nil {
					return true
				}
			}
		}
		if bDef.CacheDir != nil {
			cDir := bDef.CacheDir(c.homeDir, c.goos)
			if cDir != "" {
				if _, err := os.Stat(cDir); err == nil {
					return true
				}
			}
		}
	}
	return false
}

// Scan scans all supported browser installations according to ecosystem rules.
func (c *browsersCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if !rules.Enabled || c.homeDir == "" {
		return nil, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var cutoff time.Time
	if rules.MaxAgeDays > 0 {
		cutoff = time.Now().AddDate(0, 0, -rules.MaxAgeDays)
	}

	var allTargets []domain.Target

	for _, bDef := range GetBrowserDefinitions() {
		if bDef.DarwinOnly && c.goos != "darwin" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		running, uncertain := CheckBrowserProcessStatus(ctx, c.exec, bDef, c.goos)
		isBlocked := running || uncertain

		var targets []domain.Target
		var err error

		switch bDef.Engine {
		case EngineWebKit:
			targets, err = scanSafari(ctx, c.homeDir, bDef, isBlocked, cutoff, rules.MaxAgeDays)
		case EngineGecko:
			targets, err = scanFirefox(ctx, c.homeDir, c.goos, bDef, isBlocked, cutoff, rules.MaxAgeDays)
		case EngineChromium:
			targets, err = scanChromiumBrowser(ctx, c.homeDir, c.goos, bDef, isBlocked, cutoff, rules.MaxAgeDays)
		}

		if err != nil {
			return nil, err
		}
		allTargets = append(allTargets, targets...)
	}

	return allTargets, nil
}

// Clean executes surgical cache cleaning on eligible targets.
// Enforces defensive refusal on ReviewOnly and ScriptCache targets (D-05, D-07)
// and performs atomic pre-clean process revalidation (D-10).
func (c *browsersCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Stage 1: Defensive Refusal Guard (D-05, D-07)
		if t.ReviewOnly || t.Category == "browser-scriptcache" || strings.HasSuffix(t.Path, "/scriptcache") {
			continue
		}

		// Stage 2: Extract browser ID from virtual URI
		browserID := extractBrowserIDFromURI(t.Path)
		bDef, ok := FindBrowserDef(browserID)
		if !ok {
			continue
		}

		// Stage 3: Atomic Pre-Clean Process Revalidation (D-10)
		running, uncertain := CheckBrowserProcessStatus(ctx, c.exec, bDef, c.goos)
		if running || uncertain {
			continue // fail-closed race prevention: abort target deletion
		}

		// Stage 4: Surgical Unlinking
		switch bDef.Engine {
		case EngineWebKit:
			_ = cleanSafari(c.homeDir)
		case EngineGecko:
			_ = cleanFirefox(c.homeDir, c.goos, bDef)
		case EngineChromium:
			_ = cleanChromiumBrowser(c.homeDir, c.goos, bDef)
		}
	}

	return nil
}

// extractBrowserIDFromURI extracts the browser identifier from a virtual path (e.g. "browser://chrome/caches").
func extractBrowserIDFromURI(uri string) string {
	if !strings.HasPrefix(uri, "browser://") {
		return ""
	}
	trimmed := strings.TrimPrefix(uri, "browser://")
	parts := strings.Split(trimmed, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
