package domain

import (
	"context"
	"time"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

type AIAssessment struct {
	ConfidenceScore float64 `json:"confidence_score"`
	Action          string  `json:"action"`
	Reasoning       string  `json:"reasoning"`
}

type Target struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	Path         string        `json:"path"`
	SizeBytes    int64         `json:"size_bytes"`
	Risk         RiskLevel     `json:"risk"`
	LastAccessed time.Time     `json:"last_accessed"`
	Category     string        `json:"category"`
	AI           *AIAssessment `json:"ai_assessment,omitempty"`
	SmartScore   float64       `json:"smart_score,omitempty"`
}

type ScanResult struct {
	Ecosystem string
	Targets   []Target
	Error     error
}

type EcosystemRules struct {
	Enabled      bool
	MaxAgeDays   int
	FailMode     string
	ExcludePaths []string
}

type Cleaner interface {
	Name() string
	IsInstalled() bool
	Scan(ctx context.Context, rules EcosystemRules) ([]Target, error)
	Clean(ctx context.Context, targets []Target) error
}
