package intelligence

import (
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

// AgeScore returns a 0.0-1.0 score where older targets score higher.
// Linear normalization: age/maxAge, clamped to [0.0, 1.0].
func AgeScore(lastAccessed time.Time, maxAge time.Duration) float64 {
	if maxAge <= 0 {
		return 0.0
	}
	age := time.Since(lastAccessed)
	score := float64(age) / float64(maxAge)
	return clamp(score)
}

// SizeScore returns a 0.0-1.0 score where larger targets score higher.
// Linear normalization: sizeBytes/refSize, clamped to [0.0, 1.0].
func SizeScore(sizeBytes, refSize int64) float64 {
	if refSize <= 0 {
		return 0.0
	}
	score := float64(sizeBytes) / float64(refSize)
	return clamp(score)
}

// RiskScore returns a 0.0-1.0 score based on risk level.
// Low risk = 1.0 (safe to delete), Medium = 0.5, High = 0.0 (risky to delete).
func RiskScore(level domain.RiskLevel) float64 {
	switch level {
	case domain.RiskLow:
		return 1.0
	case domain.RiskMedium:
		return 0.5
	case domain.RiskHigh:
		return 0.0
	default:
		return 0.5
	}
}

func clamp(v float64) float64 {
	if v < 0.0 {
		return 0.0
	}
	if v > 1.0 {
		return 1.0
	}
	return v
}
