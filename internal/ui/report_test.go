package ui

import (
	"bytes"
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
