package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/tui"
)

func TestAnalyzer_VimNavigation(t *testing.T) {
	root := &analyzer.Node{
		Path:      "/test",
		Name:      "test",
		Type:      analyzer.ItemDir,
		SizeBytes: 1000,
		Children: []*analyzer.Node{
			{Path: "/test/dirA", Name: "dirA", Type: analyzer.ItemDir, SizeBytes: 700, ItemCount: 5},
			{Path: "/test/dirB", Name: "dirB", Type: analyzer.ItemDir, SizeBytes: 300, ItemCount: 2},
		},
	}
	root.Children[0].Parent = root
	root.Children[1].Parent = root

	m := tui.NewAnalyzerModel(root, nil, nil, 80, 24)

	if m.Cursor() != 0 {
		t.Fatalf("expected initial cursor 0, got %d", m.Cursor())
	}

	// Move cursor down with 'j'
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	am := updated.(tui.AnalyzerModel)
	if am.Cursor() != 1 {
		t.Fatalf("expected cursor 1 after 'j', got %d", am.Cursor())
	}

	// Move cursor up with 'k'
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	am = updated.(tui.AnalyzerModel)
	if am.Cursor() != 0 {
		t.Fatalf("expected cursor 0 after 'k', got %d", am.Cursor())
	}

	// Down arrow
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyDown})
	am = updated.(tui.AnalyzerModel)
	if am.Cursor() != 1 {
		t.Fatalf("expected cursor 1 after Down arrow, got %d", am.Cursor())
	}

	// Enter directory with 'l'
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	am = updated.(tui.AnalyzerModel)
	if am.CurrentDir().Path != "/test/dirB" {
		t.Fatalf("expected currentDir /test/dirB after 'l', got %s", am.CurrentDir().Path)
	}
	if am.Cursor() != 0 {
		t.Fatalf("expected cursor reset to 0 in new dir, got %d", am.Cursor())
	}

	// Ascend with 'h'
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	am = updated.(tui.AnalyzerModel)
	if am.CurrentDir().Path != "/test" {
		t.Fatalf("expected return to /test after 'h', got %s", am.CurrentDir().Path)
	}

	// Enter with Enter key on dirA (cursor 0)
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyEnter})
	am = updated.(tui.AnalyzerModel)
	if am.CurrentDir().Path != "/test/dirA" {
		t.Fatalf("expected currentDir /test/dirA after Enter, got %s", am.CurrentDir().Path)
	}

	// Ascend with Backspace
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	am = updated.(tui.AnalyzerModel)
	if am.CurrentDir().Path != "/test" {
		t.Fatalf("expected return to /test after Backspace, got %s", am.CurrentDir().Path)
	}
}

func TestAnalyzer_Sorting(t *testing.T) {
	root := &analyzer.Node{
		Path:      "/sort-test",
		Name:      "sort-test",
		Type:      analyzer.ItemDir,
		SizeBytes: 900,
		Children: []*analyzer.Node{
			{Path: "/sort-test/beta", Name: "beta", Type: analyzer.ItemFile, SizeBytes: 100, ItemCount: 10},
			{Path: "/sort-test/alpha", Name: "alpha", Type: analyzer.ItemFile, SizeBytes: 500, ItemCount: 2},
			{Path: "/sort-test/gamma", Name: "gamma", Type: analyzer.ItemFile, SizeBytes: 300, ItemCount: 5},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	m := tui.NewAnalyzerModel(root, nil, nil, 80, 24)

	// Default sort mode: SortBySizeDesc
	if m.SortMode() != tui.SortBySizeDesc {
		t.Fatalf("expected default SortBySizeDesc, got %v", m.SortMode())
	}
	children := m.CurrentDir().Children
	if children[0].Name != "alpha" || children[1].Name != "gamma" || children[2].Name != "beta" {
		t.Fatalf("unexpected order for SizeDesc: %s, %s, %s", children[0].Name, children[1].Name, children[2].Name)
	}

	// Cycle sort with 's' -> SortByNameAsc
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	am := updated.(tui.AnalyzerModel)
	if am.SortMode() != tui.SortByNameAsc {
		t.Fatalf("expected SortByNameAsc, got %v", am.SortMode())
	}
	children = am.CurrentDir().Children
	if children[0].Name != "alpha" || children[1].Name != "beta" || children[2].Name != "gamma" {
		t.Fatalf("unexpected order for NameAsc: %s, %s, %s", children[0].Name, children[1].Name, children[2].Name)
	}

	// Cycle sort with 's' -> SortByItemsDesc
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	am = updated.(tui.AnalyzerModel)
	if am.SortMode() != tui.SortByItemsDesc {
		t.Fatalf("expected SortByItemsDesc, got %v", am.SortMode())
	}
	children = am.CurrentDir().Children
	if children[0].Name != "beta" || children[1].Name != "gamma" || children[2].Name != "alpha" {
		t.Fatalf("unexpected order for ItemsDesc: %s, %s, %s", children[0].Name, children[1].Name, children[2].Name)
	}

	// Cycle sort with 's' -> SortBySizeDesc again
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	am = updated.(tui.AnalyzerModel)
	if am.SortMode() != tui.SortBySizeDesc {
		t.Fatalf("expected cycle back to SortBySizeDesc, got %v", am.SortMode())
	}
}

func TestAnalyzer_ProportionBars(t *testing.T) {
	// Full width 10
	barFull := tui.RenderProportionBar(1000, 1000, 10)
	if strings.Count(barFull, "█") != 10 {
		t.Fatalf("expected 10 full blocks, got: %s", barFull)
	}

	// Half width 10
	barHalf := tui.RenderProportionBar(500, 1000, 10)
	if strings.Count(barHalf, "█") != 5 {
		t.Fatalf("expected 5 full blocks, got: %s", barHalf)
	}

	// Zero size
	barZero := tui.RenderProportionBar(0, 1000, 10)
	if strings.Count(barZero, "█") != 0 {
		t.Fatalf("expected 0 full blocks, got: %s", barZero)
	}

	// Max 0 fallback
	barZeroMax := tui.RenderProportionBar(100, 0, 10)
	if strings.Count(barZeroMax, "█") != 0 {
		t.Fatalf("expected 0 full blocks when max is 0, got: %s", barZeroMax)
	}
}

func TestAnalyzer_LargeFilesPanel(t *testing.T) {
	root := &analyzer.Node{
		Path:      "/large-test",
		Name:      "large-test",
		Type:      analyzer.ItemDir,
		SizeBytes: 200 * 1024 * 1024,
		Children: []*analyzer.Node{
			{
				Path:        "/large-test/big.iso",
				Name:        "big.iso",
				Type:        analyzer.ItemFile,
				SizeBytes:   150 * 1024 * 1024,
				IsLargeFile: true,
			},
		},
	}
	root.Children[0].Parent = root

	m := tui.NewAnalyzerModel(root, nil, nil, 80, 24)

	// Press 'f' to toggle large files
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	am := updated.(tui.AnalyzerModel)
	if am.State() != tui.StateLargeFiles {
		t.Fatalf("expected StateLargeFiles, got %v", am.State())
	}
	view := am.View()
	if !strings.Contains(view, "big.iso") {
		t.Fatalf("expected large files view to display big.iso, got: %s", view)
	}

	// Press 'esc' to return to normal
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am = updated.(tui.AnalyzerModel)
	if am.State() != tui.StateNormal {
		t.Fatalf("expected return to StateNormal, got %v", am.State())
	}

	// Test Tab key toggle
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyTab})
	am = updated.(tui.AnalyzerModel)
	if am.State() != tui.StateLargeFiles {
		t.Fatalf("expected StateLargeFiles via Tab, got %v", am.State())
	}
}

func TestAnalyzer_HelpOverlay(t *testing.T) {
	root := &analyzer.Node{
		Path: "/help-test",
		Name: "help-test",
		Type: analyzer.ItemDir,
	}

	m := tui.NewAnalyzerModel(root, nil, nil, 80, 24)

	// Press '?' to toggle help
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	am := updated.(tui.AnalyzerModel)
	if am.State() != tui.StateHelp {
		t.Fatalf("expected StateHelp, got %v", am.State())
	}
	view := am.View()
	if !strings.Contains(view, "Ajuda") && !strings.Contains(view, "Vim") {
		t.Fatalf("expected help content in view, got: %s", view)
	}

	// Press 'esc' to dismiss
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyEsc})
	am = updated.(tui.AnalyzerModel)
	if am.State() != tui.StateNormal {
		t.Fatalf("expected return to StateNormal, got %v", am.State())
	}

	// Press 'i' to toggle help
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	am = updated.(tui.AnalyzerModel)
	if am.State() != tui.StateHelp {
		t.Fatalf("expected StateHelp via 'i', got %v", am.State())
	}
}

func TestAnalyzer_ProtectedBadge(t *testing.T) {
	bl := safety.NewBlacklist(nil)
	root := &analyzer.Node{
		Path:      "/project",
		Name:      "project",
		Type:      analyzer.ItemDir,
		SizeBytes: 1024,
		Children: []*analyzer.Node{
			{Path: "/project/.git", Name: ".git", Type: analyzer.ItemDir, IsProtected: true},
			{Path: "/project/.env", Name: ".env", Type: analyzer.ItemFile, IsProtected: true},
			{Path: "/project/normal", Name: "normal", Type: analyzer.ItemDir, IsProtected: false},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	m := tui.NewAnalyzerModel(root, bl, nil, 100, 30)
	view := m.View()

	if !strings.Contains(view, "[🔒 PROTEGIDO]") && !strings.Contains(view, "[PROTEGIDO]") {
		t.Fatalf("expected view to contain protected badge, got: %s", view)
	}
}

func TestAnalyzer_BlacklistBlockDeletion(t *testing.T) {
	bl := safety.NewBlacklist(nil)
	root := &analyzer.Node{
		Path:      "/repo",
		Name:      "repo",
		Type:      analyzer.ItemDir,
		SizeBytes: 2048,
		Children: []*analyzer.Node{
			{Path: "/repo/.git", Name: ".git", Type: analyzer.ItemDir, IsProtected: true},
			{Path: "/repo/target", Name: "target", Type: analyzer.ItemDir, IsProtected: false},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	m := tui.NewAnalyzerModel(root, bl, nil, 100, 30)

	// Cursor is at 0 (.git)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am := updated.(tui.AnalyzerModel)

	if am.IsConfirmingDelete() {
		t.Fatal("expected deletion modal to remain closed for protected item")
	}

	if !strings.Contains(am.StatusMessage(), "Item protegido pela Blacklist Imutável") {
		t.Fatalf("expected status message warning, got: %s", am.StatusMessage())
	}

	view := am.View()
	if !strings.Contains(view, "Item protegido pela Blacklist Imutável") {
		t.Fatalf("expected protection warning in view, got: %s", view)
	}
}

func TestAnalyzer_DeleteModalFlow(t *testing.T) {
	root := &analyzer.Node{
		Path:      "/clean-test",
		Name:      "clean-test",
		Type:      analyzer.ItemDir,
		SizeBytes: 5000,
		Children: []*analyzer.Node{
			{Path: "/clean-test/build", Name: "build", Type: analyzer.ItemDir, SizeBytes: 5000, IsProtected: false},
		},
	}
	root.Children[0].Parent = root

	m := tui.NewAnalyzerModel(root, nil, nil, 100, 30)

	// Press 'd' on unprotected item
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am := updated.(tui.AnalyzerModel)

	if !am.IsConfirmingDelete() {
		t.Fatal("expected IsConfirmingDelete to be true")
	}

	view := am.View()
	if !strings.Contains(view, "CONFIRMAR EXCLUSÃO PARA A LIXEIRA") {
		t.Fatalf("expected delete modal title in view, got: %s", view)
	}
	if !strings.Contains(view, "/clean-test/build") {
		t.Fatalf("expected target path in delete modal view, got: %s", view)
	}

	// Press 'n' to cancel
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	am = updated.(tui.AnalyzerModel)

	if am.IsConfirmingDelete() {
		t.Fatal("expected IsConfirmingDelete to be false after 'n'")
	}
	if am.StatusMessage() != "Exclusão cancelada." {
		t.Fatalf("expected status 'Exclusão cancelada.', got: %s", am.StatusMessage())
	}

	// Press 'd' again and confirm with 'y'
	updated, _ = am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am = updated.(tui.AnalyzerModel)

	updated, cmd := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am = updated.(tui.AnalyzerModel)

	if am.IsConfirmingDelete() {
		t.Fatal("expected modal closed after 'y'")
	}

	if cmd == nil {
		t.Fatal("expected delete command to be returned")
	}

	// Execute delete cmd
	msg := cmd()
	if msg == nil {
		t.Fatal("expected non-nil message from delete command")
	}

	updated, _ = am.Update(msg)
	am = updated.(tui.AnalyzerModel)

	if len(am.CurrentDir().Children) != 0 {
		t.Fatalf("expected 0 children after delete, got %d", len(am.CurrentDir().Children))
	}
	if !strings.Contains(am.StatusMessage(), "sucesso") {
		t.Fatalf("expected success message, got: %s", am.StatusMessage())
	}
}

