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

func TestTarget_ReviewReasons(t *testing.T) {
	if domain.ReviewReasonActiveVersion != "active_version" {
		t.Errorf("ReviewReasonActiveVersion = %q, want %q", domain.ReviewReasonActiveVersion, "active_version")
	}
	if domain.ReviewReasonAmbiguous != "ambiguous_state" {
		t.Errorf("ReviewReasonAmbiguous = %q, want %q", domain.ReviewReasonAmbiguous, "ambiguous_state")
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

func TestTarget_ProviderMetrics_Serialization(t *testing.T) {
	metrics := &domain.ProviderMetrics{
		TotalCount:       15,
		ActiveCount:      3,
		SizeBytes:        10737418240, // 10 GB
		ReclaimableBytes: 5368709120,  // 5 GB
		ApparentBytes:    536870912000, // 500 GB
		AllocatedBytes:   5368709120,   // 5 GB
		Partial:          false,
		StatusNote:       "operational",
	}

	target := domain.Target{
		ID:              "docker-img-1",
		Name:            "Docker Images (15 total, 3 active)",
		Path:            "docker://system-df/images",
		SizeBytes:       metrics.SizeBytes,
		EligibleBytes:   0,
		TTLEligible:     false,
		ReviewOnly:      true,
		ReviewReason:    domain.ReviewReasonSharedStore,
		Risk:            domain.RiskHigh,
		LastAccessed:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Category:        "docker",
		ProviderMetrics: metrics,
	}

	// 1. Serialize target with non-nil ProviderMetrics
	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	pmRaw, ok := raw["provider_metrics"].(map[string]interface{})
	if !ok || pmRaw == nil {
		t.Fatalf("expected provider_metrics in JSON, got nil or wrong type: %v", raw["provider_metrics"])
	}
	if int(pmRaw["total_count"].(float64)) != 15 {
		t.Errorf("total_count = %v, want 15", pmRaw["total_count"])
	}
	if int(pmRaw["active_count"].(float64)) != 3 {
		t.Errorf("active_count = %v, want 3", pmRaw["active_count"])
	}
	if int64(pmRaw["size_bytes"].(float64)) != 10737418240 {
		t.Errorf("size_bytes = %v, want 10737418240", pmRaw["size_bytes"])
	}
	if int64(pmRaw["reclaimable_bytes"].(float64)) != 5368709120 {
		t.Errorf("reclaimable_bytes = %v, want 5368709120", pmRaw["reclaimable_bytes"])
	}
	if int64(pmRaw["apparent_bytes"].(float64)) != 536870912000 {
		t.Errorf("apparent_bytes = %v, want 536870912000", pmRaw["apparent_bytes"])
	}
	if int64(pmRaw["allocated_bytes"].(float64)) != 5368709120 {
		t.Errorf("allocated_bytes = %v, want 5368709120", pmRaw["allocated_bytes"])
	}
	if pmRaw["status_note"] != "operational" {
		t.Errorf("status_note = %v, want operational", pmRaw["status_note"])
	}

	// 2. Round-trip unmarshaling
	var decoded domain.Target
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal to Target failed: %v", err)
	}
	if decoded.ProviderMetrics == nil {
		t.Fatalf("decoded.ProviderMetrics is nil")
	}
	if *decoded.ProviderMetrics != *metrics {
		t.Errorf("decoded.ProviderMetrics = %+v, want %+v", *decoded.ProviderMetrics, *metrics)
	}

	// 3. Verify Target with nil ProviderMetrics omits "provider_metrics"
	targetNil := domain.Target{
		ID:   "clean-target-1",
		Path: "/some/path",
	}
	dataNil, err := json.Marshal(targetNil)
	if err != nil {
		t.Fatalf("Marshal targetNil failed: %v", err)
	}
	var rawNil map[string]interface{}
	if err := json.Unmarshal(dataNil, &rawNil); err != nil {
		t.Fatalf("Unmarshal rawNil failed: %v", err)
	}
	if _, exists := rawNil["provider_metrics"]; exists {
		t.Errorf("expected provider_metrics to be omitted when nil, got: %v", rawNil["provider_metrics"])
	}
}

