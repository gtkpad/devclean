package cleaner

import (
	"os"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

// ClassifyLocalArtifactRisk determines risk level based on manifest recency.
// manifestPaths are project manifest files (Cargo.toml, pom.xml, build.gradle, package.json, etc.)
// artifactPath is the build artifact directory being classified.
// maxAgeDays is the ecosystem's configured threshold.
func ClassifyLocalArtifactRisk(artifactPath string, manifestPaths []string, maxAgeDays int) domain.RiskLevel {
	artifactInfo, err := os.Stat(artifactPath)
	if err != nil {
		return domain.RiskLow
	}

	var mostRecentManifest time.Time
	manifestFound := false
	for _, mp := range manifestPaths {
		info, err := os.Stat(mp)
		if err != nil {
			continue
		}
		manifestFound = true
		if info.ModTime().After(mostRecentManifest) {
			mostRecentManifest = info.ModTime()
		}
	}

	if !manifestFound {
		return domain.RiskLow
	}

	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	artifactMtime := artifactInfo.ModTime()

	if artifactMtime.After(cutoff) || mostRecentManifest.After(cutoff) {
		return domain.RiskHigh
	}

	if artifactMtime.Before(cutoff) && mostRecentManifest.Before(cutoff) {
		return domain.RiskLow
	}

	return domain.RiskMedium
}

// IsLocalArtifactTTLEligible determines whether a local build artifact qualifies for TTL cleanup.
// When maxAgeDays == 0, returns true (D-06).
// When maxAgeDays > 0, returns true if both the artifact and all existing manifests were modified before cutoff (D-08).
func IsLocalArtifactTTLEligible(artifactPath string, manifestPaths []string, maxAgeDays int) bool {
	if maxAgeDays == 0 {
		return true
	}
	cutoff := time.Now().AddDate(0, 0, -maxAgeDays)
	info, err := os.Stat(artifactPath)
	if err != nil || info.ModTime().After(cutoff) {
		return false
	}
	for _, mp := range manifestPaths {
		if minfo, err := os.Stat(mp); err == nil {
			if minfo.ModTime().After(cutoff) {
				return false
			}
		}
	}
	return true
}

