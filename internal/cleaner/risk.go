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
