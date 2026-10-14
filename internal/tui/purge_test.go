package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/domain"
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
