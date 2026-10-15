package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/domain"
)

func TestHumanSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
		{1536, "1.5 KB"},
	}

	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			got := humanSize(tc.bytes)
			if got != tc.want {
				t.Errorf("humanSize(%d) = %q, want %q", tc.bytes, got, tc.want)
			}
		})
	}
}

func TestFormatTable_GroupedByEcosystem(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{Path: "/home/user/project/node_modules", SizeBytes: 1048576, Risk: domain.RiskLow, LastAccessed: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)},
			},
		},
		{
			Ecosystem: "go",
			Targets: []domain.Target{
				{Path: "/home/user/.cache/go-build", SizeBytes: 2097152, Risk: domain.RiskLow, LastAccessed: time.Date(2025, 2, 20, 0, 0, 0, 0, time.UTC)},
				{Path: "/home/user/go/pkg/mod", SizeBytes: 5368709120, Risk: domain.RiskMedium, LastAccessed: time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)},
			},
		},
	}

	var buf bytes.Buffer
	formatTable(&buf, results, false, false)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("ECOSYSTEM")) {
		t.Error("table output missing ECOSYSTEM header")
	}
	if !bytes.Contains([]byte(output), []byte("node")) {
		t.Error("table output missing node ecosystem")
	}
	if !bytes.Contains([]byte(output), []byte("go")) {
		t.Error("table output missing go ecosystem")
	}
}

func TestFormatTable_EmptyResults(t *testing.T) {
	var buf bytes.Buffer
	formatTable(&buf, nil, false, false)
	output := buf.String()

	if output != "No targets found\n" {
		t.Errorf("expected 'No targets found', got %q", output)
	}
}

func TestFormatJSON_ValidOutput(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "docker",
			Targets: []domain.Target{
				{ID: "d1", Name: "builder cache", Path: "/var/lib/docker/buildkit", SizeBytes: 1073741824, Risk: domain.RiskLow, LastAccessed: time.Now()},
			},
		},
	}

	var buf bytes.Buffer
	err := formatJSON(&buf, results)
	if err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}

	if !json.Valid(buf.Bytes()) {
		t.Error("formatJSON produced invalid JSON")
	}

	var parsed []jsonEcosystem
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("cannot unmarshal JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Ecosystem != "docker" {
		t.Errorf("unexpected JSON structure: %+v", parsed)
	}
}

func TestFormatOutput_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := formatOutput(&buf, nil, "xml", false, false)
	if err == nil {
		t.Error("expected error for unknown format 'xml'")
	}
}

func TestFormat_ErrorAndVerboseBranches(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "error-eco",
			Error:     errors.New("scan failed"),
		},
		{
			Ecosystem: "empty-eco",
			Targets:   nil,
		},
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:           "t1",
					Name:         "long-path-target",
					Path:         "/this/is/a/very/long/path/that/exceeds/fifty/characters/in/total/length/node_modules",
					SizeBytes:    1024,
					Risk:         domain.RiskLow,
					LastAccessed: time.Now(),
				},
			},
		},
	}

	var buf bytes.Buffer
	formatTable(&buf, results, true, false)

	var jsonBuf bytes.Buffer
	if err := formatJSON(&jsonBuf, results); err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}
}

func TestFormatTable_Verbose_PartialEligibility(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:            "part1",
					Name:          "node_modules",
					Path:          "/project/node_modules",
					SizeBytes:     10000,
					EligibleBytes: 5000,
					TTLEligible:   false,
					Risk:          domain.RiskLow,
					LastAccessed:  time.Now(),
				},
			},
		},
	}

	var buf bytes.Buffer
	formatTable(&buf, results, true, false)
	output := buf.String()

	if !strings.Contains(output, "eligible:") || !strings.Contains(output, "ttl:") {
		t.Errorf("expected verbose table to display partial eligibility info, got: %s", output)
	}
}

func TestFormatJSON_IncludesEligibleFields(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "rust",
			Targets: []domain.Target{
				{
					ID:            "r1",
					Name:          "target",
					Path:          "/project/target",
					SizeBytes:     50000,
					EligibleBytes: 50000,
					TTLEligible:   true,
					Risk:          domain.RiskLow,
					LastAccessed:  time.Now(),
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := formatJSON(&buf, results); err != nil {
		t.Fatalf("formatJSON error: %v", err)
	}

	var parsed []struct {
		Ecosystem string `json:"ecosystem"`
		Targets   []struct {
			EligibleBytes int64 `json:"eligible_bytes"`
			TTLEligible   bool  `json:"ttl_eligible"`
		} `json:"targets"`
	}

	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if len(parsed) != 1 || len(parsed[0].Targets) != 1 {
		t.Fatalf("unexpected structure: %+v", parsed)
	}

	target := parsed[0].Targets[0]
	if target.EligibleBytes != 50000 {
		t.Errorf("expected eligible_bytes=50000, got %d", target.EligibleBytes)
	}
	if !target.TTLEligible {
		t.Errorf("expected ttl_eligible=true, got false")
	}
}

func TestFormatTable_ReviewOnly(t *testing.T) {
	results := []domain.ScanResult{
		{
			Ecosystem: "docker",
			Targets: []domain.Target{
				{
					ID:           "doc1",
					Name:         "docker-storage",
					Path:         "/var/lib/docker",
					SizeBytes:    10737418240,
					Risk:         domain.RiskHigh,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonSharedStore,
					LastAccessed: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				},
				{
					ID:           "doc2",
					Name:         "docker-cache",
					Path:         "/var/lib/docker/buildkit",
					SizeBytes:    1048576,
					Risk:         domain.RiskLow,
					ReviewOnly:   false,
					LastAccessed: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				},
			},
		},
	}

	// 1. Standard table output: ReviewOnly path must be prefixed with "⊙ " (POL-03, D-09)
	var buf bytes.Buffer
	formatTable(&buf, results, false, false)
	output := buf.String()

	if !strings.Contains(output, "⊙ /var/lib/docker") {
		t.Errorf("expected standard table to prefix ReviewOnly target path with '⊙ ', got:\n%s", output)
	}
	if strings.Contains(output, "⊙ /var/lib/docker/buildkit") {
		t.Errorf("expected normal target path NOT to be prefixed with '⊙ ', got:\n%s", output)
	}

	// 2. Verbose table output: ReviewOnly target must display [ReviewOnly: <reason>] (eligible: 0 B) (POL-03, D-09)
	var verboseBuf bytes.Buffer
	formatTable(&verboseBuf, results, true, false)
	verboseOutput := verboseBuf.String()

	if !strings.Contains(verboseOutput, "⊙ /var/lib/docker") {
		t.Errorf("expected verbose table to prefix ReviewOnly target path with '⊙ ', got:\n%s", verboseOutput)
	}
	expectedVerboseTag := fmt.Sprintf("[ReviewOnly: %s] (eligible: 0 B)", domain.ReviewReasonSharedStore)
	if !strings.Contains(verboseOutput, expectedVerboseTag) {
		t.Errorf("expected verbose table to contain %q, got:\n%s", expectedVerboseTag, verboseOutput)
	}

	// 3. Long path ReviewOnly target: marker '⊙ ' must remain visible after truncation
	longResult := []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{
					ID:           "long1",
					Name:         "long-pnpm-store",
					Path:         "/Users/developer/Library/pnpm/store/v3/files/very/deep/nested/path/to/cache",
					SizeBytes:    524288000,
					Risk:         domain.RiskLow,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonSharedStore,
					LastAccessed: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
				},
			},
		},
	}
	var longBuf bytes.Buffer
	formatTable(&longBuf, longResult, false, false)
	longOutput := longBuf.String()
	if !strings.Contains(longOutput, "⊙ ") {
		t.Errorf("expected long ReviewOnly path to retain '⊙ ' marker, got:\n%s", longOutput)
	}
}



