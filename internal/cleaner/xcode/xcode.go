package xcode

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

type xcodeCleaner struct {
	exec    cleaner.CommandExecutor
	baseDir string
	goos    string
}

// New creates a new xcodeCleaner with default executor and home directory.
func New() domain.Cleaner {
	return NewWithExecutor(&cleaner.DefaultExecutor{})
}

// NewWithExecutor creates a new xcodeCleaner with a custom command executor.
func NewWithExecutor(exec cleaner.CommandExecutor) domain.Cleaner {
	baseDir := ""
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		baseDir = filepath.Join(home, "Library", "Developer", "Xcode")
	}
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    runtime.GOOS,
	}
}

// NewWithDirAndExecutor creates a new xcodeCleaner with explicit baseDir and executor for testing.
func NewWithDirAndExecutor(baseDir string, exec cleaner.CommandExecutor) domain.Cleaner {
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    runtime.GOOS,
	}
}

// NewWithDirExecutorAndOS creates a new xcodeCleaner allowing OS override for testing.
func NewWithDirExecutorAndOS(baseDir string, exec cleaner.CommandExecutor, goos string) domain.Cleaner {
	return &xcodeCleaner{
		exec:    exec,
		baseDir: baseDir,
		goos:    goos,
	}
}

func (c *xcodeCleaner) getOS() string {
	if c.goos != "" {
		return c.goos
	}
	return runtime.GOOS
}

// Name returns the identifier of the cleaner.
func (c *xcodeCleaner) Name() string {
	return "xcode"
}

// IsInstalled checks if running on Darwin and xcrun is available in PATH or DerivedData exists on disk.
func (c *xcodeCleaner) IsInstalled() bool {
	if c.getOS() != "darwin" || c.baseDir == "" {
		return false
	}
	if _, err := c.exec.LookPath("xcrun"); err == nil {
		return true
	}
	return devfs.DirExists(c.baseDir)
}


type xcodeTargetDef struct {
	category   string
	name       string
	path       string
	risk       domain.RiskLevel
	simRelated bool
	childOnly  bool
}

// checkProcessStatus returns running=true if pgrep finds matching processes.
// If pgrep fails with exit code 1, running=false, uncertain=false.
// If pgrep fails with any other error, uncertain=true (FAIL CLOSED!).
func checkProcessStatus(ctx context.Context, exec cleaner.CommandExecutor, processName string) (running bool, uncertain bool) {
	out, err := exec.Output(ctx, "pgrep", "-x", processName)
	if err == nil && len(strings.TrimSpace(string(out))) > 0 {
		return true, false
	}

	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, false // Cleanly not running
	}

	// Any other error (missing pgrep, permission denied, timeout, exit code > 1) is UNCERTAIN -> FAIL CLOSED!
	return false, true
}

func isSimulatorTarget(t domain.Target) bool {
	return t.Category == "xcode-coresimulator-caches" ||
		t.Category == "xcode-coresimulator-temp" ||
		t.Category == "virtual-xcode-simulators" ||
		t.Path == "xcode://simulators/unavailable"
}

func (c *xcodeCleaner) getExpandedTargetDefs() []xcodeTargetDef {
	if c.baseDir == "" {
		return nil
	}
	devDir := filepath.Dir(c.baseDir)
	if devfs.DirExists(filepath.Join(c.baseDir, "CoreSimulator")) {
		devDir = c.baseDir
	}
	libDir := filepath.Dir(devDir)

	return []xcodeTargetDef{
		{
			category:   "xcode-coresimulator-caches",
			name:       "CoreSimulator Caches",
			path:       filepath.Join(devDir, "CoreSimulator", "Caches"),
			risk:       domain.RiskLow,
			simRelated: true,
			childOnly:  false,
		},
		{
			category:   "xcode-coresimulator-temp",
			name:       "CoreSimulator Temp",
			path:       filepath.Join(devDir, "CoreSimulator", "Temp"),
			risk:       domain.RiskLow,
			simRelated: true,
			childOnly:  false,
		},
		{
			category:   "xcode-coresimulator-logs",
			name:       "CoreSimulator Logs",
			path:       filepath.Join(libDir, "Logs", "CoreSimulator"),
			risk:       domain.RiskLow,
			simRelated: false,
			childOnly:  true,
		},
		{
			category:   "xcode-app-cache",
			name:       "Xcode App Cache",
			path:       filepath.Join(libDir, "Caches", "com.apple.dt.Xcode"),
			risk:       domain.RiskLow,
			simRelated: false,
			childOnly:  false,
		},
		{
			category:   "xcode-ios-devicesupport",
			name:       "Xcode iOS DeviceSupport",
			path:       filepath.Join(c.baseDir, "iOS DeviceSupport"),
			risk:       domain.RiskLow,
			simRelated: false,
			childOnly:  false,
		},
		{
			category:   "xcode-watchos-devicesupport",
			name:       "Xcode watchOS DeviceSupport",
			path:       filepath.Join(c.baseDir, "watchOS DeviceSupport"),
			risk:       domain.RiskLow,
			simRelated: false,
			childOnly:  false,
		},
		{
			category:   "xcode-archives",
			name:       "Xcode Archives",
			path:       filepath.Join(c.baseDir, "Archives"),
			risk:       domain.RiskHigh,
			simRelated: false,
			childOnly:  false,
		},
	}
}

// Scan looks for individual project directories in DerivedData, expanded targets, and emits virtual simulator target.
func (c *xcodeCleaner) Scan(ctx context.Context, rules domain.EcosystemRules) ([]domain.Target, error) {
	if c.getOS() != "darwin" || c.baseDir == "" {
		return nil, nil
	}

	var targets []domain.Target
	cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

	// 1. DerivedData segmentado por projeto (D-09)
	derivedDataDir := filepath.Join(c.baseDir, "DerivedData")
	entries, err := os.ReadDir(derivedDataDir)
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "ModuleCache.noindex" {
				continue
			}

			projDir := filepath.Join(derivedDataDir, e.Name())

			if isExcluded(projDir, rules.ExcludePaths) {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}

			size, _ := devfs.CalcDirSize(projDir)
			if size > 0 {
				eligibleBytes := int64(0)
				ttlEligible := false
				if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
					eligibleBytes = size
					ttlEligible = true
				}

				sum := sha256.Sum256([]byte(projDir))
				targets = append(targets, domain.Target{
					ID:            fmt.Sprintf("%x", sum[:6]),
					Name:          "Xcode DerivedData: " + e.Name(),
					Path:          projDir,
					SizeBytes:     size,
					EligibleBytes: eligibleBytes,
					TTLEligible:   ttlEligible,
					Risk:          domain.RiskLow,
					LastAccessed:  info.ModTime(),
					Category:      "xcode-derived-data",
				})
			}
		}
	}

	// 2. Alvos expandidos (CoreSimulator Caches/Temp, DeviceSupport, Archives) (DEV-03, D-13, D-14)
	for _, def := range c.getExpandedTargetDefs() {
		if isExcluded(def.path, rules.ExcludePaths) {
			continue
		}
		info, err := os.Stat(def.path)
		if err != nil || !info.IsDir() {
			continue
		}
		size, _ := devfs.CalcDirSize(def.path)
		if size == 0 {
			continue
		}

		eligibleBytes := int64(0)
		ttlEligible := false
		if def.category == "xcode-coresimulator-logs" || def.category == "xcode-app-cache" {
			eligibleBytes = size
			ttlEligible = true
		} else if rules.MaxAgeDays == 0 || info.ModTime().Before(cutoff) {
			eligibleBytes = size
			ttlEligible = true
		}

		sum := sha256.Sum256([]byte(def.path))
		targets = append(targets, domain.Target{
			ID:            fmt.Sprintf("%x", sum[:6]),
			Name:          def.name,
			Path:          def.path,
			SizeBytes:     size,
			EligibleBytes: eligibleBytes,
			TTLEligible:   ttlEligible,
			Risk:          def.risk,
			LastAccessed:  info.ModTime(),
			Category:      def.category,
		})
	}

	// 3. Alvo virtual para simuladores iOS indisponíveis (D-10)
	targets = append(targets, domain.Target{
		ID:            "xcode-simulators-unavailable",
		Name:          "iOS Simulators (unavailable runtimes)",
		Path:          "xcode://simulators/unavailable",
		SizeBytes:     0,
		EligibleBytes: 0,
		TTLEligible:   true,
		Risk:          domain.RiskLow,
		Category:      "virtual-xcode-simulators",
	})

	// 4. Process guard: fail-closed evaluation for Xcode and Simulator (DEV-03, D-11, D-12)
	xcodeRunning, xcodeUncertain := checkProcessStatus(ctx, c.exec, "Xcode")
	simRunning, simUncertain := checkProcessStatus(ctx, c.exec, "Simulator")

	xcodeBlocked := xcodeRunning || xcodeUncertain
	simBlocked := simRunning || simUncertain

	for i := range targets {
		if targets[i].Category == "xcode-coresimulator-logs" {
			// Non-blocking: logs can be safely rotated and cleaned even while Xcode or Simulator runs (D-14)
			continue
		}

		if targets[i].Category == "xcode-app-cache" {
			if xcodeBlocked {
				targets[i].ReviewOnly = true
				targets[i].ReviewReason = domain.ReviewReasonActiveProcess
				targets[i].EligibleBytes = 0
				targets[i].TTLEligible = false
			}
			continue
		}

		if xcodeBlocked {
			targets[i].ReviewOnly = true
			targets[i].ReviewReason = domain.ReviewReasonActiveProcess
			targets[i].EligibleBytes = 0
			targets[i].TTLEligible = false
		} else if simBlocked && isSimulatorTarget(targets[i]) {
			targets[i].ReviewOnly = true
			targets[i].ReviewReason = domain.ReviewReasonActiveProcess
			targets[i].EligibleBytes = 0
			targets[i].TTLEligible = false
		}
	}

	return targets, nil
}

// Clean removes physical directories and triggers simctl for virtual targets.
func (c *xcodeCleaner) Clean(ctx context.Context, targets []domain.Target) error {
	for _, t := range targets {
		if t.Path == "xcode://simulators/unavailable" {
			if _, err := c.exec.Output(ctx, "xcrun", "simctl", "delete", "unavailable"); err != nil {
				return fmt.Errorf("failed to delete unavailable simulators: %w", err)
			}
			continue
		}

		if safety.IsCoreBlacklisted(t.Path) {
			continue
		}
		if isLink, err := safety.IsSymlink(t.Path); err == nil && isLink {
			continue
		}

		if t.Category == "xcode-coresimulator-logs" {
			entries, err := os.ReadDir(t.Path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			for _, entry := range entries {
				childPath := filepath.Join(t.Path, entry.Name())
				if safety.IsCoreBlacklisted(childPath) {
					continue
				}
				if isLink, err := safety.IsSymlink(childPath); err == nil && isLink {
					continue
				}
				_ = os.RemoveAll(childPath)
			}
			continue
		}

		if err := os.RemoveAll(t.Path); err != nil {
			return err
		}
	}
	return nil
}

func isExcluded(path string, excludes []string) bool {
	base := filepath.Base(path)
	for _, exc := range excludes {
		if strings.HasPrefix(path, exc) {
			return true
		}
		if matched, _ := doublestar.Match(exc, path); matched {
			return true
		}
		if matched, _ := doublestar.Match(exc, base); matched {
			return true
		}
	}
	return false
}
