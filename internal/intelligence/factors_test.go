package intelligence

import (
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestAgeScore(t *testing.T) {
	maxAge := 180 * 24 * time.Hour

	tests := []struct {
		name         string
		lastAccessed time.Time
		want         float64
	}{
		{"just accessed", time.Now(), 0.0},
		{"half max age", time.Now().Add(-90 * 24 * time.Hour), 0.5},
		{"at max age", time.Now().Add(-maxAge), 1.0},
		{"over max age clamped", time.Now().Add(-2 * maxAge), 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AgeScore(tt.lastAccessed, maxAge)
			if diff := got - tt.want; diff > 0.02 || diff < -0.02 {
				t.Errorf("AgeScore() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

func TestAgeScore_ZeroMaxAge(t *testing.T) {
	got := AgeScore(time.Now().Add(-24*time.Hour), 0)
	if got != 0.0 {
		t.Errorf("AgeScore with zero maxAge = %v, want 0.0", got)
	}
}

func TestSizeScore(t *testing.T) {
	refSize := int64(5 * 1024 * 1024 * 1024) // 5GB

	tests := []struct {
		name      string
		sizeBytes int64
		want      float64
	}{
		{"zero size", 0, 0.0},
		{"half ref", refSize / 2, 0.5},
		{"at ref", refSize, 1.0},
		{"over ref clamped", refSize * 2, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SizeScore(tt.sizeBytes, refSize)
			if diff := got - tt.want; diff > 0.01 || diff < -0.01 {
				t.Errorf("SizeScore() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

func TestSizeScore_ZeroRef(t *testing.T) {
	got := SizeScore(1000, 0)
	if got != 0.0 {
		t.Errorf("SizeScore with zero ref = %v, want 0.0", got)
	}
}

func TestRiskScore(t *testing.T) {
	tests := []struct {
		name  string
		level domain.RiskLevel
		want  float64
	}{
		{"low risk", domain.RiskLow, 1.0},
		{"medium risk", domain.RiskMedium, 0.5},
		{"high risk", domain.RiskHigh, 0.0},
		{"unknown defaults to medium", domain.RiskLevel("unknown"), 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RiskScore(tt.level)
			if got != tt.want {
				t.Errorf("RiskScore(%v) = %v, want %v", tt.level, got, tt.want)
			}
		})
	}
}
