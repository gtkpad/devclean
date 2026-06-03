package intelligence

import (
	"context"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestNewScorer(t *testing.T) {
	cfg := config.IntelligenceConfig{
		Threshold:  0.7,
		MaxAgeDays: 180,
		RefSizeMB:  5120,
		Weights: config.WeightsConfig{
			Age:         0.35,
			Size:        0.30,
			GitActivity: 0.20,
			Risk:        0.15,
		},
	}

	s := NewScorer(cfg)
	if s.MaxAge != 180*24*time.Hour {
		t.Errorf("MaxAge = %v, want %v", s.MaxAge, 180*24*time.Hour)
	}
	if s.RefSize != 5120*1024*1024 {
		t.Errorf("RefSize = %v, want %v", s.RefSize, int64(5120*1024*1024))
	}
}

func TestScorer_Score_KnownInputs(t *testing.T) {
	cfg := config.IntelligenceConfig{
		Threshold:  0.7,
		MaxAgeDays: 180,
		RefSizeMB:  5120,
		Weights: config.WeightsConfig{
			Age:         0.35,
			Size:        0.30,
			GitActivity: 0.20,
			Risk:        0.15,
		},
	}

	s := NewScorer(cfg)

	// Target: 90 days old (age=0.5), 2560MB (size=0.5), low risk (risk=1.0)
	// Git activity is non-deterministic in tests, so we use a non-git path
	tmp := t.TempDir()
	target := domain.Target{
		Path:         tmp,
		SizeBytes:    2560 * 1024 * 1024,
		Risk:         domain.RiskLow,
		LastAccessed: time.Now().Add(-90 * 24 * time.Hour),
	}

	score, err := s.Score(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}

	// Expected: 0.35*0.5 + 0.30*0.5 + 0.20*0.0 + 0.15*1.0 = 0.175 + 0.15 + 0 + 0.15 = 0.475
	expectedTotal := 0.35*0.5 + 0.30*0.5 + 0.20*0.0 + 0.15*1.0
	if diff := score.Total - expectedTotal; diff > 0.02 || diff < -0.02 {
		t.Errorf("Score.Total = %v, want ~%v", score.Total, expectedTotal)
	}

	if diff := score.Age - 0.5; diff > 0.02 || diff < -0.02 {
		t.Errorf("Score.Age = %v, want ~0.5", score.Age)
	}
	if diff := score.Size - 0.5; diff > 0.02 || diff < -0.02 {
		t.Errorf("Score.Size = %v, want ~0.5", score.Size)
	}
	if score.Risk != 1.0 {
		t.Errorf("Score.Risk = %v, want 1.0", score.Risk)
	}
}

func TestScorer_Score_HighRiskLowScore(t *testing.T) {
	cfg := config.IntelligenceConfig{
		Threshold:  0.7,
		MaxAgeDays: 180,
		RefSizeMB:  5120,
		Weights: config.WeightsConfig{
			Age:         0.35,
			Size:        0.30,
			GitActivity: 0.20,
			Risk:        0.15,
		},
	}

	s := NewScorer(cfg)
	tmp := t.TempDir()

	// High risk, recently accessed, small — should score very low
	target := domain.Target{
		Path:         tmp,
		SizeBytes:    100 * 1024 * 1024, // 100MB
		Risk:         domain.RiskHigh,
		LastAccessed: time.Now().Add(-1 * 24 * time.Hour), // 1 day ago
	}

	score, err := s.Score(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}

	if score.Total > 0.3 {
		t.Errorf("High risk recent target should score low, got %v", score.Total)
	}
}
