package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/gabrielvp/devclean/internal/domain"
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
	formatTable(&buf, results, false)
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
	formatTable(&buf, nil, false)
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
	err := formatOutput(&buf, nil, "xml", false)
	if err == nil {
		t.Error("expected error for unknown format 'xml'")
	}
}
