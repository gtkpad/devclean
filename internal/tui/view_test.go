package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/gtkpad/devclean/internal/domain"
)

func TestView_Scanning(t *testing.T) {
	m := testModel()
	m.state = stateScanning
	m.spinner = spinner.New()
	m.ecoNames = []string{"docker", "node", "go", "rust"}
	m.ecoStatus = map[string]string{
		"docker": "scanning",
		"node":   "done",
		"go":     "waiting",
		"rust":   "error",
	}
	m.results = []domain.ScanResult{
		{
			Ecosystem: "node",
			Targets: []domain.Target{
				{ID: "n1", Path: "/workspace/node_modules", SizeBytes: 1048576},
			},
		},
	}

	out := viewScanning(m)

	if !strings.Contains(out, "Scanning...") {
		t.Error("expected Scanning... in viewScanning output")
	}
	if !strings.Contains(out, "docker") || !strings.Contains(out, "node") || !strings.Contains(out, "go") || !strings.Contains(out, "rust") {
		t.Error("expected all ecosystem names in viewScanning output")
	}
	if !strings.Contains(out, "(waiting)") {
		t.Error("expected (waiting) status for go")
	}
	if !strings.Contains(out, "(error)") {
		t.Error("expected (error) status for rust")
	}
	if !strings.Contains(out, "(1 targets, 1.0 MB)") {
		t.Errorf("expected node summary in viewScanning, got: %s", out)
	}
	if !strings.Contains(out, "q: quit") {
		t.Error("expected quit help in viewScanning footer")
	}
}

func TestView_EcosystemDoneSummary(t *testing.T) {
	m := testModel()
	m.results = []domain.ScanResult{
		{
			Ecosystem: "node",
			Error:     nil,
			Targets: []domain.Target{
				{ID: "n1", SizeBytes: 2048},
				{ID: "n2", SizeBytes: 2048},
			},
		},
		{
			Ecosystem: "rust",
			Error:     errors.New("scan failed"),
		},
	}

	// 1. Success matching ecosystem
	summary := ecosystemDoneSummary(m, "node")
	if summary != "(2 targets, 4.0 KB)" {
		t.Errorf("ecosystemDoneSummary(node) = %q, want '(2 targets, 4.0 KB)'", summary)
	}

	// 2. Error ecosystem returns default (done)
	summaryRust := ecosystemDoneSummary(m, "rust")
	if summaryRust != "(done)" {
		t.Errorf("ecosystemDoneSummary(rust) = %q, want '(done)'", summaryRust)
	}

	// 3. Not found ecosystem returns default (done)
	summaryUnknown := ecosystemDoneSummary(m, "unknown")
	if summaryUnknown != "(done)" {
		t.Errorf("ecosystemDoneSummary(unknown) = %q, want '(done)'", summaryUnknown)
	}
}

func TestView_Selecting(t *testing.T) {
	t.Run("No selection and empty filter", func(t *testing.T) {
		m := testModel()
		m.state = stateSelecting
		m.width = 100
		m.height = 40

		out := viewSelecting(m)
		if !strings.Contains(out, "No targets selected") {
			t.Error("expected 'No targets selected' when selected is empty")
		}
		if !strings.Contains(out, "↑↓/jk: navigate") {
			t.Error("expected standard footer help in viewSelecting")
		}
	})

	t.Run("With selection and active filter", func(t *testing.T) {
		m := testModel()
		m.state = stateSelecting
		m.width = 100
		m.height = 40
		m.selected["t1"] = true
		m.filter.sort = sortAge

		out := viewSelecting(m)
		if !strings.Contains(out, "Selected: 1 targets") {
			t.Errorf("expected selected count in header, got: %s", out)
		}
		if !strings.Contains(out, m.filter.label()) {
			t.Errorf("expected filter label in viewSelecting, got: %s", out)
		}
	})

	t.Run("Fuzzy filter active", func(t *testing.T) {
		m := testModel()
		m.state = stateSelecting
		m.filter.fuzzyActive = true
		m.filter.fuzzyText = "cache"

		out := viewSelecting(m)
		if !strings.Contains(out, "/ cache▎") {
			t.Errorf("expected fuzzy input bar with cursor, got: %s", out)
		}
		if !strings.Contains(out, "type to filter | Esc: exit filter | Enter: apply") {
			t.Errorf("expected fuzzy input footer, got: %s", out)
		}
	})

	t.Run("Fuzzy filter inactive but text present", func(t *testing.T) {
		m := testModel()
		m.state = stateSelecting
		m.filter.fuzzyActive = false
		m.filter.fuzzyText = "cache"

		out := viewSelecting(m)
		if !strings.Contains(out, "filter: cache") {
			t.Errorf("expected filter indicator when fuzzy text is present, got: %s", out)
		}
	})

	t.Run("Scroll indicator when list exceeds visible height", func(t *testing.T) {
		m := testModel()
		m.height = 10 // visibleHeight becomes max(1, 10 - 8) = 2
		m.flatTargets = make([]flatTarget, 20)
		m.visibleTargets = make([]int, 20)
		for i := 0; i < 20; i++ {
			m.flatTargets[i] = flatTarget{
				target:    domain.Target{ID: string(rune('a' + i)), Path: "/path", SizeBytes: 100},
				ecosystem: "go",
				groupIdx:  0,
			}
			m.visibleTargets[i] = i
		}
		m.cursor = 5
		m.adjustOffset()

		out := viewSelecting(m)
		if !strings.Contains(out, "[6/20]") {
			t.Errorf("expected scroll indicator [6/20], got: %s", out)
		}
	})
}

func TestView_RenderDashboard(t *testing.T) {
	t.Run("Empty results", func(t *testing.T) {
		m := model{results: nil}
		got := renderDashboard(m)
		if got != "  No ecosystems found" {
			t.Errorf("renderDashboard(empty) = %q, want '  No ecosystems found'", got)
		}
	})

	t.Run("Results with errors and empty targets", func(t *testing.T) {
		m := model{
			results: []domain.ScanResult{
				{Ecosystem: "docker", Error: errors.New("daemon down")},
				{Ecosystem: "node", Targets: nil},
			},
		}
		got := renderDashboard(m)
		if got != "  No ecosystems found" {
			t.Errorf("renderDashboard(errors/empty) = %q, want '  No ecosystems found'", got)
		}
	})

	t.Run("Valid results", func(t *testing.T) {
		m := model{
			results: []domain.ScanResult{
				{
					Ecosystem: "go",
					Targets: []domain.Target{
						{SizeBytes: 1048576},
					},
				},
				{
					Ecosystem: "rust",
					Targets: []domain.Target{
						{SizeBytes: 2097152},
						{SizeBytes: 1048576},
					},
				},
			},
		}
		got := renderDashboard(m)
		if !strings.Contains(got, "go: 1 targets (1.0 MB)") || !strings.Contains(got, "rust: 2 targets (3.0 MB)") {
			t.Errorf("renderDashboard(valid) unexpected output: %s", got)
		}
	})
}

func TestView_RenderTargetLine(t *testing.T) {
	m := testModel()
	m.width = 120
	m.cursor = 0
	m.selected["t1"] = true

	ftSelected := flatTarget{
		target: domain.Target{
			ID:        "t1",
			Path:      "/short/path",
			SizeBytes: 512,
			Risk:      domain.RiskLow,
		},
		ecosystem: "go",
	}

	// 1. Cursor on selected item
	line := renderTargetLine(m, 0, ftSelected)
	if !strings.HasPrefix(line, "> [x]") {
		t.Errorf("expected cursor and checked box, got line: %q", line)
	}
	if !strings.Contains(line, "LOW") {
		t.Errorf("expected LOW badge, got: %q", line)
	}

	// 2. Cursor not on unselected item
	ftUnselected := flatTarget{
		target: domain.Target{
			ID:        "t2",
			Path:      "/short/path",
			SizeBytes: 1024,
			Risk:      domain.RiskHigh,
		},
		ecosystem: "go",
	}
	line2 := renderTargetLine(m, 1, ftUnselected)
	if !strings.HasPrefix(line2, "  [ ]") {
		t.Errorf("expected no cursor and unchecked box, got line: %q", line2)
	}
	if !strings.Contains(line2, "HIGH") {
		t.Errorf("expected HIGH badge, got: %q", line2)
	}

	// 3. Path truncation for width <= 90 (maxPathLen = 50)
	m.width = 80
	longPath := "/Users/developer/very/deeply/nested/directory/structure/that/exceeds/the/fifty/characters/limit/node_modules"
	ftLong := flatTarget{
		target: domain.Target{
			ID:        "t3",
			Path:      longPath,
			SizeBytes: 100,
			Risk:      domain.RiskMedium,
		},
	}
	lineLong := renderTargetLine(m, 2, ftLong)
	if !strings.Contains(lineLong, "...") {
		t.Errorf("expected truncated path with ellipsis, got: %s", lineLong)
	}

	// 4. Path truncation for width > 90 (maxPathLen = width - 40 = 60)
	m.width = 100
	lineLong100 := renderTargetLine(m, 2, ftLong)
	if !strings.Contains(lineLong100, "...") {
		t.Errorf("expected truncated path with ellipsis for width 100, got: %s", lineLong100)
	}
}

func TestView_RiskBadge(t *testing.T) {
	tests := []struct {
		risk domain.RiskLevel
		want string
	}{
		{domain.RiskLow, "LOW"},
		{domain.RiskMedium, "MED"},
		{domain.RiskHigh, "HIGH"},
		{domain.RiskLevel("CRITICAL"), "CRITICAL"},
	}

	for _, tt := range tests {
		got := riskBadge(tt.risk)
		if !strings.Contains(got, tt.want) {
			t.Errorf("riskBadge(%v) = %q, want substring %q", tt.risk, got, tt.want)
		}
	}
}

func TestView_EcosystemTotals(t *testing.T) {
	m := testModel()
	total, count := ecosystemTotals(m, "go")
	if total != 300 || count != 2 {
		t.Errorf("ecosystemTotals(go) = (%d, %d), want (300, 2)", total, count)
	}

	totalNode, countNode := ecosystemTotals(m, "node")
	if totalNode != 300 || countNode != 1 {
		t.Errorf("ecosystemTotals(node) = (%d, %d), want (300, 1)", totalNode, countNode)
	}

	totalMissing, countMissing := ecosystemTotals(m, "missing")
	if totalMissing != 0 || countMissing != 0 {
		t.Errorf("ecosystemTotals(missing) = (%d, %d), want (0, 0)", totalMissing, countMissing)
	}
}

func TestView_Confirming(t *testing.T) {
	t.Run("Under 10 targets selected", func(t *testing.T) {
		m := testModel()
		m.state = stateConfirming
		m.selected["t1"] = true
		m.selected["t2"] = true

		out := viewConfirming(m)
		if !strings.Contains(out, "Confirm deletion?") {
			t.Error("expected 'Confirm deletion?' in header")
		}
		if !strings.Contains(out, "Targets: 2 selected") {
			t.Errorf("expected 2 selected, got: %s", out)
		}
		if !strings.Contains(out, "/a") || !strings.Contains(out, "/b") {
			t.Errorf("expected paths /a and /b listed, got: %s", out)
		}
		if !strings.Contains(out, "[y] confirm  [n] cancel") {
			t.Error("expected confirm/cancel prompt in footer")
		}
	})

	t.Run("Over 10 targets selected (overflow + X more)", func(t *testing.T) {
		m := testModel()
		m.state = stateConfirming
		m.flatTargets = make([]flatTarget, 15)
		for i := 0; i < 15; i++ {
			id := string(rune('a' + i))
			m.flatTargets[i] = flatTarget{
				target: domain.Target{ID: id, Path: "/path/" + id, SizeBytes: 100},
			}
			m.selected[id] = true
		}

		out := viewConfirming(m)
		if !strings.Contains(out, "+ 5 more") {
			t.Errorf("expected '+ 5 more' overflow line, got: %s", out)
		}
	})
}

func TestView_Deleting(t *testing.T) {
	m := testModel()
	m.state = stateDeleting
	m.spinner = spinner.New()

	out := viewDeleting(m)
	if !strings.Contains(out, "Deleting targets...") {
		t.Errorf("expected 'Deleting targets...' in viewDeleting, got: %s", out)
	}
}

func TestView_Done(t *testing.T) {
	t.Run("Empty result", func(t *testing.T) {
		m := testModel()
		m.state = stateDone
		m.emptyResult = true

		out := viewDone(m)
		if !strings.Contains(out, "All clean! No targets found") {
			t.Errorf("expected empty result message, got: %s", out)
		}
		if !strings.Contains(out, "Press any key to exit") {
			t.Error("expected exit prompt")
		}
	})

	t.Run("Completed with freed space", func(t *testing.T) {
		m := testModel()
		m.state = stateDone
		m.emptyResult = false
		m.freedBytes = 10485760
		m.freedCount = 7

		out := viewDone(m)
		if !strings.Contains(out, "Done! Freed 10.0 MB from 7 targets.") {
			t.Errorf("expected freed summary, got: %s", out)
		}
		if !strings.Contains(out, "Press any key to exit") {
			t.Error("expected exit prompt")
		}
	})
}
