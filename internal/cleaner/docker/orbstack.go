package docker

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtkpad/devclean/internal/cleaner"
	"github.com/gtkpad/devclean/internal/domain"
	devfs "github.com/gtkpad/devclean/internal/fs"
	"github.com/gtkpad/devclean/internal/safety"
)

// checkOrbStackProcess queries whether OrbStack is currently running (D-08).
// Exit 0 with output -> running=true, uncertain=false.
// Exit 1 -> running=false, uncertain=false (cleanly stopped).
// Any other exit code, error, or empty output on exit 0 -> uncertain=true (fail closed).
func checkOrbStackProcess(ctx context.Context, exec cleaner.CommandExecutor) (running bool, uncertain bool) {
	if exec == nil {
		return false, true
	}

	out, err := exec.Output(ctx, "pgrep", "-x", "OrbStack")
	if err == nil {
		if len(strings.TrimSpace(string(out))) > 0 {
			return true, false
		}
		// Exit 0 with empty output is unexpected / ambiguous
		return false, true
	}

	var exitErr interface{ ExitCode() int }
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return false, false // Cleanly stopped
	}

	// Unexpected error or tool failure: fail closed
	return false, true
}

// scanOrbStack discovers regenerable caches and persistent VM/Data storage for OrbStack (STORE-02, D-05, D-06, D-07, D-08).
func scanOrbStack(ctx context.Context, exec cleaner.CommandExecutor, rules domain.EcosystemRules) ([]domain.Target, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil, nil
	}

	var targets []domain.Target

	// 1. Inspect regenerable cache candidates
	cacheCandidates := []string{
		filepath.Join(home, "Library", "Caches", "dev.orbstack.OrbStack"),
		filepath.Join(home, ".cache", "orbstack"),
	}

	var existingCaches []struct {
		path string
		info os.FileInfo
	}
	for _, cPath := range cacheCandidates {
		if info, err := os.Stat(cPath); err == nil {
			existingCaches = append(existingCaches, struct {
				path string
				info os.FileInfo
			}{path: cPath, info: info})
		}
	}

	if len(existingCaches) > 0 {
		running, uncertain := checkOrbStackProcess(ctx, exec)
		isProcessBlocked := running || uncertain
		cutoff := time.Now().AddDate(0, 0, -rules.MaxAgeDays)

		for _, c := range existingCaches {
			size, _ := devfs.CalcDirSize(c.path)
			if !c.info.IsDir() {
				size = c.info.Size()
			}

			sum := sha256.Sum256([]byte(c.path))
			id := fmt.Sprintf("orbstack-cache-%x", sum[:4])

			target := domain.Target{
				ID:           id,
				Name:         "OrbStack Cache",
				Path:         c.path,
				SizeBytes:    size,
				Risk:         domain.RiskLow,
				LastAccessed: c.info.ModTime(),
				Category:     "orbstack-cache",
			}

			if isProcessBlocked {
				target.ReviewOnly = true
				if running {
					target.ReviewReason = domain.ReviewReasonActiveProcess
				} else {
					target.ReviewReason = domain.ReviewReasonAmbiguous
				}
				target.EligibleBytes = 0
				target.TTLEligible = false
			} else if rules.MaxAgeDays == 0 || c.info.ModTime().Before(cutoff) {
				target.TTLEligible = true
				target.EligibleBytes = size
			}

			targets = append(targets, target)
		}
	}

	// 2. Inspect VM/Data storage candidates (D-06, D-07)
	var vmCandidates []string
	groupMatches, _ := filepath.Glob(filepath.Join(home, "Library", "Group Containers", "*dev.orbstack", "data"))
	vmCandidates = append(vmCandidates, groupMatches...)
	vmCandidates = append(vmCandidates,
		filepath.Join(home, "Library", "Group Containers", "group.dev.orbstack", "data"),
		filepath.Join(home, ".orbstack", "data"),
		filepath.Join(home, ".orbstack"),
	)

	seen := make(map[string]bool)
	for _, vPath := range vmCandidates {
		vPath = filepath.Clean(vPath)
		if seen[vPath] {
			continue
		}
		seen[vPath] = true

		info, err := os.Stat(vPath)
		if err != nil {
			continue
		}

		usage, _ := devfs.CalcUsage(vPath)
		size := usage.AllocatedBytes
		if size == 0 {
			size = info.Size()
		}
		apparent := usage.ApparentBytes
		if apparent == 0 {
			apparent = info.Size()
		}

		name := fmt.Sprintf("OrbStack VM/Data (allocated: %s, apparent: %s)", formatBytes(size), formatBytes(apparent))
		sum := sha256.Sum256([]byte(vPath))
		id := fmt.Sprintf("orbstack-data-%x", sum[:4])

		target := domain.Target{
			ID:            id,
			Name:          name,
			Path:          vPath,
			SizeBytes:     size,
			EligibleBytes: 0, // D-06: 100% ReviewOnly
			TTLEligible:   false,
			ReviewOnly:    true,
			ReviewReason:  domain.ReviewReasonSharedStore,
			Risk:          domain.RiskLow,
			LastAccessed:  info.ModTime(),
			Category:      "orbstack-data",
			ProviderMetrics: &domain.ProviderMetrics{
				SizeBytes:      size,
				AllocatedBytes: usage.AllocatedBytes,
				ApparentBytes:  apparent,
				Partial:        usage.Partial,
				StatusNote:     "OrbStack virtual machines and persistent container storage",
			},
		}

		targets = append(targets, target)
		break // Break after first matching VM data root to prevent double-counting nested data paths
	}

	return targets, nil
}

// cleanOrbStackCache removes regenerable OrbStack cache after pre-deletion revalidation (D-08, D-12).
func cleanOrbStackCache(ctx context.Context, exec cleaner.CommandExecutor, t domain.Target) error {
	running, uncertain := checkOrbStackProcess(ctx, exec)
	if running || uncertain {
		return nil // Abort deletion if process is running or state is uncertain
	}
	if safety.IsCoreBlacklisted(t.Path) {
		return nil
	}
	return os.RemoveAll(t.Path)
}
