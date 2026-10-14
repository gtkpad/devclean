package tui_test

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/tui"
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
