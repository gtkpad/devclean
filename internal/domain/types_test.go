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
