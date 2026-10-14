package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/safety"
)

func TestPurge_RepoGrouping(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:       "/home/user/projects/web-app",
			RepoName:      "web-app",
			Branch:        "main",
			LastCommit:    time.Now().Add(-45 * 24 * time.Hour),
			InactivityStr: "45d atrás",
			Ecosystems:    []string{"node"},
			Targets: []domain.Target{
				{
					ID:        "web-app-nm",
					Path:      "/home/user/projects/web-app/node_modules",
					SizeBytes: 500 * 1024 * 1024,
				},
			},
			TotalBytes: 500 * 1024 * 1024,
			Expanded:   false,
			Selected:   false,
		},
		{
			RepoDir:       "/home/user/projects/backend-api",
			RepoName:      "backend-api",
			Branch:        "feat/v2",
			LastCommit:    time.Now().Add(-90 * 24 * time.Hour),
			InactivityStr: "90d atrás",
			Ecosystems:    []string{"go", "docker"},
			Targets: []domain.Target{
				{
					ID:        "backend-bin",
					Path:      "/home/user/projects/backend-api/bin",
					SizeBytes: 50 * 1024 * 1024,
				},
			},
			TotalBytes: 50 * 1024 * 1024,
			Expanded:   false,
			Selected:   false,
		},
		{
			RepoDir:       "/home/user/projects/detached-tool",
			RepoName:      "detached-tool",
			Branch:        "", // empty branch -> detached/unknown
			LastCommit:    time.Now().Add(-120 * 24 * time.Hour),
			InactivityStr: "120d atrás",
			Ecosystems:    []string{"rust"},
			Targets: []domain.Target{
				{
					ID:        "rust-target",
					Path:      "/home/user/projects/detached-tool/target",
					SizeBytes: 1024 * 1024 * 1024,
				},
			},
			TotalBytes: 1024 * 1024 * 1024,
			Expanded:   false,
			Selected:   false,
		},
	}

	model := NewPurgeModel(repos, nil, nil)
	view := model.View()

	// Validate title & count
	if !strings.Contains(view, "DEVCLEAN PURGE INTERATIVO") {
		t.Errorf("expected header to contain 'DEVCLEAN PURGE INTERATIVO', got:\n%s", view)
	}
	if !strings.Contains(view, "3") {
		t.Errorf("expected view to mention 3 repositories, got:\n%s", view)
	}

	// Validate repository names
	for _, repo := range repos {
		if !strings.Contains(view, repo.RepoName) {
			t.Errorf("expected view to contain repo name %q", repo.RepoName)
		}
	}

	// Validate branch names and detached fallback
	if !strings.Contains(view, "(main)") {
		t.Errorf("expected view to contain branch (main)")
	}
	if !strings.Contains(view, "(feat/v2)") {
		t.Errorf("expected view to contain branch (feat/v2)")
	}
	if !strings.Contains(view, "(detached)") && !strings.Contains(view, "(unknown)") {
		t.Errorf("expected view to contain (detached) or (unknown) for empty branch")
	}

	// Validate relative inactivity strings
	if !strings.Contains(view, "45d atrás") {
		t.Errorf("expected view to contain '45d atrás'")
	}
	if !strings.Contains(view, "90d atrás") {
		t.Errorf("expected view to contain '90d atrás'")
	}

	// Validate ecosystem badges
	if !strings.Contains(view, "[node]") {
		t.Errorf("expected view to contain ecosystem badge [node]")
	}
	if !strings.Contains(view, "[go]") || !strings.Contains(view, "[docker]") {
		t.Errorf("expected view to contain ecosystem badges [go] and [docker]")
	}
	if !strings.Contains(view, "[rust]") {
		t.Errorf("expected view to contain ecosystem badge [rust]")
	}
}

func TestPurge_ExpandCollapse(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:       "/home/user/projects/web-app",
			RepoName:      "web-app",
			Branch:        "main",
			InactivityStr: "45d atrás",
			Ecosystems:    []string{"node"},
			Targets: []domain.Target{
				{
					ID:        "web-app-nm",
					Path:      "/home/user/projects/web-app/node_modules",
					SizeBytes: 500 * 1024 * 1024,
				},
				{
					ID:        "web-app-dist",
					Path:      "/home/user/projects/web-app/dist",
					SizeBytes: 20 * 1024 * 1024,
				},
			},
			TotalBytes: 520 * 1024 * 1024,
			Expanded:   false,
			Selected:   false,
		},
		{
			RepoDir:       "/home/user/projects/backend-api",
			RepoName:      "backend-api",
			Branch:        "main",
			InactivityStr: "10d atrás",
			Ecosystems:    []string{"go"},
			Targets: []domain.Target{
				{
					ID:        "backend-bin",
					Path:      "/home/user/projects/backend-api/bin",
					SizeBytes: 10 * 1024 * 1024,
				},
			},
			TotalBytes: 10 * 1024 * 1024,
			Expanded:   false,
			Selected:   false,
		},
	}

	m := NewPurgeModel(repos, nil, nil)

	// Initially collapsed: targets should not be visible
	initialView := m.View()
	if strings.Contains(initialView, "dist") {
		t.Errorf("expected 'dist' to be hidden when collapsed, got:\n%s", initialView)
	}

	// Press Enter to expand first repo
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm := updated.(PurgeModel)

	if !pm.repos[0].Expanded {
		t.Errorf("expected first repo Expanded to be true after Enter")
	}

	expandedView := pm.View()
	if !strings.Contains(expandedView, "node_modules") {
		t.Errorf("expected expanded view to contain child artifact 'node_modules'")
	}
	if !strings.Contains(expandedView, "dist") {
		t.Errorf("expected expanded view to contain child artifact 'dist'")
	}
	if !strings.Contains(expandedView, "▼") {
		t.Errorf("expected expand indicator '▼' when expanded")
	}

	// Press Tab to collapse it back
	updated2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyTab})
	pm2 := updated2.(PurgeModel)

	if pm2.repos[0].Expanded {
		t.Errorf("expected first repo Expanded to be false after Tab")
	}

	collapsedView := pm2.View()
	if strings.Contains(collapsedView, "dist") {
		t.Errorf("expected 'dist' to be hidden after collapse, got:\n%s", collapsedView)
	}
	if !strings.Contains(collapsedView, "▶") {
		t.Errorf("expected collapse indicator '▶' when collapsed")
	}

	// Test navigation j/k
	// Move down with 'j'
	updated3, _ := pm2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	pm3 := updated3.(PurgeModel)
	if pm3.Cursor() != 1 {
		t.Errorf("expected cursor to be 1 after 'j', got %d", pm3.Cursor())
	}

	// Move up with 'k'
	updated4, _ := pm3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	pm4 := updated4.(PurgeModel)
	if pm4.Cursor() != 0 {
		t.Errorf("expected cursor to be 0 after 'k', got %d", pm4.Cursor())
	}
}
func TestPurge_BatchSelection(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:    "/home/user/repo-a",
			RepoName:   "repo-a",
			TotalBytes: 100 * 1024 * 1024,
			Targets: []domain.Target{
				{ID: "t-a", SizeBytes: 100 * 1024 * 1024},
			},
		},
		{
			RepoDir:    "/home/user/repo-b",
			RepoName:   "repo-b",
			TotalBytes: 200 * 1024 * 1024,
			Targets: []domain.Target{
				{ID: "t-b", SizeBytes: 200 * 1024 * 1024},
			},
		},
		{
			RepoDir:    "/home/user/repo-c",
			RepoName:   "repo-c",
			TotalBytes: 300 * 1024 * 1024,
			Targets: []domain.Target{
				{ID: "t-c", SizeBytes: 300 * 1024 * 1024},
			},
		},
	}

	m := NewPurgeModel(repos, nil, nil)

	if m.SelectedCount() != 0 || m.SelectedBytes() != 0 {
		t.Fatalf("expected initial 0 selections, got count=%d bytes=%d", m.SelectedCount(), m.SelectedBytes())
	}

	// 1. Space: toggle cursor repo (repo-a)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	pm := updated.(PurgeModel)
	if pm.SelectedCount() != 1 || pm.SelectedBytes() != 100*1024*1024 {
		t.Errorf("expected 1 repo selected (100MB), got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// Space again: deselect repo-a
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeySpace})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 0 || pm.SelectedBytes() != 0 {
		t.Errorf("expected 0 repos selected after second Space, got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// 2. 'a': select all
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 3 || pm.SelectedBytes() != 600*1024*1024 {
		t.Errorf("expected 3 repos selected (600MB) with 'a', got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// 3. 'n': deselect all
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 0 || pm.SelectedBytes() != 0 {
		t.Errorf("expected 0 repos selected with 'n', got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// 4. 'i': invert selection (all were false, now all true)
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 3 || pm.SelectedBytes() != 600*1024*1024 {
		t.Errorf("expected 3 repos selected with 'i', got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// Deselect repo-a with Space
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeySpace})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 2 || pm.SelectedBytes() != 500*1024*1024 {
		t.Errorf("expected 2 repos selected (500MB), got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}

	// Invert again: repo-a becomes selected, repo-b and repo-c become unselected
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")})
	pm = updated.(PurgeModel)
	if pm.SelectedCount() != 1 || pm.SelectedBytes() != 100*1024*1024 {
		t.Errorf("expected 1 repo selected (100MB) after 'i', got count=%d bytes=%d", pm.SelectedCount(), pm.SelectedBytes())
	}
}

func TestPurge_FuzzySearch(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:    "/home/user/web-app",
			RepoName:   "web-app",
			Branch:     "main",
			Ecosystems: []string{"node"},
			TotalBytes: 50 * 1024 * 1024,
			Targets:    []domain.Target{{ID: "t1", SizeBytes: 50 * 1024 * 1024}},
		},
		{
			RepoDir:    "/home/user/backend-api",
			RepoName:   "backend-api",
			Branch:     "staging",
			Ecosystems: []string{"go"},
			TotalBytes: 70 * 1024 * 1024,
			Targets:    []domain.Target{{ID: "t2", SizeBytes: 70 * 1024 * 1024}},
		},
		{
			RepoDir:    "/home/user/rust-tools",
			RepoName:   "rust-tools",
			Branch:     "dev",
			Ecosystems: []string{"rust"},
			TotalBytes: 90 * 1024 * 1024,
			Targets:    []domain.Target{{ID: "t3", SizeBytes: 90 * 1024 * 1024}},
		},
	}

	m := NewPurgeModel(repos, nil, nil)

	// Press '/' to enter search mode
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	pm := updated.(PurgeModel)
	if !pm.IsSearchMode() {
		t.Fatalf("expected IsSearchMode to be true after '/'")
	}

	// Type "web"
	for _, ch := range "web" {
		updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		pm = updated.(PurgeModel)
	}

	if len(pm.VisibleIndices()) != 1 {
		t.Fatalf("expected 1 match for 'web', got %d", len(pm.VisibleIndices()))
	}
	matchedRepo := pm.repos[pm.VisibleIndices()[0]]
	if matchedRepo.RepoName != "web-app" {
		t.Errorf("expected matched repo to be 'web-app', got %s", matchedRepo.RepoName)
	}

	view := pm.View()
	if !strings.Contains(view, "web-app") || strings.Contains(view, "backend-api") {
		t.Errorf("expected view to show only web-app, got:\n%s", view)
	}

	// Backspace 3 times
	for i := 0; i < 3; i++ {
		updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyBackspace})
		pm = updated.(PurgeModel)
	}
	if len(pm.VisibleIndices()) != 3 {
		t.Errorf("expected all 3 repos visible after backspaces, got %d", len(pm.VisibleIndices()))
	}

	// Search for ecosystem "rust"
	for _, ch := range "rust" {
		updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		pm = updated.(PurgeModel)
	}
	if len(pm.VisibleIndices()) != 1 {
		t.Fatalf("expected 1 match for 'rust', got %d", len(pm.VisibleIndices()))
	}
	if pm.repos[pm.VisibleIndices()[0]].RepoName != "rust-tools" {
		t.Errorf("expected matched repo 'rust-tools', got %s", pm.repos[pm.VisibleIndices()[0]].RepoName)
	}

	// Press Esc to exit search mode
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm = updated.(PurgeModel)
	if pm.IsSearchMode() {
		t.Errorf("expected IsSearchMode to be false after Esc")
	}
}

func TestPurge_ConfirmModal(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:    "/home/user/web-app",
			RepoName:   "web-app",
			TotalBytes: 50 * 1024 * 1024,
			Targets: []domain.Target{
				{ID: "t1", Path: "/home/user/web-app/node_modules", SizeBytes: 50 * 1024 * 1024},
			},
		},
	}

	m := NewPurgeModel(repos, nil, nil)

	// 1. Press 'x' with 0 selected: should show warning and not enter confirming
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	pm := updated.(PurgeModel)
	if pm.IsConfirming() {
		t.Errorf("expected confirming to be false when 0 repos selected")
	}
	if !strings.Contains(pm.StatusMessage(), "Nenhum repositório selecionado") {
		t.Errorf("expected warning status message, got: %s", pm.StatusMessage())
	}

	// 2. Select repo with Space
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeySpace})
	pm = updated.(PurgeModel)

	// 3. Press 'x' (or 'd') with selection: opens confirming modal
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	pm = updated.(PurgeModel)
	if !pm.IsConfirming() {
		t.Fatalf("expected confirming to be true after 'x' with selection")
	}
	modalView := pm.View()
	if !strings.Contains(modalView, "CONFIRMAR EXPURGO") {
		t.Errorf("expected view to contain 'CONFIRMAR EXPURGO', got:\n%s", modalView)
	}

	// 4. Press 'n': cancels confirmation
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	pm = updated.(PurgeModel)
	if pm.IsConfirming() {
		t.Errorf("expected confirming to be false after 'n'")
	}

	// 5. Press 'd': opens confirmation modal again
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	pm = updated.(PurgeModel)
	if !pm.IsConfirming() {
		t.Fatalf("expected confirming to be true after 'd'")
	}

	// 6. Press 'y': confirms and starts deletion
	updated, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	pm = updated.(PurgeModel)
	if pm.IsConfirming() {
		t.Errorf("expected confirming to be false after 'y'")
	}
	if !pm.IsDeleting() {
		t.Errorf("expected deleting to be true after 'y'")
	}
	if cmd == nil {
		t.Fatalf("expected non-nil deletion command returned after 'y'")
	}

	// Execute deletion command
	msg := cmd()
	if _, ok := msg.(purgeCompleteMsg); !ok {
		t.Fatalf("expected purgeCompleteMsg from cmd, got %T", msg)
	}

	// Feed completion message to Update
	updated, _ = pm.Update(msg)
	pm = updated.(PurgeModel)
	if pm.IsDeleting() {
		t.Errorf("expected deleting to be false after purgeCompleteMsg")
	}

	doneView := pm.View()
	if !strings.Contains(doneView, "Expurgo concluído com sucesso") {
		t.Errorf("expected done view to contain success message, got:\n%s", doneView)
	}

	// Press 'q' on done screen: quits
	_, quitCmd := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if quitCmd == nil {
		t.Errorf("expected quit command on 'q' from done view")
	}
}
func TestPurge_AdditionalCoverage(t *testing.T) {
	repos := []*RepoGroup{
		{
			RepoDir:    "/home/user/r1",
			RepoName:   "r1",
			TotalBytes: 10 * 1024 * 1024,
			Targets:    []domain.Target{{ID: "t1", SizeBytes: 10 * 1024 * 1024}},
		},
		{
			RepoDir:    "/home/user/r2",
			RepoName:   "r2",
			TotalBytes: 20 * 1024 * 1024,
			Targets:    []domain.Target{{ID: "t2", SizeBytes: 20 * 1024 * 1024}},
		},
	}

	m := NewPurgeModel(repos, nil, nil)

	// Init
	if cmd := m.Init(); cmd != nil {
		t.Errorf("expected nil Init cmd, got %v", cmd)
	}

	// Repos getter
	if len(m.Repos()) != 2 {
		t.Errorf("expected Repos() to return 2 items, got %d", len(m.Repos()))
	}

	// WindowSizeMsg
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pm := updated.(PurgeModel)
	if pm.width != 120 || pm.height != 40 {
		t.Errorf("expected width 120 and height 40, got %dx%d", pm.width, pm.height)
	}

	// Small height edge case
	smallUpdate, _ := m.Update(tea.WindowSizeMsg{Width: 40, Height: 5})
	pmSmall := smallUpdate.(PurgeModel)
	if pmSmall.visibleHeight() < 1 {
		t.Errorf("expected visibleHeight >= 1 even for small window")
	}

	// Ctrl+C quit
	_, quitCmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if quitCmd == nil {
		t.Errorf("expected quit cmd on Ctrl+C")
	}

	// Deleting view and progress
	mDeleting := m
	mDeleting.deleting = true
	mDeleting.progressTotal = 10
	mDeleting.progressCount = 5
	mDeleting.freedBytes = 50 * 1024 * 1024
	deletingView := mDeleting.View()
	if !strings.Contains(deletingView, "Expurgando") || !strings.Contains(deletingView, "5/10") {
		t.Errorf("expected progress bar in deleting view, got:\n%s", deletingView)
	}

	// PurgeProgressMsg
	updated, _ = mDeleting.Update(purgeProgressMsg{freedBytes: 10 * 1024 * 1024})
	pmProgress := updated.(PurgeModel)
	if pmProgress.progressCount != 6 || pmProgress.freedBytes != 60*1024*1024 {
		t.Errorf("expected progress count 6 and 60MB freed, got %d and %d", pmProgress.progressCount, pmProgress.freedBytes)
	}

	// Quitting returns empty string
	mQuitting := m
	mQuitting.quitting = true
	if v := mQuitting.View(); v != "" {
		t.Errorf("expected empty string when quitting, got: %s", v)
	}

	// Scrolling bounds with many items
	var manyRepos []*RepoGroup
	for i := 0; i < 20; i++ {
		manyRepos = append(manyRepos, &RepoGroup{
			RepoDir:    fmt.Sprintf("/repo/%d", i),
			RepoName:   fmt.Sprintf("repo-%02d", i),
			TotalBytes: 1024,
			Targets:    []domain.Target{{ID: fmt.Sprintf("t%d", i), SizeBytes: 1024}},
		})
	}
	mScroll := NewPurgeModel(manyRepos, nil, nil)
	mScroll.height = 12 // visibleHeight = 5
	for i := 0; i < 15; i++ {
		u, _ := mScroll.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
		mScroll = u.(PurgeModel)
	}
	if mScroll.Cursor() != 15 {
		t.Errorf("expected cursor at 15, got %d", mScroll.Cursor())
	}
	if mScroll.offset <= 0 {
		t.Errorf("expected offset to scroll down, got %d", mScroll.offset)
	}
}

func TestPurge_CoordinatorIntegration(t *testing.T) {
	bl := safety.NewBlacklist([]string{"*.protected"})
	fc := &fakeCleaner{}
	cleaners := &fakeCleanerProvider{cleaner: fc}
	cfg := &config.Config{}
	coord := deleter.New(cleaners, cfg, bl, nil, nil, "", "")

	repos := []*RepoGroup{
		{
			RepoDir:    "/home/user/my-app",
			RepoName:   "my-app",
			Ecosystems: []string{"go"},
			Targets: []domain.Target{
				{ID: "t-safe", Path: "/home/user/my-app/bin", SizeBytes: 2048},
				{ID: "t-blocked", Path: "/home/user/my-app/.git", SizeBytes: 1024},
				{ID: "t-custom", Path: "/home/user/my-app/secrets.protected", SizeBytes: 512},
			},
			LastCommit: time.Now().Add(-30 * 24 * time.Hour),
		},
	}

	// NewPurgeModel calculates TotalBytes and InactivityStr automatically if empty
	m := NewPurgeModel(repos, coord, bl)
	if m.repos[0].TotalBytes != 3584 {
		t.Errorf("expected auto-calculated TotalBytes 3584, got %d", m.repos[0].TotalBytes)
	}
	if !strings.Contains(m.repos[0].InactivityStr, "30d atrás") {
		t.Errorf("expected auto-calculated InactivityStr, got %q", m.repos[0].InactivityStr)
	}

	// Select the repo
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	pm := updated.(PurgeModel)

	// Press 'x' to open confirmation modal
	updated, _ = pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	pm = updated.(PurgeModel)
	if !pm.IsConfirming() {
		t.Fatal("expected IsConfirming true")
	}

	// Press 'y' to start deletion with coordinator
	updated, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	pm = updated.(PurgeModel)
	if cmd == nil {
		t.Fatal("expected non-nil deletion command")
	}

	msg := cmd()
	compMsg, ok := msg.(purgeCompleteMsg)
	if !ok {
		t.Fatalf("expected purgeCompleteMsg, got %T", msg)
	}
	if compMsg.err != nil {
		t.Fatalf("unexpected deletion error: %v", compMsg.err)
	}

	// Verify coordinator safety:
	// The fake cleaner must only have cleaned safe target (t-safe).
	// .git and secrets.protected MUST have been filtered by blacklist in Stage 1!
	if len(fc.cleaned) != 1 {
		t.Fatalf("expected exactly 1 cleaned target, got %d", len(fc.cleaned))
	}
	if fc.cleaned[0].Path != "/home/user/my-app/bin" {
		t.Fatalf("expected cleaned path /home/user/my-app/bin, got %s", fc.cleaned[0].Path)
	}

	// Feed completion to model
	updated, _ = pm.Update(compMsg)
	pm = updated.(PurgeModel)
	view := pm.View()
	if !strings.Contains(view, "Expurgo concluído com sucesso") {
		t.Errorf("expected success in view, got:\n%s", view)
	}

	// Test error case branch in Update
	errMsg := purgeCompleteMsg{err: errors.New("simulated purge error")}
	updated, _ = pm.Update(errMsg)
	pmErr := updated.(PurgeModel)
	errView := pmErr.renderDoneView()
	if !strings.Contains(errView, "Erros encontrados") || !strings.Contains(errView, "simulated purge error") {
		t.Errorf("expected error in renderDoneView, got:\n%s", errView)
	}
}

func TestPurge_TrashOptionsPassed(t *testing.T) {
	trashOpt := false
	m := NewPurgeModel(nil, nil, nil, &trashOpt)
	if m.useTrashOpt == nil || *m.useTrashOpt != false {
		t.Fatalf("expected useTrashOpt false, got %v", m.useTrashOpt)
	}

	trashOptTrue := true
	m2 := NewPurgeModel(nil, nil, nil, &trashOptTrue)
	if m2.useTrashOpt == nil || *m2.useTrashOpt != true {
		t.Fatalf("expected useTrashOpt true, got %v", m2.useTrashOpt)
	}
}
