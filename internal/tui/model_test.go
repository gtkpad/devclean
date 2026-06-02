package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/domain"
)

func testModel() model {
	return model{
		state:    stateSelecting,
		selected: make(map[string]bool),
		flatTargets: []flatTarget{
			{target: domain.Target{ID: "t1", Path: "/a", SizeBytes: 100, Risk: domain.RiskLow}, ecosystem: "go", groupIdx: 0},
			{target: domain.Target{ID: "t2", Path: "/b", SizeBytes: 200, Risk: domain.RiskMedium}, ecosystem: "go", groupIdx: 0},
			{target: domain.Target{ID: "t3", Path: "/c", SizeBytes: 300, Risk: domain.RiskLow}, ecosystem: "node", groupIdx: 1},
		},
		width:  80,
		height: 40,
	}
}

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestUpdate_ScanComplete_TransitionsToSelecting(t *testing.T) {
	m := model{
		state:    stateScanning,
		selected: make(map[string]bool),
	}

	results := []domain.ScanResult{
		{Ecosystem: "go", Targets: []domain.Target{{ID: "t1", Path: "/a"}}},
	}

	updated, _ := m.Update(scanCompleteMsg{results: results})
	um := updated.(model)

	if um.state != stateSelecting {
		t.Errorf("state = %v, want stateSelecting", um.state)
	}
	if len(um.flatTargets) != 1 {
		t.Errorf("flatTargets len = %d, want 1", len(um.flatTargets))
	}
}

func TestUpdate_ScanComplete_EmptyResults_GoesToDone(t *testing.T) {
	m := model{
		state:    stateScanning,
		selected: make(map[string]bool),
	}

	updated, _ := m.Update(scanCompleteMsg{results: nil})
	um := updated.(model)

	if um.state != stateDone {
		t.Errorf("state = %v, want stateDone", um.state)
	}
	if !um.emptyResult {
		t.Error("emptyResult should be true")
	}
}

func TestUpdate_Selecting_CursorDown(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('j'))
	um := updated.(model)

	if um.cursor != 1 {
		t.Errorf("cursor = %d, want 1", um.cursor)
	}
}

func TestUpdate_Selecting_CursorUp(t *testing.T) {
	m := testModel()
	m.cursor = 2

	updated, _ := m.Update(keyMsg('k'))
	um := updated.(model)

	if um.cursor != 1 {
		t.Errorf("cursor = %d, want 1", um.cursor)
	}
}

func TestUpdate_Selecting_CursorUp_AtTop(t *testing.T) {
	m := testModel()
	m.cursor = 0

	updated, _ := m.Update(keyMsg('k'))
	um := updated.(model)

	if um.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (shouldn't go negative)", um.cursor)
	}
}

func TestUpdate_Selecting_Space_TogglesSelection(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	um := updated.(model)

	if !um.selected["t1"] {
		t.Error("t1 should be selected after space")
	}

	// Toggle off
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeySpace})
	um = updated.(model)

	if um.selected["t1"] {
		t.Error("t1 should be deselected after second space")
	}
}

func TestUpdate_Selecting_SelectAll(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('a'))
	um := updated.(model)

	if len(um.selected) != 3 {
		t.Errorf("selected count = %d, want 3", len(um.selected))
	}
}

func TestUpdate_Selecting_DeselectAll(t *testing.T) {
	m := testModel()
	m.selected["t1"] = true
	m.selected["t2"] = true

	updated, _ := m.Update(keyMsg('n'))
	um := updated.(model)

	if len(um.selected) != 0 {
		t.Errorf("selected count = %d, want 0", len(um.selected))
	}
}

func TestUpdate_Selecting_LowRiskOnly(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('l'))
	um := updated.(model)

	if len(um.selected) != 2 {
		t.Errorf("selected count = %d, want 2 (low-risk only)", len(um.selected))
	}
	if !um.selected["t1"] || !um.selected["t3"] {
		t.Error("should select only low-risk targets t1 and t3")
	}
}

func TestUpdate_Selecting_Enter_TransitionsToConfirming(t *testing.T) {
	m := testModel()
	m.selected["t1"] = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)

	if um.state != stateConfirming {
		t.Errorf("state = %v, want stateConfirming", um.state)
	}
}

func TestUpdate_Selecting_Enter_NoSelection_StaysInSelecting(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um := updated.(model)

	if um.state != stateSelecting {
		t.Errorf("state = %v, want stateSelecting (no selection)", um.state)
	}
}

func TestUpdate_Confirming_N_GoesBackToSelecting(t *testing.T) {
	m := testModel()
	m.state = stateConfirming

	updated, _ := m.Update(keyMsg('n'))
	um := updated.(model)

	if um.state != stateSelecting {
		t.Errorf("state = %v, want stateSelecting", um.state)
	}
}

func TestUpdate_DeleteComplete_TransitionsToDone(t *testing.T) {
	m := testModel()
	m.state = stateDeleting

	updated, _ := m.Update(deleteCompleteMsg{freed: 1024, count: 2})
	um := updated.(model)

	if um.state != stateDone {
		t.Errorf("state = %v, want stateDone", um.state)
	}
	if um.freedBytes != 1024 {
		t.Errorf("freedBytes = %d, want 1024", um.freedBytes)
	}
	if um.freedCount != 2 {
		t.Errorf("freedCount = %d, want 2", um.freedCount)
	}
}

func TestUpdate_Tab_JumpsToNextGroup(t *testing.T) {
	m := testModel()
	m.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	um := updated.(model)

	if um.cursor != 2 {
		t.Errorf("cursor = %d, want 2 (next group start)", um.cursor)
	}
}

func TestSelectedTotal(t *testing.T) {
	m := testModel()
	m.selected["t1"] = true
	m.selected["t3"] = true

	bytes, count := m.selectedTotal()
	if bytes != 400 {
		t.Errorf("selectedTotal bytes = %d, want 400", bytes)
	}
	if count != 2 {
		t.Errorf("selectedTotal count = %d, want 2", count)
	}
}
