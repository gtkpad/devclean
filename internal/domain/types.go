package domain

import (
	"context"
	"strings"
	"time"
)

type RiskLevel string

const (
	RiskLow    RiskLevel = "low"
	RiskMedium RiskLevel = "medium"
	RiskHigh   RiskLevel = "high"
)

// Semantic Review Reasons (POL-01, D-02, D-10)
const (
	ReviewReasonSharedStore      = "shared_store"
	ReviewReasonActiveProcess    = "active_process"
	ReviewReasonOrphanLeftover   = "orphan_leftover"
	ReviewReasonSystemProtected  = "system_protected"
	ReviewReasonSessionSensitive = "session_sensitive"
	ReviewReasonActiveVersion    = "active_version"
	ReviewReasonAmbiguous        = "ambiguous_state"
)

type AIAssessment struct {
	ConfidenceScore float64 `json:"confidence_score"`
	Action          string  `json:"action"`
	Reasoning       string  `json:"reasoning"`
}

// ProviderMetrics contains structured metrics for container engines, virtual disks,
// and shared package stores (STORE-01, STORE-02, D-03).
type ProviderMetrics struct {
	TotalCount       int    `json:"total_count,omitempty"`
	ActiveCount      int    `json:"active_count,omitempty"`
	SizeBytes        int64  `json:"size_bytes,omitempty"`
	ReclaimableBytes int64  `json:"reclaimable_bytes,omitempty"`
	ApparentBytes    int64  `json:"apparent_bytes,omitempty"`
	AllocatedBytes   int64  `json:"allocated_bytes,omitempty"`
	Partial          bool   `json:"partial,omitempty"`
	StatusNote       string `json:"status_note,omitempty"`
}

type Target struct {
	ID              string           `json:"id"`
	Name            string           `json:"name"`
	Path            string           `json:"path"`
	SizeBytes       int64            `json:"size_bytes"`
	EligibleBytes   int64            `json:"eligible_bytes"`
	TTLEligible     bool             `json:"ttl_eligible"`
	ReviewOnly      bool             `json:"review_only"`
	ReviewReason    string           `json:"review_reason,omitempty"`
	Risk            RiskLevel        `json:"risk"`
	LastAccessed    time.Time        `json:"last_accessed"`
	Category        string           `json:"category"`
	AI              *AIAssessment    `json:"ai_assessment,omitempty"`
	SmartScore      float64          `json:"smart_score,omitempty"`
	ProviderMetrics *ProviderMetrics `json:"provider_metrics,omitempty"`
}

// IsReviewOnly reports whether the target is restricted to inventory inspection only.
func (t Target) IsReviewOnly() bool {
	return t.ReviewOnly
}

// IsVirtual returns true if the target represents a virtual resource (e.g. Docker images,
// volumes, build cache) that cannot be manipulated via standard filesystem operations.
func (t Target) IsVirtual() bool {
	return strings.HasPrefix(t.Path, "docker://") || strings.Contains(t.Path, "://")
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
