package tui

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/format"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/sahilm/fuzzy"
)

// RepoGroup represents a Git repository with detected build artifacts.
type RepoGroup struct {
	RepoDir       string
	RepoName      string
	Branch        string
	LastCommit    time.Time
	InactivityStr string
	Ecosystems    []string
	Targets       []domain.Target
	TotalBytes    int64
	Expanded      bool
	Selected      bool
}

// PurgeModel is the Bubble Tea model for the interactive purge interface.
type PurgeModel struct {
	repos          []*RepoGroup
	visibleIndices []int
	cursor         int
	offset         int
	width          int
	height         int
	filterText     string
	searchMode     bool
	confirming     bool
	deleting       bool
	coordinator    *deleter.Coordinator
	blacklist      *safety.Blacklist
	selectedCount  int
	selectedBytes  int64
	statusMessage  string
	quitting       bool
	done           bool
	freedBytes     int64
	freedCount     int
	progressCount  int
	progressTotal  int
	deletionResult *deleter.Result
	deletionErr    error
}

// NewPurgeModel creates and initializes a PurgeModel.
func NewPurgeModel(repos []*RepoGroup, coord *deleter.Coordinator, bl *safety.Blacklist) PurgeModel {
	visible := make([]int, len(repos))
	for i, rg := range repos {
		visible[i] = i
		if rg.TotalBytes == 0 {
			var total int64
			for _, t := range rg.Targets {
				total += t.SizeBytes
			}
			rg.TotalBytes = total
		}
		if rg.InactivityStr == "" && !rg.LastCommit.IsZero() {
			days := int(time.Since(rg.LastCommit).Hours() / 24)
			rg.InactivityStr = fmt.Sprintf("%dd atrás", days)
		}
	}

	m := PurgeModel{
		repos:          repos,
		visibleIndices: visible,
		cursor:         0,
		offset:         0,
		width:          80,
		height:         24,
		coordinator:    coord,
		blacklist:      bl,
	}
	m.updateTotals()
	return m
}

// Accessor methods for testing and inspection
func (m PurgeModel) Cursor() int {
	return m.cursor
}

func (m PurgeModel) Repos() []*RepoGroup {
	return m.repos
}

func (m PurgeModel) VisibleIndices() []int {
	return m.visibleIndices
}

func (m PurgeModel) SelectedCount() int {
	return m.selectedCount
}

func (m PurgeModel) SelectedBytes() int64 {
	return m.selectedBytes
}

func (m PurgeModel) IsSearchMode() bool {
	return m.searchMode
}

func (m PurgeModel) IsConfirming() bool {
	return m.confirming
}

func (m PurgeModel) IsDeleting() bool {
	return m.deleting
}

func (m PurgeModel) StatusMessage() string {
	return m.statusMessage
}

// Init initializes the Bubble Tea model.
func (m PurgeModel) Init() tea.Cmd {
	return nil
}

// Update handles incoming messages and events in the Elm loop.
func (m PurgeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustScroll()
		return m, nil

	case purgeProgressMsg:
		m.progressCount++
		m.freedBytes += msg.freedBytes
		return m, nil

	case purgeCompleteMsg:
		m.deleting = false
		m.done = true
		m.freedBytes = msg.freedBytes
		m.freedCount = msg.freedCount
		m.deletionResult = msg.result
		m.deletionErr = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}

	return m, nil
}

func (m PurgeModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		m.quitting = true
		return m, tea.Quit
	}

	if m.done {
		switch msg.String() {
		case "q", "enter", "esc":
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	}

	if m.deleting {
		return m, nil
	}

	if m.confirming {
		switch msg.String() {
		case "y", "Y":
			m.confirming = false
			m.deleting = true
			m.statusMessage = ""
			return m, m.startDeletionCmd()
		case "n", "N", "esc", "q":
			m.confirming = false
			m.statusMessage = ""
			return m, nil
		}
		return m, nil
	}

	if m.searchMode {
		switch msg.Type {
		case tea.KeyEsc, tea.KeyEnter:
			m.searchMode = false
			return m, nil
		case tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
				m.recomputeFilter()
			}
			return m, nil
		case tea.KeyRunes:
			m.filterText += string(msg.Runes)
			m.recomputeFilter()
			return m, nil
		}
		return m, nil
	}

	// Normal navigation mode
	switch msg.String() {
	case "q":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.visibleIndices)-1 {
			m.cursor++
			m.adjustScroll()
		}
		m.statusMessage = ""
		return m, nil

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.adjustScroll()
		}
		m.statusMessage = ""
		return m, nil

	case "enter", "tab":
		if len(m.visibleIndices) > 0 && m.cursor < len(m.visibleIndices) {
			rg := m.repos[m.visibleIndices[m.cursor]]
			rg.Expanded = !rg.Expanded
		}
		m.statusMessage = ""
		return m, nil

	case " ":
		if len(m.visibleIndices) > 0 && m.cursor < len(m.visibleIndices) {
			rg := m.repos[m.visibleIndices[m.cursor]]
			rg.Selected = !rg.Selected
			m.updateTotals()
		}
		m.statusMessage = ""
		return m, nil

	case "a":
		for _, idx := range m.visibleIndices {
			m.repos[idx].Selected = true
		}
		m.updateTotals()
		m.statusMessage = ""
		return m, nil

	case "n":
		for _, rg := range m.repos {
			rg.Selected = false
		}
		m.updateTotals()
		m.statusMessage = ""
		return m, nil

	case "i":
		for _, rg := range m.repos {
			rg.Selected = !rg.Selected
		}
		m.updateTotals()
		m.statusMessage = ""
		return m, nil

	case "/":
		m.searchMode = true
		m.statusMessage = ""
		return m, nil

	case "x", "d":
		if m.selectedCount == 0 {
			m.statusMessage = "Nenhum repositório selecionado para expurgo"
			return m, nil
		}
		m.confirming = true
		m.statusMessage = ""
		return m, nil
	}

	return m, nil
}

func (m *PurgeModel) updateTotals() {
	var count int
	var totalBytes int64
	for _, rg := range m.repos {
		if rg.Selected {
			count++
			totalBytes += rg.TotalBytes
		}
	}
	m.selectedCount = count
	m.selectedBytes = totalBytes
}

func (m *PurgeModel) adjustScroll() {
	visible := m.visibleHeight()
	if visible <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m PurgeModel) visibleHeight() int {
	h := m.height - 7
	if h < 1 {
		return 10
	}
	return h
}

// sahilm/fuzzy Source implementation for RepoGroup
type purgeRepoSource []*RepoGroup

func (r purgeRepoSource) String(i int) string {
	return fmt.Sprintf("%s %s %s %s", r[i].RepoName, r[i].Branch, strings.Join(r[i].Ecosystems, " "), r[i].RepoDir)
}

func (r purgeRepoSource) Len() int {
	return len(r)
}

func (m *PurgeModel) recomputeFilter() {
	if m.filterText == "" {
		m.visibleIndices = make([]int, len(m.repos))
		for i := range m.repos {
			m.visibleIndices[i] = i
		}
		if m.cursor >= len(m.visibleIndices) {
			m.cursor = max(0, len(m.visibleIndices)-1)
		}
		return
	}

	matches := fuzzy.FindFrom(m.filterText, purgeRepoSource(m.repos))
	m.visibleIndices = make([]int, len(matches))
	for i, match := range matches {
		m.visibleIndices[i] = match.Index
	}
	if m.cursor >= len(m.visibleIndices) {
		m.cursor = max(0, len(m.visibleIndices)-1)
	}
	m.adjustScroll()
}

// View renders the PurgeModel interface.
func (m PurgeModel) View() string {
	if m.quitting && !m.done {
		return ""
	}

	if m.done {
		return m.renderDoneView()
	}

	if m.deleting {
		return m.renderDeletingView()
	}

	var b strings.Builder

	// Header
	b.WriteString("\n")
	b.WriteString("  " + headerStyle.Render("DEVCLEAN PURGE INTERATIVO"))
	b.WriteString(helpStyle.Render(fmt.Sprintf(" (%d repositórios encontrados)\n\n", len(m.repos))))

	// Repository list with scrolling
	visibleCount := m.visibleHeight()
	end := m.offset + visibleCount
	if end > len(m.visibleIndices) {
		end = len(m.visibleIndices)
	}

	if len(m.visibleIndices) == 0 {
		b.WriteString(helpStyle.Render("    Nenhum repositório corresponde ao filtro atual.\n\n"))
	} else {
		for i := m.offset; i < end; i++ {
			repoIdx := m.visibleIndices[i]
			rg := m.repos[repoIdx]
			isFocused := i == m.cursor

			cursorPrefix := "  "
			if isFocused {
				cursorPrefix = "> "
			}

			check := "[ ]"
			if rg.Selected {
				check = "[x]"
			}

			expandIcon := "▶"
			if rg.Expanded {
				expandIcon = "▼"
			}

			branchStr := rg.Branch
			if branchStr == "" {
				branchStr = "detached"
			}

			inactivity := rg.InactivityStr
			if inactivity == "" && !rg.LastCommit.IsZero() {
				days := int(time.Since(rg.LastCommit).Hours() / 24)
				inactivity = fmt.Sprintf("%dd atrás", days)
			}

			var ecoBadges strings.Builder
			for _, eco := range rg.Ecosystems {
				ecoBadges.WriteString(fmt.Sprintf("[%s] ", eco))
			}

			repoNameFormatted := rg.RepoName
			if isFocused {
				repoNameFormatted = selectedStyle.Render(rg.RepoName)
			} else {
				repoNameFormatted = lipgloss.NewStyle().Bold(true).Render(rg.RepoName)
			}

			line := fmt.Sprintf("%s%s %s %s (%s) (%s) %s%s\n",
				cursorPrefix,
				check,
				expandIcon,
				repoNameFormatted,
				branchStr,
				inactivity,
				ecoBadges.String(),
				format.HumanSize(rg.TotalBytes),
			)
			b.WriteString(line)

			if rg.Expanded {
				for _, target := range rg.Targets {
					relPath, err := filepath.Rel(rg.RepoDir, target.Path)
					if err != nil || relPath == "" || relPath == "." {
						relPath = target.Path
					}
					targetLine := fmt.Sprintf("      └─ %s (%s)\n", relPath, format.HumanSize(target.SizeBytes))
					b.WriteString(helpStyle.Render(targetLine))
				}
			}
		}
	}

	b.WriteString("\n")

	// Status message if any
	if m.statusMessage != "" {
		b.WriteString("  " + statusAlertStyle.Render("⚠️  "+m.statusMessage) + "\n")
	}

	// Search bar if active
	if m.searchMode {
		b.WriteString("  " + searchPromptStyle.Render(fmt.Sprintf("/ %s█", m.filterText)) + "\n")
	}

	// Footer totals
	totalsStr := fmt.Sprintf("  %d repositórios selecionados | %s a liberar", m.selectedCount, format.HumanSize(m.selectedBytes))
	b.WriteString(headerStyle.Render(totalsStr) + "\n")

	// Shortcuts bar
	shortcuts := "  [Space] marcar  [a] todos  [n] nenhum  [i] inverter  [Enter/Tab] expandir  [/] buscar  [x] expurgar  [q] sair"
	b.WriteString(helpStyle.Render(shortcuts) + "\n")

	if m.confirming {
		return m.renderConfirmModal(b.String())
	}

	return b.String()
}

func (m PurgeModel) renderConfirmModal(underlying string) string {
	var targetCount int
	for _, rg := range m.repos {
		if rg.Selected {
			targetCount += len(rg.Targets)
		}
	}

	modalContent := fmt.Sprintf(
		"%s\n\nRepositórios: %d\nAlvos de build: %d\nEspaço a liberar: %s\n\n%s",
		modalTitleStyle.Render("CONFIRMAR EXPURGO DE BUILD ARTIFACTS"),
		m.selectedCount,
		targetCount,
		format.HumanSize(m.selectedBytes),
		"[y] Confirmar expurgo    [n/Esc] Cancelar",
	)

	box := modalBoxStyle.Render(modalContent)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return underlying + "\n" + box
}

func (m PurgeModel) renderDeletingView() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString("  " + headerStyle.Render("Expurgando artefatos de build...") + "\n\n")

	total := m.progressTotal
	if total == 0 {
		total = 1
	}
	current := m.progressCount
	if current > total {
		current = total
	}

	width := 30
	filled := (current * width) / total
	empty := width - filled
	if empty < 0 {
		empty = 0
	}

	bar := fmt.Sprintf("[%s%s] %d/%d (%s liberados)",
		strings.Repeat("█", filled),
		strings.Repeat(" ", empty),
		current,
		total,
		format.HumanSize(m.freedBytes),
	)

	b.WriteString("  " + bar + "\n\n")
	b.WriteString(helpStyle.Render("  Aguarde a conclusão da operação...") + "\n")
	return b.String()
}

func (m PurgeModel) renderDoneView() string {
	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString("  " + headerStyle.Render("✓ Expurgo concluído com sucesso!") + "\n\n")
	b.WriteString(fmt.Sprintf("  Espaço liberado: %s em %d alvos de build\n", format.HumanSize(m.freedBytes), m.freedCount))
	if m.deletionErr != nil {
		b.WriteString(statusAlertStyle.Render(fmt.Sprintf("  Erros encontrados: %v\n", m.deletionErr)))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  Pressione 'q' ou 'Enter' para sair.") + "\n")
	return b.String()
}

type purgeProgressMsg struct {
	freedBytes int64
}

type purgeCompleteMsg struct {
	freedBytes int64
	freedCount int
	result     *deleter.Result
	err        error
}

func (m PurgeModel) startDeletionCmd() tea.Cmd {
	coord := m.coordinator
	repos := m.repos

	var totalTargets int
	for _, rg := range repos {
		if rg.Selected {
			totalTargets += len(rg.Targets)
		}
	}

	return func() tea.Msg {
		if coord == nil {
			var freed int64
			var count int
			for _, rg := range repos {
				if rg.Selected {
					freed += rg.TotalBytes
					count += len(rg.Targets)
				}
			}
			return purgeCompleteMsg{freedBytes: freed, freedCount: count}
		}

		targetsByEco := make(map[string][]domain.Target)
		for _, rg := range repos {
			if !rg.Selected {
				continue
			}
			for _, t := range rg.Targets {
				eco := "generic"
				if len(rg.Ecosystems) > 0 {
					eco = rg.Ecosystems[0]
				}
				targetsByEco[eco] = append(targetsByEco[eco], t)
			}
		}

		opts := deleter.Options{
			Mode: "purge",
		}

		res, err := coord.Delete(context.Background(), targetsByEco, opts)
		if err != nil {
			return purgeCompleteMsg{freedBytes: 0, freedCount: 0, err: err}
		}

		return purgeCompleteMsg{
			freedBytes: res.TotalFreed,
			freedCount: res.TotalCount,
			result:     res,
		}
	}
}
