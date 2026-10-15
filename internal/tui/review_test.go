package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/domain"
)

func testModelWithReview() model {
	m := model{
		state:    stateSelecting,
		selected: make(map[string]bool),
		flatTargets: []flatTarget{
			{
				target: domain.Target{
					ID:         "t1",
					Name:       "cargo-target",
					Path:       "/project/target",
					SizeBytes:  1000,
					Risk:       domain.RiskLow,
					ReviewOnly: false,
				},
				ecosystem: "rust",
				groupIdx:  0,
			},
			{
				target: domain.Target{
					ID:           "t2-review",
					Name:         "pnpm-store",
					Path:         "/Users/dev/Library/pnpm/store",
					SizeBytes:    5000000000,
					Risk:         domain.RiskLow,
					ReviewOnly:   true,
					ReviewReason: domain.ReviewReasonSharedStore,
				},
				ecosystem: "node",
				groupIdx:  1,
			},
			{
				target: domain.Target{
					ID:         "t3",
					Name:       "go-cache",
					Path:       "/Users/dev/.cache/go-build",
					SizeBytes:  2000,
					Risk:       domain.RiskLow,
					ReviewOnly: false,
				},
				ecosystem: "go",
				groupIdx:  2,
			},
		},
		cfg:    &config.Config{TUI: config.TUIConfig{MassSelectThresholdMB: 0}},
		filter: filterState{sort: sortSize},
		width:  80,
		height: 40,
	}
	m.visibleTargets = []int{0, 1, 2}
	return m
}

func TestTUI_ReviewOnly_RenderLockedIndicator(t *testing.T) {
	m := testModelWithReview()

	// Normal target should render [ ]
	normalLine := renderTargetLine(m, 0, m.flatTargets[0])
	if !strings.Contains(normalLine, "[ ]") {
		t.Errorf("expected normal target to have '[ ]', got %q", normalLine)
	}

	// ReviewOnly target should render [⊙]
	reviewLine := renderTargetLine(m, 1, m.flatTargets[1])
	if !strings.Contains(reviewLine, "[⊙]") {
		t.Errorf("expected ReviewOnly target to render '[⊙]', got %q", reviewLine)
	}
	if strings.Contains(reviewLine, "[ ]") || strings.Contains(reviewLine, "[x]") {
		t.Errorf("ReviewOnly target should not render regular checkbox '[ ]' or '[x]', got %q", reviewLine)
	}
}

func TestTUI_ReviewOnly_SpaceToggle_Ignored(t *testing.T) {
	m := testModelWithReview()

	// Position cursor on ReviewOnly target (index 1)
	m.cursor = 1

	// Press space
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	um := updated.(model)

	// Target should NOT be selected
	if um.selected["t2-review"] {
		t.Error("ReviewOnly target should not be selected after Space key")
	}

	// Status message should indicate rejection with review reason
	if !strings.Contains(um.statusMessage, "shared_store") && !strings.Contains(um.statusMessage, "revisão") {
		t.Errorf("expected statusMessage to explain review lock, got %q", um.statusMessage)
	}
}

func TestTUI_ReviewOnly_MassSelect_Excluded(t *testing.T) {
	t.Run("select all ('a') excludes ReviewOnly", func(t *testing.T) {
		m := testModelWithReview()
		updated, _ := m.Update(keyMsg('a'))
		um := updated.(model)

		if um.selected["t2-review"] {
			t.Error("'a' (select all) must exclude ReviewOnly targets")
		}
		if !um.selected["t1"] || !um.selected["t3"] {
			t.Error("'a' (select all) should select regular targets")
		}
	})

	t.Run("mass select ('m') excludes ReviewOnly", func(t *testing.T) {
		m := testModelWithReview()
		// Mass select threshold is 0, so all regular targets qualify
		updated, _ := m.Update(keyMsg('m'))
		um := updated.(model)

		if um.selected["t2-review"] {
			t.Error("'m' (mass select) must exclude ReviewOnly targets even if size exceeds threshold")
		}
		if !um.selected["t1"] || !um.selected["t3"] {
			t.Error("'m' (mass select) should select regular targets above threshold")
		}
	})

	t.Run("low risk select ('l') excludes ReviewOnly", func(t *testing.T) {
		m := testModelWithReview()
		// All targets in fixture are RiskLow
		updated, _ := m.Update(keyMsg('l'))
		um := updated.(model)

		if um.selected["t2-review"] {
			t.Error("'l' (low risk select) must exclude ReviewOnly targets even with RiskLow")
		}
		if !um.selected["t1"] || !um.selected["t3"] {
			t.Error("'l' (low risk select) should select regular low risk targets")
		}
	})
}
