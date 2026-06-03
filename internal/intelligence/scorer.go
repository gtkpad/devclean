package intelligence

import (
	"context"
	"time"

	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

// Weights holds the normalized weights for each scoring factor.
type Weights struct {
	Age         float64
	Size        float64
	GitActivity float64
	Risk        float64
}

// Score holds the total and per-factor scores for a target.
type Score struct {
	Total       float64
	Age         float64
	Size        float64
	GitActivity float64
	Risk        float64
}

// Scorer computes heuristic scores for scan targets.
type Scorer struct {
	Weights Weights
	MaxAge  time.Duration
	RefSize int64 // bytes
}

// NewScorer creates a Scorer from IntelligenceConfig.
func NewScorer(cfg config.IntelligenceConfig) *Scorer {
	return &Scorer{
		Weights: Weights{
			Age:         cfg.Weights.Age,
			Size:        cfg.Weights.Size,
			GitActivity: cfg.Weights.GitActivity,
			Risk:        cfg.Weights.Risk,
		},
		MaxAge:  time.Duration(cfg.MaxAgeDays) * 24 * time.Hour,
		RefSize: cfg.RefSizeMB * 1024 * 1024,
	}
}

// Score computes a weighted heuristic score for a target.
// Total is a weighted average in [0.0, 1.0].
func (s *Scorer) Score(ctx context.Context, target domain.Target) (Score, error) {
	ageS := AgeScore(target.LastAccessed, s.MaxAge)
	sizeS := SizeScore(target.SizeBytes, s.RefSize)
	riskS := RiskScore(target.Risk)

	gitS, err := GitActivityScore(ctx, target.Path, s.MaxAge)
	if err != nil {
		gitS = 0.0
	}

	total := s.Weights.Age*ageS +
		s.Weights.Size*sizeS +
		s.Weights.GitActivity*gitS +
		s.Weights.Risk*riskS

	return Score{
		Total:       clamp(total),
		Age:         ageS,
		Size:        sizeS,
		GitActivity: gitS,
		Risk:        riskS,
	}, nil
}
