package engine

import (
	"path/filepath"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
)

// EcosystemTier defines the precedence level of cleaners.
// Dedicated ecosystem cleaners (TierDedicated) take strict precedence over generic system/user cleaners (TierGeneric).
type EcosystemTier int

const (
	// TierGeneric represents broad or system-level cleaners (e.g. macos, system, browsers, leftovers).
	TierGeneric EcosystemTier = 1
	// TierDedicated represents specific technology/language cleaners (e.g. node, rust, python, bun, etc.).
	TierDedicated EcosystemTier = 2
)

// getEcosystemTier returns the tier for a given ecosystem name.
func getEcosystemTier(eco string) EcosystemTier {
	switch strings.ToLower(strings.TrimSpace(eco)) {
	case "macos", "system", "browsers", "leftovers":
		return TierGeneric
	default:
		return TierDedicated
	}
}

// isDescendant returns true if child is strictly inside parent.
func isDescendant(parent, child string) bool {
	if parent == "" || child == "" || parent == child {
		return false
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return !strings.HasPrefix(rel, "..") && rel != "."
}

type resolvedTarget struct {
	ecoIndex          int
	targetIndex       int
	target            *domain.Target
	realPath          string
	isVirtual         bool
	tier              EcosystemTier
	initSizeBytes     int64
	initEligibleBytes int64
	omitted           bool
}

// isDirectDescendant checks if child is a descendant of parent without any intermediate active target.
func isDirectDescendant(parent, child *resolvedTarget, active []*resolvedTarget) bool {
	if !isDescendant(parent.realPath, child.realPath) {
		return false
	}
	for _, mid := range active {
		if mid == parent || mid == child {
			continue
		}
		if isDescendant(parent.realPath, mid.realPath) && isDescendant(mid.realPath, child.realPath) {
			return false
		}
	}
	return true
}

// ResolveCanonicalOwnership implements canonical path resolution, precedence-based deduplication,
// and hierarchical byte deduction across scan results per D-01 to D-04 and SCAN-04.
//
// Rules enforced:
// 1. Filesystem paths are normalized and resolved via filepath.EvalSymlinks (D-02).
//    Circular symlinks or evaluation errors degrade gracefully to filepath.Clean (T-06-05).
// 2. Exact physical path collisions: TierDedicated cleaners take precedence over TierGeneric;
//    subordinate duplicates are omitted (D-01, D-04).
// 3. Hierarchical containment: when a target contains child targets, child bytes are deducted from
//    the parent with defensive clamping to prevent underflow (D-01, T-06-06).
// 4. Virtual targets (t.IsVirtual() == true) are completely immune to filesystem deduplication.
func ResolveCanonicalOwnership(results []domain.ScanResult) []domain.ScanResult {
	if len(results) == 0 {
		return results
	}

	// 1. Normalize all targets and resolve symlinks
	var allTargets []*resolvedTarget
	for rIdx := range results {
		tier := getEcosystemTier(results[rIdx].Ecosystem)
		for tIdx := range results[rIdx].Targets {
			t := &results[rIdx].Targets[tIdx]
			if t.IsVirtual() || strings.TrimSpace(t.Path) == "" {
				allTargets = append(allTargets, &resolvedTarget{
					ecoIndex:          rIdx,
					targetIndex:       tIdx,
					target:            t,
					realPath:          t.Path,
					isVirtual:         true,
					tier:              tier,
					initSizeBytes:     t.SizeBytes,
					initEligibleBytes: t.EligibleBytes,
				})
				continue
			}

			cleanPath := filepath.Clean(t.Path)
			if abs, err := filepath.Abs(cleanPath); err == nil {
				cleanPath = abs
			}

			realPath, err := filepath.EvalSymlinks(cleanPath)
			if err != nil {
				realPath = cleanPath
			}
			t.Path = realPath

			allTargets = append(allTargets, &resolvedTarget{
				ecoIndex:          rIdx,
				targetIndex:       tIdx,
				target:            t,
				realPath:          realPath,
				isVirtual:         false,
				tier:              tier,
				initSizeBytes:     t.SizeBytes,
				initEligibleBytes: t.EligibleBytes,
			})
		}
	}

	// 2. Resolve exact physical path collisions (D-01, D-04)
	seenPaths := make(map[string]*resolvedTarget)
	for _, rt := range allTargets {
		if rt.isVirtual {
			continue
		}
		if existing, exists := seenPaths[rt.realPath]; exists {
			if rt.tier > existing.tier {
				existing.omitted = true
				seenPaths[rt.realPath] = rt
			} else {
				rt.omitted = true
			}
		} else {
			seenPaths[rt.realPath] = rt
		}
	}

	// 3. Collect active non-omitted filesystem targets for hierarchical deduction
	var activeTargets []*resolvedTarget
	for _, rt := range allTargets {
		if !rt.isVirtual && !rt.omitted {
			activeTargets = append(activeTargets, rt)
		}
	}

	// 4. Hierarchical byte deduction from direct descendants with defensive clamping (D-01, T-06-06)
	for _, parent := range activeTargets {
		if parent.target.IsConsolidated || len(parent.target.SubPaths) > 0 {
			continue
		}
		for _, child := range activeTargets {
			if parent == child {
				continue
			}
			if isDirectDescendant(parent, child, activeTargets) {
				if parent.target.SizeBytes <= child.initSizeBytes {
					parent.target.SizeBytes = 0
				} else {
					parent.target.SizeBytes -= child.initSizeBytes
				}

				if parent.target.EligibleBytes <= child.initEligibleBytes {
					parent.target.EligibleBytes = 0
				} else {
					parent.target.EligibleBytes -= child.initEligibleBytes
				}
			}
		}
	}

	// 5. Reconstruct results slice omitting suppressed targets (D-04)
	filtered := make([]domain.ScanResult, len(results))
	for rIdx, r := range results {
		filtered[rIdx] = domain.ScanResult{
			Ecosystem: r.Ecosystem,
			Error:     r.Error,
			Targets:   make([]domain.Target, 0, len(r.Targets)),
		}
	}

	for _, rt := range allTargets {
		if !rt.omitted {
			filtered[rt.ecoIndex].Targets = append(filtered[rt.ecoIndex].Targets, *rt.target)
		}
	}

	return filtered
}
