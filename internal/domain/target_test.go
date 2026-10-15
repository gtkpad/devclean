package domain_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestTarget_ReviewOnly_Constants(t *testing.T) {
	tests := []struct {
		got  string
		want string
	}{
		{domain.ReviewReasonSharedStore, "shared_store"},
		{domain.ReviewReasonActiveProcess, "active_process"},
		{domain.ReviewReasonOrphanLeftover, "orphan_leftover"},
		{domain.ReviewReasonSystemProtected, "system_protected"},
		{domain.ReviewReasonSessionSensitive, "session_sensitive"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("ReviewReason constant = %q, want %q", tt.got, tt.want)
		}
	}
}

func TestTarget_ReviewOnly_IsReviewOnly(t *testing.T) {
	targetReview := domain.Target{
		ID:         "rev-1",
		ReviewOnly: true,
	}
	if !targetReview.IsReviewOnly() {
		t.Errorf("targetReview.IsReviewOnly() = false, want true")
	}

	targetNormal := domain.Target{
		ID:         "norm-1",
		ReviewOnly: false,
	}
	if targetNormal.IsReviewOnly() {
		t.Errorf("targetNormal.IsReviewOnly() = true, want false")
	}
}

func TestTarget_ReviewOnly_JSONSerialization(t *testing.T) {
	target := domain.Target{
		ID:            "rev-json-1",
		Name:          "docker-storage",
		Path:          "/var/lib/docker",
		SizeBytes:     5000000000,
		EligibleBytes: 0,
		TTLEligible:   false,
		ReviewOnly:    true,
		ReviewReason:  domain.ReviewReasonSharedStore,
		Risk:          domain.RiskHigh,
		LastAccessed:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Category:      "docker",
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if raw["review_only"] != true {
		t.Errorf("raw[review_only] = %v, want true", raw["review_only"])
	}
	if raw["review_reason"] != "shared_store" {
		t.Errorf("raw[review_reason] = %v, want shared_store", raw["review_reason"])
	}

	var decoded domain.Target
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal to Target failed: %v", err)
	}

	if !decoded.ReviewOnly {
		t.Errorf("decoded.ReviewOnly = false, want true")
	}
	if decoded.ReviewReason != domain.ReviewReasonSharedStore {
		t.Errorf("decoded.ReviewReason = %q, want %q", decoded.ReviewReason, domain.ReviewReasonSharedStore)
	}

	// Test omitempty on review_reason when empty
	emptyReasonTarget := domain.Target{
		ID:         "rev-empty-1",
		ReviewOnly: false,
	}
	emptyData, err := json.Marshal(emptyReasonTarget)
	if err != nil {
		t.Fatalf("Marshal empty reason target failed: %v", err)
	}

	var emptyRaw map[string]interface{}
	if err := json.Unmarshal(emptyData, &emptyRaw); err != nil {
		t.Fatalf("Unmarshal empty raw failed: %v", err)
	}
	if _, exists := emptyRaw["review_reason"]; exists {
		t.Errorf("review_reason should be omitted when empty, got %v", emptyRaw["review_reason"])
	}
}

func TestTarget_ReviewOnly_BackwardCompatibility(t *testing.T) {
	legacyJSON := `{
		"id": "legacy-review-1",
		"name": "cargo-target",
		"path": "/home/user/rust/target",
		"size_bytes": 1048576,
		"risk": "low",
		"category": "rust"
	}`

	var decoded domain.Target
	if err := json.Unmarshal([]byte(legacyJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ReviewOnly != false {
		t.Errorf("decoded.ReviewOnly = %v, want false", decoded.ReviewOnly)
	}
	if decoded.ReviewReason != "" {
		t.Errorf("decoded.ReviewReason = %q, want empty string", decoded.ReviewReason)
	}
}

func TestTarget_ReviewOnly_Invariants(t *testing.T) {
	// D-04: For any target with ReviewOnly = true, EligibleBytes must be 0 and TTLEligible = false
	reviewTargets := []domain.Target{
		{
			ID:           "docker-store",
			ReviewOnly:   true,
			ReviewReason: domain.ReviewReasonSharedStore,
			SizeBytes:    1024 * 1024 * 100,
		},
		{
			ID:           "active-simulator",
			ReviewOnly:   true,
			ReviewReason: domain.ReviewReasonActiveProcess,
			SizeBytes:    1024 * 1024 * 50,
		},
		{
			ID:           "orphan-agent",
			ReviewOnly:   true,
			ReviewReason: domain.ReviewReasonOrphanLeftover,
			SizeBytes:    1024 * 10,
		},
	}

	for _, rt := range reviewTargets {
		if !rt.IsReviewOnly() {
			t.Errorf("target %s: IsReviewOnly() = false, want true", rt.ID)
		}
		if rt.EligibleBytes != 0 {
			t.Errorf("target %s: EligibleBytes = %d, want 0", rt.ID, rt.EligibleBytes)
		}
		if rt.TTLEligible != false {
			t.Errorf("target %s: TTLEligible = %v, want false", rt.ID, rt.TTLEligible)
		}
	}
}
