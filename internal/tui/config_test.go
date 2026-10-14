package tui_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/gtkpad/devclean/internal/tui"
	"gopkg.in/yaml.v3"
)

func TestConfigTUI_TabSwitching(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	if m.ActiveTab() != tui.TabPaths {
		t.Fatalf("expected initial tab TabPaths (0), got %v", m.ActiveTab())
	}

	// Tab advances cyclically (0 -> 1 -> 2 -> 3 -> 4 -> 0)
	tabs := []tui.ConfigTab{
		tui.TabEcosystems,
		tui.TabSafety,
		tui.TabWatch,
		tui.TabScheduler,
		tui.TabPaths,
	}
	for _, expected := range tabs {
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		cm, ok := updated.(tui.ConfigModel)
		if !ok {
			t.Fatalf("expected ConfigModel type from Update")
		}
		m = cm
		if m.ActiveTab() != expected {
			t.Fatalf("expected tab %v after Tab, got %v", expected, m.ActiveTab())
		}
		if m.Cursor() != 0 {
			t.Errorf("expected cursor reset to 0 after tab switch, got %d", m.Cursor())
		}
	}

	// Shift+Tab goes backward (0 -> 4)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = updated.(tui.ConfigModel)
	if m.ActiveTab() != tui.TabScheduler {
		t.Fatalf("expected TabScheduler after Shift+Tab, got %v", m.ActiveTab())
	}

	// Number keys 1-5 direct jumps
	jumps := []struct {
		key      rune
		expected tui.ConfigTab
	}{
		{'1', tui.TabPaths},
		{'2', tui.TabEcosystems},
		{'3', tui.TabSafety},
		{'4', tui.TabWatch},
		{'5', tui.TabScheduler},
	}
	for _, tc := range jumps {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{tc.key}})
		m = updated.(tui.ConfigModel)
		if m.ActiveTab() != tc.expected {
			t.Errorf("expected tab %v after key %c, got %v", tc.expected, tc.key, m.ActiveTab())
		}
	}
}

func TestConfigTUI_VerticalNavigation(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Switch to TabEcosystems (18 items)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(tui.ConfigModel)

	// Set limited window size
	updated, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: 15})
	m = updated.(tui.ConfigModel)

	if m.Cursor() != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.Cursor())
	}

	// Up / 'k' at top should not go below 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updated.(tui.ConfigModel)
	if m.Cursor() != 0 {
		t.Fatalf("expected cursor to remain 0, got %d", m.Cursor())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	m = updated.(tui.ConfigModel)
	if m.Cursor() != 0 {
		t.Fatalf("expected cursor to remain 0 after 'k', got %d", m.Cursor())
	}

	// Down / 'j' navigates down
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = updated.(tui.ConfigModel)
	if m.Cursor() != 1 {
		t.Fatalf("expected cursor 1 after Down, got %d", m.Cursor())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updated.(tui.ConfigModel)
	if m.Cursor() != 2 {
		t.Fatalf("expected cursor 2 after 'j', got %d", m.Cursor())
	}

	// Repeated 'j' should clamp to maxCursor (17 for 18 ecosystems)
	for i := 0; i < 25; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = updated.(tui.ConfigModel)
	}
	if m.Cursor() != 17 {
		t.Fatalf("expected cursor clamped at 17, got %d", m.Cursor())
	}

	// Repeated 'k' should return to 0
	for i := 0; i < 25; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		m = updated.(tui.ConfigModel)
	}
	if m.Cursor() != 0 {
		t.Fatalf("expected cursor back at 0, got %d", m.Cursor())
	}
}

func TestConfigTUI_BooleanToggles(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Initially clean
	if m.IsDirty() {
		t.Fatalf("expected initial IsDirty to be false")
	}

	// Switch to TabEcosystems
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(tui.ConfigModel)

	// Ecosystem 0 is Docker (enabled by default)
	if !cfg.Ecosystems.Docker.Enabled {
		t.Fatalf("expected Docker enabled by default")
	}

	// Toggle Docker off with Space
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(tui.ConfigModel)
	if cfg.Ecosystems.Docker.Enabled {
		t.Fatalf("expected Docker to be disabled after Space toggle")
	}
	if !m.IsDirty() {
		t.Fatalf("expected IsDirty to be true after toggle")
	}

	// Toggle Docker back on
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(tui.ConfigModel)
	if !cfg.Ecosystems.Docker.Enabled {
		t.Fatalf("expected Docker to be re-enabled after second Space toggle")
	}

	// Switch to TabSafety (Tab index 2, shortcut '3')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(tui.ConfigModel)

	// Item 0 is Safety.TrashEnabled (true by default)
	if !cfg.Safety.TrashEnabled {
		t.Fatalf("expected Safety.TrashEnabled to be true by default")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(tui.ConfigModel)
	if cfg.Safety.TrashEnabled {
		t.Fatalf("expected Safety.TrashEnabled to be false after toggle")
	}

	// Switch to TabWatch (Tab index 3, shortcut '4')
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m = updated.(tui.ConfigModel)

	// Item 0 is Watch Daemon toggle
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(tui.ConfigModel)
	if !m.IsDirty() {
		t.Fatalf("expected IsDirty to be true after watch toggle")
	}
}

func TestConfigTUI_WindowSize(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	initCmd := m.Init()
	_ = initCmd // can be nil

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.ConfigModel)

	view := m.View()
	if view == "" {
		t.Fatalf("expected non-empty View output")
	}
}

func TestConfigTUI_InlineTextInputEditing(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Switch to TabEcosystems
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(tui.ConfigModel)

	if cfg.Ecosystems.Docker.MaxAgeDays != 30 {
		t.Fatalf("expected Docker max_age_days 30, got %d", cfg.Ecosystems.Docker.MaxAgeDays)
	}

	// Press 'e' to start inline editing
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(tui.ConfigModel)

	if m.State() != tui.ConfigStateEditingField {
		t.Fatalf("expected ConfigStateEditingField, got %v", m.State())
	}

	// Clear value with backspaces and type "45"
	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(tui.ConfigModel)
	}
	for _, r := range "45" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}

	// Confirm with Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)

	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected return to ConfigStateNormal, got %v", m.State())
	}
	if cfg.Ecosystems.Docker.MaxAgeDays != 45 {
		t.Fatalf("expected Docker max_age_days updated to 45, got %d", cfg.Ecosystems.Docker.MaxAgeDays)
	}
	if !m.IsDirty() {
		t.Fatalf("expected IsDirty to be true after edit")
	}

	// Test Esc cancellation
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(tui.ConfigModel)
	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(tui.ConfigModel)
	}
	for _, r := range "999" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(tui.ConfigModel)

	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected return to ConfigStateNormal after Esc, got %v", m.State())
	}
	if cfg.Ecosystems.Docker.MaxAgeDays != 45 {
		t.Fatalf("expected Docker max_age_days kept at 45 after Esc, got %d", cfg.Ecosystems.Docker.MaxAgeDays)
	}

	// Test invalid input rejection (e.g. "0" for max_age_days)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = updated.(tui.ConfigModel)
	for i := 0; i < 5; i++ {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		m = updated.(tui.ConfigModel)
	}
	for _, r := range "0" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)

	if m.State() != tui.ConfigStateEditingField {
		t.Fatalf("expected state to remain ConfigStateEditingField for invalid 0, got %v", m.State())
	}
	if !strings.Contains(m.StatusMessage(), "inválido") && !strings.Contains(m.StatusMessage(), "> 0") {
		t.Errorf("expected error status message, got: %s", m.StatusMessage())
	}

	// Cancel invalid edit
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(tui.ConfigModel)
}

func TestConfigTUI_SaveValidationBlock(t *testing.T) {
	cfg := config.Defaults()
	cfg.Safety.RetentionDays = 0 // Invalid: must be >= 1
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Attempt to save with Ctrl+S
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	m = updated.(tui.ConfigModel)

	if !strings.Contains(m.StatusMessage(), "safety.retention_days: must be >= 1") {
		t.Fatalf("expected validation failure message for retention_days, got: %s", m.StatusMessage())
	}
}

func TestConfigTUI_PathManagement_BlacklistValidation(t *testing.T) {
	cfg := config.Defaults()
	bl := safety.NewBlacklist(nil)
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, bl)

	// Injects 'a' to add path
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateAddingPath {
		t.Fatalf("expected ConfigStateAddingPath, got %v", m.State())
	}

	// Attempt to add forbidden path /etc (system root)
	for _, r := range "/etc" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)

	// Must be blocked by Blacklist (NFR-03)
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected return to ConfigStateNormal, got %v", m.State())
	}
	for _, p := range cfg.ScanPaths {
		if p == "/etc" {
			t.Errorf("forbidden path /etc was illegally added to ScanPaths")
		}
	}
	if !strings.Contains(m.StatusMessage(), "proibido pela Blacklist Imutável") {
		t.Errorf("expected blacklist rejection status message, got: %s", m.StatusMessage())
	}

	// Attempt to add ~/.ssh
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(tui.ConfigModel)
	for _, r := range "~/.ssh" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)
	for _, p := range cfg.ScanPaths {
		if p == "~/.ssh" {
			t.Errorf("forbidden path ~/.ssh was illegally added to ScanPaths")
		}
	}
	if !strings.Contains(m.StatusMessage(), "proibido pela Blacklist Imutável") {
		t.Errorf("expected blacklist rejection for ~/.ssh, got: %s", m.StatusMessage())
	}

	// Attempt to add .git
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(tui.ConfigModel)
	for _, r := range ".git" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)
	for _, p := range cfg.ScanPaths {
		if p == ".git" {
			t.Errorf("forbidden path .git was illegally added to ScanPaths")
		}
	}
	if !strings.Contains(m.StatusMessage(), "proibido pela Blacklist Imutável") {
		t.Errorf("expected blacklist rejection for .git, got: %s", m.StatusMessage())
	}

	// Add legitimate path
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = updated.(tui.ConfigModel)
	for _, r := range "/home/developer/workspace" {
		updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = updated.(tui.ConfigModel)
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.ConfigModel)

	found := false
	for _, p := range cfg.ScanPaths {
		if p == "/home/developer/workspace" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected /home/developer/workspace to be added to ScanPaths")
	}
	if !m.IsDirty() {
		t.Fatalf("expected IsDirty to be true after adding path")
	}

	// Delete path with 'd'
	m = tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, bl)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(tui.ConfigModel)
	for _, p := range cfg.ScanPaths {
		if p == "/home/developer/workspace" {
			t.Fatalf("expected /home/developer/workspace to be removed after 'd'")
		}
	}
	if !m.IsDirty() {
		t.Fatalf("expected IsDirty to be true after deleting path")
	}
}

func TestConfigTUI_DirtyExitConfirmation(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Clean model: 'q' exits immediately
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatalf("expected quit command on clean exit")
	}
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected ConfigStateNormal on clean quit, got %v", m.State())
	}

	// Toggle a boolean to make dirty
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(tui.ConfigModel)
	// Switch to TabEcosystems to toggle
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updated.(tui.ConfigModel)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = updated.(tui.ConfigModel)
	if !m.IsDirty() {
		t.Fatalf("expected isDirty == true after toggle")
	}

	// 'q' on dirty model enters confirmation state (D-04)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateConfirmExit {
		t.Fatalf("expected ConfigStateConfirmExit, got %v", m.State())
	}
	if !strings.Contains(m.View(), "MODIFICAÇÕES PENDENTES") {
		t.Errorf("expected view to contain MODIFICAÇÕES PENDENTES, got: %s", m.View())
	}

	// 'c' cancels and returns to normal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected ConfigStateNormal after cancel, got %v", m.State())
	}
	if !m.IsDirty() {
		t.Fatalf("expected isDirty to remain true after cancel")
	}

	// 'Esc' on dirty enters confirm, 'd' discards and exits
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateConfirmExit {
		t.Fatalf("expected ConfigStateConfirmExit after Esc, got %v", m.State())
	}
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatalf("expected quit command on discard 'd'")
	}

	// Save and quit with 's'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(tui.ConfigModel)
	_, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd == nil {
		t.Fatalf("expected quit command on save 's'")
	}
}

func TestConfigTUI_HelpOverlay(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)

	// Press '?' to open help modal
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateHelp {
		t.Fatalf("expected ConfigStateHelp, got %v", m.State())
	}
	if !strings.Contains(m.View(), "AJUDA - ATALHOS DO DEVCLEAN CONFIG") {
		t.Errorf("expected view to contain help title, got: %s", m.View())
	}

	// Press 'Esc' to exit help
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected ConfigStateNormal after Esc, got %v", m.State())
	}

	// Press '?' then '?' to toggle help off
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateHelp {
		t.Fatalf("expected ConfigStateHelp, got %v", m.State())
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(tui.ConfigModel)
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected ConfigStateNormal after second '?', got %v", m.State())
	}
}

func updateModel(m tui.ConfigModel, msg tea.Msg) tui.ConfigModel {
	updated, _ := m.Update(msg)
	return updated.(tui.ConfigModel)
}

func TestConfigTUI_ViewAllTabs(t *testing.T) {
	cfg := config.Defaults()
	cfg.ScanPaths = []string{"/tmp/test1"}
	cfg.ExcludePaths = []string{"/tmp/ex1"}
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 20})

	// TabPaths view
	v0 := m.View()
	if !strings.Contains(v0, "/tmp/test1") || !strings.Contains(v0, "/tmp/ex1") {
		t.Errorf("expected TabPaths to render paths, got: %s", v0)
	}

	// TabEcosystems view
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	v1 := m.View()
	if !strings.Contains(v1, "Docker") || !strings.Contains(v1, "Node.js") {
		t.Errorf("expected TabEcosystems view, got: %s", v1)
	}

	// TabSafety view
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	v2 := m.View()
	if !strings.Contains(v2, "retention_days") {
		t.Errorf("expected TabSafety view, got: %s", v2)
	}

	// TabWatch view
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	v3 := m.View()
	if !strings.Contains(v3, "watch.interval") {
		t.Errorf("expected TabWatch view, got: %s", v3)
	}

	// TabScheduler view
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	v4 := m.View()
	if !strings.Contains(v4, "Agendador") {
		t.Errorf("expected TabScheduler view, got: %s", v4)
	}

	// Status messages formatting check
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	vSaved := m.View()
	if !strings.Contains(vSaved, "✓") {
		t.Errorf("expected success status check in view, got: %s", vSaved)
	}
}

func TestConfigTUI_EditAllTabsAndFields(t *testing.T) {
	cfg := config.Defaults()
	cfg.ScanPaths = []string{"/home/dev/scan1"}
	cfg.ExcludePaths = []string{"/home/dev/ex1"}
	bl := safety.NewBlacklist(nil)
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, bl)

	// 1. TabPaths: edit scan path
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 20; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "/home/dev/scan_updated" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.ScanPaths[0] != "/home/dev/scan_updated" {
		t.Errorf("expected updated scan path, got: %s", cfg.ScanPaths[0])
	}

	// Try editing to forbidden path /etc -> must be rejected
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 30; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "/etc" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.ScanPaths[0] == "/etc" {
		t.Errorf("forbidden /etc was improperly accepted")
	}
	// Cancel rejected edit to return to ConfigStateNormal
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})

	// Edit exclude path: move cursor down to index 1
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 20; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "/home/dev/ex_updated" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.ExcludePaths[0] != "/home/dev/ex_updated" {
		t.Errorf("expected updated exclude path, got: %s", cfg.ExcludePaths[0])
	}

	// Delete exclude path with 'd'
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if len(cfg.ExcludePaths) != 0 {
		t.Errorf("expected excludePaths empty after d, got: %v", cfg.ExcludePaths)
	}

	// 2. TabSafety: edit retention_days, max_size_mb, mass_select, history
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	// retention_days (cursor 1)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 5; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "60" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.Safety.RetentionDays != 60 {
		t.Errorf("expected retention_days 60, got: %d", cfg.Safety.RetentionDays)
	}

	// max_size_mb (cursor 2)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "20480" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.Safety.MaxSizeMB != 20480 {
		t.Errorf("expected max_size_mb 20480, got: %d", cfg.Safety.MaxSizeMB)
	}

	// mass_select (cursor 3)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "4096" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.TUI.MassSelectThresholdMB != 4096 {
		t.Errorf("expected mass_select 4096, got: %d", cfg.TUI.MassSelectThresholdMB)
	}

	// history retention (cursor 4)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "180" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.History.RetentionDays != 180 {
		t.Errorf("expected history retention 180, got: %d", cfg.History.RetentionDays)
	}

	// 3. TabWatch: edit interval and threshold_mb
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	// interval (cursor 1)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 5; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "12h" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.Watch.Interval != "12h" {
		t.Errorf("expected watch interval 12h, got: %s", cfg.Watch.Interval)
	}

	// threshold_mb (cursor 2)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "5120" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if cfg.Watch.ThresholdMB != 5120 {
		t.Errorf("expected watch threshold 5120, got: %d", cfg.Watch.ThresholdMB)
	}

	// 4. TabScheduler: toggle & edit freq, at_time, preset, cron
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	// freq (cursor 1): Space cycles
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace}) // weekly
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace}) // hourly
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace}) // custom
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace}) // daily
	// edit freq to weekly
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "weekly" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	// at_time (cursor 2): edit to 14:30
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 10; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "14:30" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	// preset (cursor 3): Space cycles
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})
	// edit preset to standard
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 15; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "standard" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	// cron (cursor 4): edit to "0 1 * * *"
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	for i := 0; i < 20; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "0 1 * * *" {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
}

func TestConfigTUI_SaveWithAST(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "devclean-config-test-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + ".bak")

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(config.DefaultConfigYAML), &root); err != nil {
		t.Fatalf("unmarshal default yaml: %v", err)
	}

	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, &root, tmpPath, nil, nil)

	// Toggle something and save
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace}) // toggle docker

	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlS})
	if m.IsDirty() {
		t.Errorf("expected isDirty to be false after save")
	}

	// Verify file exists and is valid yaml
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		t.Fatalf("reading saved config: %v", err)
	}
	if !strings.Contains(string(data), "docker:") {
		t.Errorf("saved config does not contain docker: %s", string(data))
	}
}

func TestConfigTUI_EcosystemsCoverage(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})

	// Navigate and toggle all 18 ecosystems
	for i := 0; i < 18; i++ {
		m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})
		m = updateModel(m, tea.KeyMsg{Type: tea.KeyDown})
	}
	if !m.IsDirty() {
		t.Fatalf("expected dirty after toggling ecosystems")
	}
}

func TestConfigTUI_OSErrorModal(t *testing.T) {
	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, nil, "/tmp/cfg.yaml", nil, nil)
	m = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Make dirty
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})

	// Corrupt config so save fails
	cfg.Safety.RetentionDays = 0

	// Press 'q' to open exit confirm
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.State() != tui.ConfigStateConfirmExit {
		t.Fatalf("expected ConfigStateConfirmExit, got %v", m.State())
	}

	// Press 's' to attempt save -> fails and enters ConfigStateOSError
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if m.State() != tui.ConfigStateOSError {
		t.Fatalf("expected ConfigStateOSError, got %v", m.State())
	}

	// Verify view contains error modal
	v := m.View()
	if !strings.Contains(v, "FALHA NA OPERAÇÃO DO SISTEMA") {
		t.Errorf("expected OS error modal title, got: %s", v)
	}

	// Press Enter to dismiss OS error modal
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected ConfigStateNormal after Enter on OSError, got %v", m.State())
	}
}

type mockScheduler struct {
	installed          bool
	watchInstalled     bool
	status             *scheduler.Status
	installErr         error
	removeErr          error
	installWatchErr    error
	removeWatchErr     error
	installCalled      bool
	removeCalled       bool
	installWatchCalled bool
	removeWatchCalled  bool
	lastSchedule       scheduler.Schedule
	lastWatchSchedule  scheduler.WatchSchedule
}

func (m *mockScheduler) Install(s scheduler.Schedule) error {
	m.installCalled = true
	m.lastSchedule = s
	if m.installErr != nil {
		return m.installErr
	}
	m.installed = true
	return nil
}

func (m *mockScheduler) Remove() error {
	m.removeCalled = true
	if m.removeErr != nil {
		return m.removeErr
	}
	m.installed = false
	return nil
}

func (m *mockScheduler) Status() (*scheduler.Status, error) {
	if m.status != nil {
		return m.status, nil
	}
	return &scheduler.Status{Active: m.installed}, nil
}

func (m *mockScheduler) IsInstalled() bool {
	return m.installed
}

func (m *mockScheduler) InstallWatch(ws scheduler.WatchSchedule) error {
	m.installWatchCalled = true
	m.lastWatchSchedule = ws
	if m.installWatchErr != nil {
		return m.installWatchErr
	}
	m.watchInstalled = true
	return nil
}

func (m *mockScheduler) RemoveWatch() error {
	m.removeWatchCalled = true
	if m.removeWatchErr != nil {
		return m.removeWatchErr
	}
	m.watchInstalled = false
	return nil
}

func (m *mockScheduler) IsWatchInstalled() bool {
	return m.watchInstalled
}

func TestConfigTUI_OSSync_Success(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "devclean-ossync-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + ".bak")

	if err := os.WriteFile(tmpPath, []byte(config.DefaultConfigYAML), 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal([]byte(config.DefaultConfigYAML), &root); err != nil {
		t.Fatalf("unmarshal default yaml: %v", err)
	}

	mockSched := &mockScheduler{
		installed:      false,
		watchInstalled: false,
	}

	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, &root, tmpPath, mockSched, nil)

	// Switch to TabScheduler (5)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	// Toggle scheduler active with space
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})

	// Switch to TabWatch (4)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	// Toggle watch active with space
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})

	if !m.IsDirty() {
		t.Fatalf("expected model to be dirty before save")
	}

	// Trigger Save with Ctrl+S
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if !mockSched.installCalled {
		t.Errorf("expected scheduler.Install to be called")
	}
	if !mockSched.installWatchCalled {
		t.Errorf("expected scheduler.InstallWatch to be called")
	}
	if m.IsDirty() {
		t.Errorf("expected model isDirty to be false after successful save")
	}
	if !strings.Contains(m.StatusMessage(), "sincronizados com sucesso") {
		t.Errorf("expected success status message, got: %s", m.StatusMessage())
	}
}

func TestConfigTUI_OSSyncFailureModal(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "devclean-ossync-fail-*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)
	defer os.Remove(tmpPath + ".bak")

	initialContent := []byte(config.DefaultConfigYAML)
	if err := os.WriteFile(tmpPath, initialContent, 0644); err != nil {
		t.Fatalf("failed to write initial config: %v", err)
	}

	var root yaml.Node
	if err := yaml.Unmarshal(initialContent, &root); err != nil {
		t.Fatalf("unmarshal default yaml: %v", err)
	}

	simulatedErr := errors.New("launchctl: service already loaded")
	mockSched := &mockScheduler{
		installed:      false,
		watchInstalled: false,
		installErr:     simulatedErr,
	}

	cfg := config.Defaults()
	m := tui.NewConfigModel(&cfg, &root, tmpPath, mockSched, nil)

	// Switch to TabScheduler (5)
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	// Toggle scheduler active
	m = updateModel(m, tea.KeyMsg{Type: tea.KeySpace})

	if !m.IsDirty() {
		t.Fatalf("expected model to be dirty")
	}

	// Trigger Save with Ctrl+S
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlS})

	if m.State() != tui.ConfigStateOSError {
		t.Fatalf("expected state ConfigStateOSError, got %v", m.State())
	}
	v := m.View()
	if !strings.Contains(v, "launchctl: service already loaded") {
		t.Errorf("expected OS error message in view, got: %s", v)
	}
	if !m.IsDirty() {
		t.Errorf("expected model to remain dirty on OS failure")
	}

	// Dismiss modal with Enter
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	if m.State() != tui.ConfigStateNormal {
		t.Fatalf("expected state ConfigStateNormal after dismissing modal, got %v", m.State())
	}
	if !m.IsDirty() {
		t.Errorf("expected model to still remain dirty after dismissing modal")
	}

	// Verify file on disk was NOT overwritten
	contentAfter, _ := os.ReadFile(tmpPath)
	if string(contentAfter) != string(initialContent) {
		t.Errorf("expected config file on disk to remain unchanged")
	}
}

