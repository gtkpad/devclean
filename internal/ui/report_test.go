package ui

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/gtkpad/devclean/internal/fs"
)

func TestTracer_StatfsToReport(t *testing.T) {
	space, err := fs.GetDiskSpace(".")
	if err != nil {
		t.Fatalf("fs.GetDiskSpace(\".\") failed: %v", err)
	}

	if space.TotalBytes == 0 {
		t.Errorf("expected non-zero TotalBytes, got %d", space.TotalBytes)
	}
	if space.FreeBytes == 0 {
		t.Errorf("expected non-zero FreeBytes, got %d", space.FreeBytes)
	}

	diskBefore := space.FreeBytes
	diskAfter := space.FreeBytes + 104857600 // Simula 100 MB liberados

	data := ReportData{
		Title:          "DevClean Summary",
		TotalFreed:     104857600,
		TotalCount:     5,
		TrashCount:     3,
		PermanentCount: 2,
		Duration:       150 * time.Millisecond,
		DiskBefore:     &diskBefore,
		DiskAfter:      &diskAfter,
		Ecosystems: []EcosystemMetric{
			{Name: "node", FreedBytes: 73400320, Count: 3},
			{Name: "python", FreedBytes: 31457280, Count: 2},
		},
	}

	var buf bytes.Buffer
	RenderReport(&buf, data)
	output := buf.String()

	if !strings.Contains(output, "DevClean Summary") {
		t.Errorf("report output missing title 'DevClean Summary', got:\n%s", output)
	}
	if !strings.Contains(output, "100.0 MB") {
		t.Errorf("report output missing formatted freed size '100.0 MB', got:\n%s", output)
	}
	if !strings.Contains(output, "150 ms") {
		t.Errorf("report output missing duration '150 ms', got:\n%s", output)
	}
	if !strings.Contains(output, "Lixeira: 3, Permanente: 2") {
		t.Errorf("report output missing trash vs permanent counts, got:\n%s", output)
	}
	if !strings.Contains(output, "Volume em Disco:") {
		t.Errorf("report output missing disk volume line, got:\n%s", output)
	}
	if !strings.Contains(output, "Detalhamento por Ecossistema:") {
		t.Errorf("report output missing ecosystem breakdown, got:\n%s", output)
	}
	if !strings.Contains(output, "node") || !strings.Contains(output, "python") {
		t.Errorf("report output missing ecosystem names, got:\n%s", output)
	}
}

func TestReport_CardContentAndFormatting(t *testing.T) {
	diskBefore := uint64(500 * 1024 * 1024 * 1024)  // 500 GB
	diskAfter := uint64(510 * 1024 * 1024 * 1024)   // 510 GB (+10 GB)

	data := ReportData{
		Title:          "Clean Card",
		TotalFreed:     10 * 1024 * 1024 * 1024,
		TotalCount:     42,
		TrashCount:     30,
		PermanentCount: 12,
		Duration:       234 * time.Millisecond,
		DiskBefore:     &diskBefore,
		DiskAfter:      &diskAfter,
		Ecosystems: []EcosystemMetric{
			{Name: "node", FreedBytes: 6 * 1024 * 1024 * 1024, Count: 20},
			{Name: "rust", FreedBytes: 4 * 1024 * 1024 * 1024, Count: 22},
		},
	}

	var buf bytes.Buffer
	RenderReport(&buf, data)
	out := buf.String()

	if !strings.Contains(out, "✨ Clean Card") {
		t.Errorf("expected sparkle and title, got:\n%s", out)
	}
	if !strings.Contains(out, "Espaço Recuperado:") || !strings.Contains(out, "10.0 GB") {
		t.Errorf("expected recovered space 10.0 GB, got:\n%s", out)
	}
	if !strings.Contains(out, "Duração Total:") || !strings.Contains(out, "234 ms") {
		t.Errorf("expected duration 234 ms, got:\n%s", out)
	}
	if !strings.Contains(out, "42 (Lixeira: 30, Permanente: 12)") {
		t.Errorf("expected target counters, got:\n%s", out)
	}
	if !strings.Contains(out, "500.0 GB ➔ 510.0 GB (+10.0 GB)") {
		t.Errorf("expected disk delta line, got:\n%s", out)
	}
}

func TestReport_MiniBarsOrderingAndProportions(t *testing.T) {
	total := int64(1000)
	data := ReportData{
		Title:      "Proportion Test",
		TotalFreed: total,
		TotalCount: 3,
		Duration:   50 * time.Millisecond,
		Ecosystems: []EcosystemMetric{
			{Name: "python", FreedBytes: 100, Count: 1}, // 10%
			{Name: "node", FreedBytes: 600, Count: 1},   // 60%
			{Name: "rust", FreedBytes: 300, Count: 1},   // 30%
		},
	}

	var buf bytes.Buffer
	RenderReport(&buf, data)
	out := buf.String()

	// Ensure sorted descending: node (60%) -> rust (30%) -> python (10%)
	idxNode := strings.Index(out, "node")
	idxRust := strings.Index(out, "rust")
	idxPython := strings.Index(out, "python")

	if idxNode == -1 || idxRust == -1 || idxPython == -1 {
		t.Fatalf("missing ecosystem in output: %s", out)
	}
	if !(idxNode < idxRust && idxRust < idxPython) {
		t.Errorf("ecosystems not sorted descending by freed size: node=%d, rust=%d, python=%d",
			idxNode, idxRust, idxPython)
	}

	// 60% of 10 = 6 filled, 4 empty: [██████░░░░]
	if !strings.Contains(out, "[██████░░░░]") {
		t.Errorf("expected 60%% mini bar [██████░░░░], got:\n%s", out)
	}
	// 30% of 10 = 3 filled, 7 empty: [███░░░░░░░]
	if !strings.Contains(out, "[███░░░░░░░]") {
		t.Errorf("expected 30%% mini bar [███░░░░░░░], got:\n%s", out)
	}
	// 10% of 10 = 1 filled, 9 empty: [█░░░░░░░░░]
	if !strings.Contains(out, "[█░░░░░░░░░]") {
		t.Errorf("expected 10%% mini bar [█░░░░░░░░░], got:\n%s", out)
	}
}

func TestReport_NoColorAndNonTTY(t *testing.T) {
	data := ReportData{
		Title:          "No Color Test",
		TotalFreed:     1024,
		TotalCount:     1,
		Duration:       10 * time.Millisecond,
		Ecosystems: []EcosystemMetric{
			{Name: "node", FreedBytes: 1024, Count: 1},
		},
	}

	// 1. Forced TTY with NO_COLOR set
	t.Setenv("NO_COLOR", "1")
	trueVal := true
	forceTTYForTest = &trueVal
	defer func() {
		forceTTYForTest = nil
	}()

	var bufNoColor bytes.Buffer
	RenderReport(&bufNoColor, data)
	outNoColor := bufNoColor.String()

	if strings.Contains(outNoColor, "\x1b[") {
		t.Errorf("expected NO_COLOR to suppress all ANSI escape codes, found \\x1b[ in:\n%s", outNoColor)
	}

	// 2. Non-TTY with NO_COLOR unset
	t.Setenv("NO_COLOR", "")
	falseVal := false
	forceTTYForTest = &falseVal

	var bufNonTTY bytes.Buffer
	RenderReport(&bufNonTTY, data)
	outNonTTY := bufNonTTY.String()

	if strings.Contains(outNonTTY, "\x1b[") {
		t.Errorf("expected non-TTY to suppress all ANSI escape codes, found \\x1b[ in:\n%s", outNonTTY)
	}

	// 3. TTY with NO_COLOR unset should contain color ANSI escapes
	forceTTYForTest = &trueVal
	var bufColor bytes.Buffer
	RenderReport(&bufColor, data)
	outColor := bufColor.String()

	if !strings.Contains(outColor, "\x1b[") {
		t.Errorf("expected TTY with color to contain ANSI escape codes, got:\n%s", outColor)
	}
}

func TestReport_JSONSerialization(t *testing.T) {
	diskBefore := uint64(1000)
	diskAfter := uint64(1500)

	data := ReportData{
		Title:          "JSON Test",
		TotalFreed:     500,
		TotalCount:     10,
		TrashCount:     8,
		PermanentCount: 2,
		Duration:       350 * time.Millisecond,
		DiskBefore:     &diskBefore,
		DiskAfter:      &diskAfter,
		Ecosystems: []EcosystemMetric{
			{Name: "docker", FreedBytes: 400, Count: 7},
			{Name: "gradle", FreedBytes: 100, Count: 3},
		},
	}

	var buf bytes.Buffer
	if err := FormatReportJSON(&buf, data); err != nil {
		t.Fatalf("FormatReportJSON failed: %v", err)
	}

	rawJSON := buf.String()

	// Verify indentation with 2 spaces
	if !strings.Contains(rawJSON, "  \"title\":") {
		t.Errorf("expected 2-space indented JSON, got:\n%s", rawJSON)
	}

	var parsed CleanReportJSON
	if err := json.Unmarshal([]byte(rawJSON), &parsed); err != nil {
		t.Fatalf("failed to parse generated JSON: %v", err)
	}

	if parsed.Title != "JSON Test" {
		t.Errorf("expected Title 'JSON Test', got %q", parsed.Title)
	}
	if parsed.TotalFreed != 500 {
		t.Errorf("expected TotalFreed 500, got %d", parsed.TotalFreed)
	}
	if parsed.TotalFreedHuman != "500 B" {
		t.Errorf("expected TotalFreedHuman '500 B', got %q", parsed.TotalFreedHuman)
	}
	if parsed.DurationMs != 350 {
		t.Errorf("expected DurationMs 350, got %d", parsed.DurationMs)
	}
	if parsed.TotalCount != 10 || parsed.TrashCount != 8 || parsed.PermanentCount != 2 {
		t.Errorf("unexpected target counts: %+v", parsed)
	}
	if parsed.DiskDiff == nil || *parsed.DiskDiff != 500 {
		t.Errorf("expected DiskDiff 500, got %v", parsed.DiskDiff)
	}
	if len(parsed.Ecosystems) != 2 {
		t.Errorf("expected 2 ecosystems, got %d", len(parsed.Ecosystems))
	}
}

func TestReport_ZeroEcosystemsAndNilDisk(t *testing.T) {
	data := ReportData{
		TotalFreed: 0,
		TotalCount: 0,
		Duration:   5 * time.Millisecond,
	}

	var buf bytes.Buffer
	RenderReport(&buf, data)
	out := buf.String()

	if !strings.Contains(out, "DevClean Summary") {
		t.Errorf("expected default title, got:\n%s", out)
	}
	if strings.Contains(out, "Volume em Disco:") {
		t.Errorf("should not display disk line when pointers are nil")
	}
	if strings.Contains(out, "Detalhamento por Ecossistema:") {
		t.Errorf("should not display ecosystem section when empty")
	}
}
