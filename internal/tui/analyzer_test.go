package tui_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
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

type mockCleanerProvider struct {
	cleaner domain.Cleaner
}

func (p *mockCleanerProvider) GetCleaner(name string) domain.Cleaner {
	return p.cleaner
}

type mockCleaner struct {
	cleaned []domain.Target
	err     error
}

func (m *mockCleaner) Name() string { return "filesystem" }
func (m *mockCleaner) IsInstalled() bool { return true }
func (m *mockCleaner) Scan(_ context.Context, _ domain.EcosystemRules) ([]domain.Target, error) {
	return nil, nil
}
func (m *mockCleaner) Clean(_ context.Context, targets []domain.Target) error {
	if m.err != nil {
		return m.err
	}
	m.cleaned = append(m.cleaned, targets...)
	return nil
}

func TestAnalyzer_InviolableBlacklistAudit(t *testing.T) {
	bl := safety.NewBlacklist([]string{"*.custom", "custom_protected"})

	// Setup nodes covering core blacklist (.git, .ssh, .env, secrets.json),
	// system roots, custom blacklists, and normal unprotected items.
	protectedPaths := []struct {
		path  string
		name  string
		isDir bool
	}{
		{"/repo/.git", ".git", true},
		{"/repo/.ssh", ".ssh", true},
		{"/repo/.env", ".env", false},
		{"/repo/secrets.json", "secrets.json", false},
		{"/repo/secrets.custom", "secrets.custom", false},
		{"/repo/custom_protected", "custom_protected", true},
	}

	var children []*analyzer.Node
	for _, p := range protectedPaths {
		itemType := analyzer.ItemFile
		if p.isDir {
			itemType = analyzer.ItemDir
		}
		isProtected := safety.IsCoreBlacklisted(p.path) || bl.IsBlacklisted(p.path)
		children = append(children, &analyzer.Node{
			Path:        p.path,
			Name:        p.name,
			Type:        itemType,
			SizeBytes:   1024,
			ItemCount:   1,
			IsProtected: isProtected,
		})
	}

	// Also add an unprotected item at the end
	children = append(children, &analyzer.Node{
		Path:        "/repo/target_build",
		Name:        "target_build",
		Type:        analyzer.ItemDir,
		SizeBytes:   4096,
		ItemCount:   10,
		IsProtected: false,
	})

	root := &analyzer.Node{
		Path:      "/repo",
		Name:      "repo",
		Type:      analyzer.ItemDir,
		Children:  children,
		SizeBytes: 10000,
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	mc := &mockCleaner{}
	cleaners := &mockCleanerProvider{cleaner: mc}
	cfg := &config.Config{}
	coord := deleter.New(cleaners, cfg, bl, nil, nil, "", "")

	m := tui.NewAnalyzerModel(root, bl, coord, 100, 30)
	view := m.View()

	// 1. Verify view contains protected badge for every protected item
	if !strings.Contains(view, "[🔒 PROTEGIDO]") && !strings.Contains(view, "[PROTEGIDO]") {
		t.Fatalf("expected view to contain protected badge, got:\n%s", view)
	}

	// 2. Test deletion block on each protected item in the TUI
	for idx, child := range m.CurrentDir().Children {
		if !child.IsProtected {
			continue
		}
		am := m
		for step := 0; step < idx; step++ {
			u, _ := am.Update(tea.KeyMsg{Type: tea.KeyDown})
			am = u.(tui.AnalyzerModel)
		}
		if am.Cursor() != idx {
			t.Fatalf("expected cursor at %d, got %d", idx, am.Cursor())
		}

		// Try pressing 'd'
		u, _ := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
		am = u.(tui.AnalyzerModel)

		if am.IsConfirmingDelete() {
			t.Fatalf("expected delete confirmation modal NOT to open for protected item %s", child.Path)
		}
		if !strings.Contains(am.StatusMessage(), "Item protegido pela Blacklist Imutável") {
			t.Fatalf("expected blacklist warning message for %s, got: %s", child.Path, am.StatusMessage())
		}
	}

	// 3. Verify coordinator inviolability: if someone tries to pass protected paths directly to Coordinator,
	// the coordinator must skip all of them in Stage 1 and never delete them.
	rawTargets := make(map[string][]domain.Target)
	for _, p := range protectedPaths {
		rawTargets["filesystem"] = append(rawTargets["filesystem"], domain.Target{
			ID:   p.path,
			Path: p.path,
		})
	}
	res, err := coord.Delete(context.Background(), rawTargets, deleter.Options{Mode: "tui"})
	if err != nil {
		t.Fatalf("unexpected coordinator error: %v", err)
	}
	if len(res.CleanedPaths) != 0 {
		t.Fatalf("expected 0 cleaned paths for blacklisted targets, got %d: %v", len(res.CleanedPaths), res.CleanedPaths)
	}
	if len(mc.cleaned) != 0 {
		t.Fatalf("expected mockCleaner to clean 0 targets, got %d", len(mc.cleaned))
	}

	// 4. Verify that the unprotected item DOES open confirmation modal on 'd'
	var unprotIdx = -1
	for idx, child := range m.CurrentDir().Children {
		if !child.IsProtected {
			unprotIdx = idx
			break
		}
	}
	if unprotIdx < 0 {
		t.Fatal("expected to find unprotected child")
	}

	am := m
	for step := 0; step < unprotIdx; step++ {
		u, _ := am.Update(tea.KeyMsg{Type: tea.KeyDown})
		am = u.(tui.AnalyzerModel)
	}
	if am.Cursor() != unprotIdx {
		t.Fatalf("expected cursor on unprotected item (%d), got %d", unprotIdx, am.Cursor())
	}
	u, _ := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am = u.(tui.AnalyzerModel)
	if !am.IsConfirmingDelete() {
		t.Fatalf("expected delete confirmation modal to open for unprotected item")
	}
}

func TestAnalyzer_Lifecycle_Init_And_Updates(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "child_dir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "f.txt"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	bl := safety.NewBlacklist(nil)
	scanner := analyzer.NewScanner(bl, true, 0, 10)
	rootNode, err := scanner.ScanImmediate(tmpDir)
	if err != nil {
		t.Fatalf("ScanImmediate failed: %v", err)
	}

	m := tui.NewAnalyzerModel(rootNode, bl, nil, 100, 30)

	// Call Init()
	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("expected non-nil Init command")
	}

	// Test ScanUpdateMsg in Update
	updateMsg := analyzer.ScanUpdateMsg{
		Path:      subDir,
		SizeBytes: 120 * 1024 * 1024, // >= 100MB
		ItemCount: 15,
		Scanned:   true,
	}

	updated, followUpCmd := m.Update(updateMsg)
	am := updated.(tui.AnalyzerModel)

	// Verify child updated
	found := false
	for _, child := range am.CurrentDir().Children {
		if child.Path == subDir {
			found = true
			if child.SizeBytes != 120*1024*1024 {
				t.Errorf("expected size 120MB, got %d", child.SizeBytes)
			}
			if child.ItemCount != 15 {
				t.Errorf("expected item count 15, got %d", child.ItemCount)
			}
			if !child.Scanned {
				t.Errorf("expected Scanned=true")
			}
		}
	}
	if !found {
		t.Fatalf("child node %s not found in currentDir", subDir)
	}

	// Verify followUpCmd is returned for next scan update
	if followUpCmd == nil {
		t.Error("expected followUpCmd to listen for next scan update")
	}
}

func TestAnalyzer_DeleteNode_CoordinatorBranches(t *testing.T) {
	root := &analyzer.Node{
		Path:      "/delete-test",
		Name:      "delete-test",
		Type:      analyzer.ItemDir,
		SizeBytes: 1000,
		Children: []*analyzer.Node{
			{Path: "/delete-test/err_item", Name: "err_item", Type: analyzer.ItemFile, SizeBytes: 500},
			{Path: "/delete-test/ok_item", Name: "ok_item", Type: analyzer.ItemFile, SizeBytes: 500},
		},
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	mc := &mockCleaner{err: errors.New("simulated cleanup error")}
	cleaners := &mockCleanerProvider{cleaner: mc}
	bl := safety.NewBlacklist(nil)
	cfg := &config.Config{}
	coord := deleter.New(cleaners, cfg, bl, nil, nil, "", "")

	m := tui.NewAnalyzerModel(root, bl, coord, 100, 30)

	// 1. Delete with error from coordinator
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	am := updated.(tui.AnalyzerModel)

	updated, deleteCmd := am.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	am = updated.(tui.AnalyzerModel)
	if deleteCmd == nil {
		t.Fatal("expected deleteCmd")
	}

	msg := deleteCmd()
	updated, _ = am.Update(msg)
	am = updated.(tui.AnalyzerModel)

	// Since mock cleaner returned an error, statusMessage should reflect failure
	if !strings.Contains(am.StatusMessage(), "Erro ao mover para a lixeira") && !strings.Contains(am.StatusMessage(), "simulated cleanup error") {
		t.Errorf("expected error in status message, got: %s", am.StatusMessage())
	}
}

func TestAnalyzer_ScrollAndDimensions(t *testing.T) {
	var children []*analyzer.Node
	for i := 0; i < 25; i++ {
		children = append(children, &analyzer.Node{
			Path:      fmt.Sprintf("/scroll/node_%02d", i),
			Name:      fmt.Sprintf("node_%02d", i),
			Type:      analyzer.ItemFile,
			SizeBytes: int64(100 * (i + 1)),
		})
	}
	root := &analyzer.Node{
		Path:     "/scroll",
		Name:     "scroll",
		Type:     analyzer.ItemDir,
		Children: children,
	}
	for _, c := range root.Children {
		c.Parent = root
	}

	// Height 15 -> visibleRows = 15 - 7 = 8
	m := tui.NewAnalyzerModel(root, nil, nil, 80, 15)

	if m.Offset() != 0 {
		t.Errorf("initial offset = %d, want 0", m.Offset())
	}

	// Scroll down 12 times
	am := m
	for i := 0; i < 12; i++ {
		u, _ := am.Update(tea.KeyMsg{Type: tea.KeyDown})
		am = u.(tui.AnalyzerModel)
	}

	if am.Cursor() != 12 {
		t.Errorf("cursor after 12 downs = %d, want 12", am.Cursor())
	}
	if am.Offset() <= 0 {
		t.Errorf("expected offset > 0 after scrolling past visible rows, got %d", am.Offset())
	}

	// Scroll back up to 0
	for i := 0; i < 12; i++ {
		u, _ := am.Update(tea.KeyMsg{Type: tea.KeyUp})
		am = u.(tui.AnalyzerModel)
	}
	if am.Cursor() != 0 {
		t.Errorf("cursor after 12 ups = %d, want 0", am.Cursor())
	}
	if am.Offset() != 0 {
		t.Errorf("expected offset 0 when cursor at top, got %d", am.Offset())
	}

	// Window resize
	u, _ := am.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	am = u.(tui.AnalyzerModel)

	view := am.View()
	if !strings.Contains(view, "node_00") {
		t.Errorf("expected node_00 in view, got:\n%s", view)
	}
}

