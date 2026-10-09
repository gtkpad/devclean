package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

func testModel() model {
	m := model{
		state:    stateSelecting,
		selected: make(map[string]bool),
		flatTargets: []flatTarget{
			{target: domain.Target{ID: "t1", Path: "/a", SizeBytes: 100, Risk: domain.RiskLow}, ecosystem: "go", groupIdx: 0},
			{target: domain.Target{ID: "t2", Path: "/b", SizeBytes: 200, Risk: domain.RiskMedium}, ecosystem: "go", groupIdx: 0},
			{target: domain.Target{ID: "t3", Path: "/c", SizeBytes: 300, Risk: domain.RiskLow}, ecosystem: "node", groupIdx: 1},
		},
		cfg:    &config.Config{TUI: config.TUIConfig{MassSelectThresholdMB: 100}},
		filter: filterState{sort: sortSize},
		width:  80,
		height: 40,
	}
	m.visibleTargets = []int{0, 1, 2}
	return m
}

func keyMsg(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
}

func TestUpdate_ScanComplete_TransitionsToSelecting(t *testing.T) {
	m := model{
		state:    stateScanning,
		selected: make(map[string]bool),
		cfg:      &config.Config{TUI: config.TUIConfig{MassSelectThresholdMB: 100}},
		filter:   filterState{sort: sortSize},
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
		cfg:      &config.Config{TUI: config.TUIConfig{MassSelectThresholdMB: 100}},
		filter:   filterState{sort: sortSize},
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

func TestTargetIcon(t *testing.T) {
	tests := []struct {
		name      string
		smartMode bool
		threshold float64
		score     float64
		want      string
	}{
		{"disabled returns empty", false, 0.7, 0.9, ""},
		{"above threshold returns bolt", true, 0.7, 0.8, "⚡ "},
		{"below threshold returns shield", true, 0.7, 0.5, "🛡️ "},
		{"at threshold returns bolt (inclusive)", true, 0.7, 0.7, "⚡ "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := model{SmartMode: tt.smartMode, SmartThreshold: tt.threshold}
			target := domain.Target{SmartScore: tt.score}
			got := m.targetIcon(target)
			if got != tt.want {
				t.Errorf("targetIcon() = %q, want %q", got, tt.want)
			}
		})
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

func TestUpdateSelecting_FilterMode(t *testing.T) {
	m := testModel()

	// Press / to enter filter mode
	updated, _ := m.Update(keyMsg('/'))
	um := updated.(model)
	if !um.filter.fuzzyActive {
		t.Error("fuzzyActive should be true after /")
	}

	// Type a character
	updated, _ = um.Update(keyMsg('a'))
	um = updated.(model)
	if um.filter.fuzzyText != "a" {
		t.Errorf("fuzzyText = %q, want 'a'", um.filter.fuzzyText)
	}

	// Press Esc exits filter mode
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um = updated.(model)
	if um.filter.fuzzyActive {
		t.Error("fuzzyActive should be false after Esc in filter mode")
	}
}

func TestUpdateSelecting_SortCycle(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('s'))
	um := updated.(model)
	if um.filter.sort != sortAge {
		t.Errorf("sort = %d, want sortAge (%d)", um.filter.sort, sortAge)
	}

	updated, _ = um.Update(keyMsg('s'))
	um = updated.(model)
	if um.filter.sort != sortEcosystem {
		t.Errorf("sort = %d, want sortEcosystem (%d)", um.filter.sort, sortEcosystem)
	}
}

func TestUpdateSelecting_RiskCycle(t *testing.T) {
	m := testModel()

	updated, _ := m.Update(keyMsg('r'))
	um := updated.(model)
	if um.filter.risk != riskFilterLow {
		t.Errorf("risk = %d, want riskFilterLow (%d)", um.filter.risk, riskFilterLow)
	}
}

func TestUpdateSelecting_MassSelect(t *testing.T) {
	m := testModel()
	// Set threshold to 0 so all targets qualify
	m.cfg.TUI.MassSelectThresholdMB = 0
	m.recompute()

	updated, _ := m.Update(keyMsg('m'))
	um := updated.(model)

	// All targets have size >= 0, so all should be selected
	if len(um.selected) != 3 {
		t.Errorf("selected count = %d, want 3 (all above 0 threshold)", len(um.selected))
	}
}

func TestUpdateSelecting_EscClearsFilters(t *testing.T) {
	m := testModel()
	m.filter.risk = riskFilterHigh
	m.filter.fuzzyText = "foo"
	m.recompute()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := updated.(model)

	if um.filter.risk != riskAll {
		t.Errorf("risk = %d, want riskAll", um.filter.risk)
	}
	if um.filter.fuzzyText != "" {
		t.Errorf("fuzzyText = %q, want empty", um.filter.fuzzyText)
	}
}

func TestUpdateSelecting_SelectionsPersistAfterFilterClear(t *testing.T) {
	m := testModel()
	m.selected["t1"] = true

	// Apply and then clear a filter
	m.filter.risk = riskFilterLow
	m.recompute()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := updated.(model)

	// Selection should persist
	if !um.selected["t1"] {
		t.Error("selection of t1 should persist after clearing filters")
	}
}

type fakeCleanerProvider struct {
	cleaner domain.Cleaner
}

func (p *fakeCleanerProvider) GetCleaner(name string) domain.Cleaner {
	return p.cleaner
}

type fakeCleaner struct {
	cleaned []domain.Target
}

func (f *fakeCleaner) Name() string                                              { return "go" }
func (f *fakeCleaner) IsInstalled() bool                                         { return true }
func (f *fakeCleaner) Scan(_ context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	return nil, nil
}
func (f *fakeCleaner) Clean(_ context.Context, targets []domain.Target) error {
	f.cleaned = append(f.cleaned, targets...)
	return nil
}

func TestDeleteCmd_WithCoordinator(t *testing.T) {
	fc := &fakeCleaner{}
	cleaners := &fakeCleanerProvider{cleaner: fc}
	cfg := &config.Config{}
	bl := safety.NewBlacklist(nil)
	coord := deleter.New(cleaners, cfg, bl, nil, nil, "", "")

	m := testModel()
	m.coordinator = coord
	m.selected["t1"] = true // t1 is go ecosystem, size 100
	m.selected["t2"] = true // t2 is go ecosystem, size 200

	cmd := deleteCmd(m)
	msg := cmd()
	completeMsg, ok := msg.(deleteCompleteMsg)
	if !ok {
		t.Fatalf("expected deleteCompleteMsg, got %T", msg)
	}
	if completeMsg.count != 2 {
		t.Errorf("expected count 2, got %d", completeMsg.count)
	}
	if completeMsg.freed != 300 {
		t.Errorf("expected freed 300, got %d", completeMsg.freed)
	}
	if len(fc.cleaned) != 2 {
		t.Errorf("expected 2 cleaned targets in fakeCleaner, got %d", len(fc.cleaned))
	}
}

func TestModel_NewAndInit(t *testing.T) {
	cfg := &config.Config{
		Safety: config.SafetyConfig{TrashEnabled: false},
	}
	bl := safety.NewBlacklist(nil)
	m := New(nil, bl, cfg)

	if m.state != stateScanning {
		t.Errorf("expected stateScanning, got %v", m.state)
	}

	cmd := m.Init()
	if cmd == nil {
		t.Error("expected non-nil tea.Cmd from Init()")
	}

	// Clean up context created by New
	if m.scanCancel != nil {
		m.scanCancel()
	}
}

func TestModel_WindowSizeMsg(t *testing.T) {
	m := testModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 50})
	um := updated.(model)

	if um.width != 120 {
		t.Errorf("width = %d, want 120", um.width)
	}
	if um.height != 50 {
		t.Errorf("height = %d, want 50", um.height)
	}
}

func TestModel_KeyHandling(t *testing.T) {
	t.Run("Scanning state keys", func(t *testing.T) {
		m := testModel()
		m.state = stateScanning
		cancelled := false
		m.scanCancel = func() { cancelled = true }

		// 'q' quits and cancels
		updated, cmd := m.Update(keyMsg('q'))
		if cmd == nil {
			t.Error("expected quit cmd for 'q' in scanning")
		}
		if !cancelled {
			t.Error("expected scanCancel called on 'q'")
		}

		// Ctrl+C quits and cancels
		cancelled = false
		updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Error("expected quit cmd for Ctrl+C in scanning")
		}
		if !cancelled {
			t.Error("expected scanCancel called on Ctrl+C")
		}

		// Other key does nothing
		cancelled = false
		updated, cmd = m.Update(keyMsg('x'))
		_ = updated
		if cmd != nil || cancelled {
			t.Error("expected no quit or cancel for unrecognized key in scanning")
		}
	})

	t.Run("Selecting state quit keys", func(t *testing.T) {
		m := testModel()
		_, cmd := m.Update(keyMsg('q'))
		if cmd == nil {
			t.Error("expected tea.Quit on 'q' in selecting")
		}

		_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Error("expected tea.Quit on Ctrl+C in selecting")
		}
	})

	t.Run("Selecting state arrow navigation and bounds", func(t *testing.T) {
		m := testModel()
		m.cursor = 0

		// KeyDown moves cursor
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(model)
		if m.cursor != 1 {
			t.Errorf("cursor after KeyDown = %d, want 1", m.cursor)
		}

		// KeyUp moves cursor back
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
		m = updated.(model)
		if m.cursor != 0 {
			t.Errorf("cursor after KeyUp = %d, want 0", m.cursor)
		}

		// Cursor down at end doesn't overflow
		m.cursor = len(m.visibleTargets) - 1
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = updated.(model)
		if m.cursor != len(m.visibleTargets)-1 {
			t.Errorf("cursor down at end overflowed: %d", m.cursor)
		}

		// Space on out-of-bounds cursor does not panic
		m.cursor = 99
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
		m = updated.(model)
	})

	t.Run("Filter input keys", func(t *testing.T) {
		m := testModel()
		m.filter.fuzzyActive = true
		m.filter.fuzzyText = "abc"

		// KeyBackspace deletes character
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(model)
		if m.filter.fuzzyText != "ab" {
			t.Errorf("fuzzyText after backspace = %q, want 'ab'", m.filter.fuzzyText)
		}

		// KeyBackspace on empty does nothing
		m.filter.fuzzyText = ""
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(model)
		if m.filter.fuzzyText != "" {
			t.Errorf("fuzzyText after empty backspace = %q, want empty", m.filter.fuzzyText)
		}

		// KeyEnter exits filter mode
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = updated.(model)
		if m.filter.fuzzyActive {
			t.Error("expected fuzzyActive = false after KeyEnter")
		}

		// Other key in filter mode does nothing
		m.filter.fuzzyActive = true
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = updated.(model)
		if !m.filter.fuzzyActive {
			t.Error("fuzzyActive should remain true on KeyTab")
		}
	})

	t.Run("Confirming state keys", func(t *testing.T) {
		m := testModel()
		m.state = stateConfirming

		// 'y' transitions to stateDeleting and initiates deleteCmd
		updated, cmd := m.Update(keyMsg('y'))
		um := updated.(model)
		if um.state != stateDeleting {
			t.Errorf("state after 'y' = %v, want stateDeleting", um.state)
		}
		if cmd == nil {
			t.Error("expected delete batch cmd on 'y'")
		}

		// 'q' returns to selecting
		m.state = stateConfirming
		updated, _ = m.Update(keyMsg('q'))
		um = updated.(model)
		if um.state != stateSelecting {
			t.Errorf("state after 'q' = %v, want stateSelecting", um.state)
		}

		// Ctrl+C quits
		m.state = stateConfirming
		_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		if cmd == nil {
			t.Error("expected quit cmd on Ctrl+C in confirming")
		}

		// Other key does nothing
		m.state = stateConfirming
		updated, _ = m.Update(keyMsg('z'))
		um = updated.(model)
		if um.state != stateConfirming {
			t.Errorf("state should remain stateConfirming on 'z'")
		}
	})

	t.Run("Done state exits on any key", func(t *testing.T) {
		m := testModel()
		m.state = stateDone
		_, cmd := m.Update(keyMsg('x'))
		if cmd == nil {
			t.Error("expected quit on any key in stateDone")
		}
	})
}

func TestModel_ViewMethodCoverage(t *testing.T) {
	m := testModel()

	m.state = stateScanning
	if view := m.View(); !strings.Contains(view, "Scanning...") {
		t.Errorf("view in stateScanning missing Scanning...: %s", view)
	}

	m.state = stateSelecting
	if view := m.View(); !strings.Contains(view, "go") {
		t.Errorf("view in stateSelecting missing 'go': %s", view)
	}

	m.state = stateConfirming
	m.selected["t1"] = true
	if view := m.View(); !strings.Contains(view, "Confirm deletion?") {
		t.Errorf("view in stateConfirming missing header: %s", view)
	}

	m.state = stateDeleting
	if view := m.View(); !strings.Contains(view, "Deleting targets...") {
		t.Errorf("view in stateDeleting missing text: %s", view)
	}

	m.state = stateDone
	m.emptyResult = true
	if view := m.View(); !strings.Contains(view, "All clean!") {
		t.Errorf("view in stateDone missing text: %s", view)
	}

	// Unknown state returns empty string
	m.state = viewState(99)
	if view := m.View(); view != "" {
		t.Errorf("view with invalid state = %q, want empty", view)
	}
}

func TestModel_MessagesAndBranches(t *testing.T) {
	t.Run("Spinner tick messages", func(t *testing.T) {
		m := testModel()

		m.state = stateScanning
		_, cmd := m.Update(spinner.TickMsg{})
		if cmd == nil {
			t.Error("expected tick cmd in stateScanning")
		}

		m.state = stateDeleting
		_, cmd = m.Update(spinner.TickMsg{})
		if cmd == nil {
			t.Error("expected tick cmd in stateDeleting")
		}

		m.state = stateSelecting
		_, cmd = m.Update(spinner.TickMsg{})
		if cmd != nil {
			t.Error("expected nil cmd for spinner in stateSelecting")
		}
	})

	t.Run("Ecosystem result messages with error and success", func(t *testing.T) {
		m := testModel()
		m.state = stateScanning
		m.ecoNames = []string{"docker", "node"}
		m.ecoStatus = map[string]string{"docker": "scanning", "node": "scanning"}

		// With error
		updated, _ := m.Update(ecosystemResultMsg{
			result: domain.ScanResult{Ecosystem: "docker", Error: errors.New("err")},
		})
		um := updated.(model)
		if um.ecoStatus["docker"] != "error" {
			t.Errorf("status = %q, want 'error'", um.ecoStatus["docker"])
		}

		// With success
		updated, _ = um.Update(ecosystemResultMsg{
			result: domain.ScanResult{Ecosystem: "node", Error: nil},
		})
		um = updated.(model)
		if um.ecoStatus["node"] != "done" {
			t.Errorf("status = %q, want 'done'", um.ecoStatus["node"])
		}
	})

	t.Run("ScanDoneMsg with and without results", func(t *testing.T) {
		// Empty results
		m := testModel()
		m.state = stateScanning
		m.results = nil
		updated, _ := m.Update(scanDoneMsg{})
		um := updated.(model)
		if um.state != stateDone || !um.emptyResult {
			t.Error("expected stateDone and emptyResult = true for empty scanDoneMsg")
		}

		// Non-empty results
		m = testModel()
		m.state = stateScanning
		m.results = []domain.ScanResult{
			{Ecosystem: "go", Targets: []domain.Target{{ID: "g1", Path: "/path"}}},
		}
		updated, _ = m.Update(scanDoneMsg{})
		um = updated.(model)
		if um.state != stateSelecting {
			t.Errorf("expected stateSelecting, got %v", um.state)
		}
	})

	t.Run("NextGroupStart when visibleTargets is empty", func(t *testing.T) {
		m := testModel()
		m.visibleTargets = nil
		next := m.nextGroupStart()
		if next != 0 {
			t.Errorf("nextGroupStart = %d, want 0", next)
		}
	})

	t.Run("AdjustOffset edge cases", func(t *testing.T) {
		m := testModel()
		m.height = 8 // visibleHeight = overhead - 8 = 0 -> clamped to 20
		m.cursor = 25
		m.offset = 0
		m.adjustOffset()
		if m.offset == 0 {
			t.Error("expected offset to adjust when cursor >= offset + visible")
		}

		m.cursor = 2
		m.adjustOffset()
		if m.offset != 2 {
			t.Errorf("expected offset to adjust to cursor when cursor < offset, got %d", m.offset)
		}
	})
}

func TestDeleteCmd_NilCoordinatorAndError(t *testing.T) {
	// Coordinator is nil
	m := testModel()
	m.coordinator = nil
	cmd := deleteCmd(m)
	msg := cmd()
	completeMsg, ok := msg.(deleteCompleteMsg)
	if !ok || completeMsg.count != 0 || completeMsg.freed != 0 {
		t.Errorf("expected zero deleteCompleteMsg when coordinator is nil, got %+v", msg)
	}
}

func TestHumanSize_AllUnits(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		got := humanSize(tt.bytes)
		if got != tt.want {
			t.Errorf("humanSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestModel_WaitForScanResult(t *testing.T) {
	ch := make(chan domain.ScanResult, 2)
	ch <- domain.ScanResult{Ecosystem: "node"}
	close(ch)

	m := testModel()
	m.scanCh = ch

	cmd := m.waitForScanResult()
	msg1 := cmd()
	if ecoMsg, ok := msg1.(ecosystemResultMsg); !ok || ecoMsg.result.Ecosystem != "node" {
		t.Errorf("expected ecosystemResultMsg for node, got %+v", msg1)
	}

	msg2 := cmd()
	if _, ok := msg2.(scanDoneMsg); !ok {
		t.Errorf("expected scanDoneMsg on closed channel, got %+v", msg2)
	}
}


