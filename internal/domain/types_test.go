package domain

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRiskLevel_Values(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskLow, "low"},
		{RiskMedium, "medium"},
		{RiskHigh, "high"},
	}
	for _, tt := range tests {
		if string(tt.level) != tt.want {
			t.Errorf("RiskLevel = %q, want %q", tt.level, tt.want)
		}
	}
}

func TestTarget_JSONSerialization_WithAI(t *testing.T) {
	target := Target{
		ID:           "test-1",
		Name:         "node_modules",
		Path:         "/home/user/project/node_modules",
		SizeBytes:    1024000,
		Risk:         RiskMedium,
		LastAccessed: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Category:     "node",
		AI: &AIAssessment{
			ConfidenceScore: 0.95,
			Action:          "SAFE_TO_DELETE",
			Reasoning:       "No recent git activity",
		},
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded Target
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != target.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, target.ID)
	}
	if decoded.Risk != RiskMedium {
		t.Errorf("Risk = %q, want %q", decoded.Risk, RiskMedium)
	}
	if decoded.AI.ConfidenceScore != 0.95 {
		t.Errorf("AI.ConfidenceScore = %f, want 0.95", decoded.AI.ConfidenceScore)
	}
}

func TestTarget_JSONSerialization_WithoutAI(t *testing.T) {
	target := Target{
		ID:        "test-2",
		Name:      "go-build",
		Path:      "/home/user/.cache/go-build",
		SizeBytes: 512000,
		Risk:      RiskLow,
		Category:  "go",
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// omitempty should exclude ai_assessment when zero-value
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if _, exists := raw["ai_assessment"]; exists {
		t.Error("ai_assessment should be omitted when zero-value")
	}
}

func TestScanResult_Aggregation(t *testing.T) {
	results := []ScanResult{
		{Ecosystem: "docker", Targets: []Target{{ID: "d1"}, {ID: "d2"}}},
		{Ecosystem: "go", Targets: []Target{{ID: "g1"}}},
		{Ecosystem: "node", Error: errScanFailed},
	}

	var totalTargets int
	var failedEcosystems int
	for _, r := range results {
		if r.Error != nil {
			failedEcosystems++
			continue
		}
		totalTargets += len(r.Targets)
	}

	if totalTargets != 3 {
		t.Errorf("totalTargets = %d, want 3", totalTargets)
	}
	if failedEcosystems != 1 {
		t.Errorf("failedEcosystems = %d, want 1", failedEcosystems)
	}
}

var errScanFailed = errorString("scan failed")

type errorString string

func (e errorString) Error() string { return string(e) }

func TestTarget_IsVirtual(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"docker://sha256:abc", true},
		{"docker://volume/my-vol", true},
		{"custom://artifact", true},
		{"/home/user/project/node_modules", false},
		{"/Users/user/.cache/go-build", false},
		{"", false},
	}

	for _, tc := range tests {
		target := Target{Path: tc.path}
		if got := target.IsVirtual(); got != tc.want {
			t.Errorf("Target{Path: %q}.IsVirtual() = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestTarget_JSONSerialization(t *testing.T) {
	target := Target{
		ID:            "test-ttl-1",
		Name:          "node_modules",
		Path:          "/home/user/project/node_modules",
		SizeBytes:     1024000,
		EligibleBytes: 512000,
		TTLEligible:   true,
		Risk:          RiskMedium,
		LastAccessed:  time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		Category:      "node",
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if raw["eligible_bytes"] != float64(512000) {
		t.Errorf("raw[eligible_bytes] = %v, want 512000", raw["eligible_bytes"])
	}
	if raw["ttl_eligible"] != true {
		t.Errorf("raw[ttl_eligible] = %v, want true", raw["ttl_eligible"])
	}

	var decoded Target
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal to Target failed: %v", err)
	}

	if decoded.EligibleBytes != 512000 {
		t.Errorf("decoded.EligibleBytes = %d, want 512000", decoded.EligibleBytes)
	}
	if !decoded.TTLEligible {
		t.Errorf("decoded.TTLEligible = %v, want true", decoded.TTLEligible)
	}
}

func TestTarget_BackwardCompatibility(t *testing.T) {
	// Old JSON payload without eligible_bytes or ttl_eligible
	oldJSON := `{
		"id": "legacy-1",
		"name": "cargo-target",
		"path": "/home/user/rust/target",
		"size_bytes": 2048000,
		"risk": "low",
		"category": "rust"
	}`

	var decoded Target
	if err := json.Unmarshal([]byte(oldJSON), &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.ID != "legacy-1" {
		t.Errorf("decoded.ID = %q, want legacy-1", decoded.ID)
	}
	if decoded.SizeBytes != 2048000 {
		t.Errorf("decoded.SizeBytes = %d, want 2048000", decoded.SizeBytes)
	}
	if decoded.EligibleBytes != 0 {
		t.Errorf("decoded.EligibleBytes = %d, want 0", decoded.EligibleBytes)
	}
	if decoded.TTLEligible != false {
		t.Errorf("decoded.TTLEligible = %v, want false", decoded.TTLEligible)
	}
	if decoded.IsConsolidated != false {
		t.Errorf("decoded.IsConsolidated = %v, want false", decoded.IsConsolidated)
	}
	if len(decoded.SubPaths) != 0 {
		t.Errorf("decoded.SubPaths = %v, want nil/empty", decoded.SubPaths)
	}
}

func TestTarget_ConsolidatedFields(t *testing.T) {
	target := Target{
		ID:             "macos-caches-1",
		Name:           "macOS User Caches",
		Path:           "/Users/test/Library/Caches",
		SizeBytes:      68 * 1024 * 1024,
		EligibleBytes:  68 * 1024 * 1024,
		TTLEligible:    true,
		IsConsolidated: true,
		SubPaths: []string{
			"/Users/test/Library/Caches/com.apple.Safari",
			"/Users/test/Library/Caches/com.apple.finder",
		},
	}

	data, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if raw["is_consolidated"] != true {
		t.Errorf("raw is_consolidated = %v, want true", raw["is_consolidated"])
	}
	subPaths, ok := raw["sub_paths"].([]interface{})
	if !ok || len(subPaths) != 2 {
		t.Errorf("raw sub_paths = %v, want 2 items", raw["sub_paths"])
	}

	var decoded Target
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal to Target failed: %v", err)
	}

	if !decoded.IsConsolidated {
		t.Errorf("decoded.IsConsolidated = false, want true")
	}
	if len(decoded.SubPaths) != 2 || decoded.SubPaths[0] != target.SubPaths[0] {
		t.Errorf("decoded.SubPaths = %v, want %v", decoded.SubPaths, target.SubPaths)
	}
}

