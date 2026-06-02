package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gabrielvp/devclean/internal/domain"
	"github.com/gabrielvp/devclean/internal/engine"
	"github.com/gabrielvp/devclean/internal/safety"
)

type viewState int

const (
	stateScanning   viewState = iota
	stateSelecting
	stateConfirming
	stateDeleting
	stateDone
)

type flatTarget struct {
	target    domain.Target
	ecosystem string
	groupIdx  int
}

type model struct {
	state       viewState
	results     []domain.ScanResult
	flatTargets []flatTarget
	selected    map[string]bool
	cursor      int
	width       int
	height      int
	spinner     spinner.Model
	engine      *engine.Engine
	blacklist   *safety.Blacklist

	// deletion progress
	deleteProgress int
	deleteTotal    int
	deletePath     string
	freedBytes     int64
	freedCount     int

	// empty result flag
	emptyResult bool

	// error
	err error
}

func New(eng *engine.Engine, bl *safety.Blacklist) model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	return model{
		state:     stateScanning,
		selected:  make(map[string]bool),
		spinner:   s,
		engine:    eng,
		blacklist: bl,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, scanCmd(m.engine))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q") {
			return m, tea.Quit
		}
		return m, nil

	case spinner.TickMsg:
		if m.state == stateScanning || m.state == stateDeleting {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case scanCompleteMsg:
		m.results = msg.results
		m.flatTargets = flattenResults(msg.results)
		if len(m.flatTargets) == 0 {
			m.state = stateDone
			m.emptyResult = true
			return m, tea.Quit
		}
		m.state = stateSelecting
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	switch m.state {
	case stateScanning:
		return fmt.Sprintf("\n  %s Scanning...\n\n", m.spinner.View())
	case stateSelecting:
		return "  Select targets to clean\n"
	case stateConfirming:
		return "  Confirm deletion\n"
	case stateDeleting:
		return fmt.Sprintf("\n  %s Deleting %d/%d... %s\n\n",
			m.spinner.View(), m.deleteProgress, m.deleteTotal, m.deletePath)
	case stateDone:
		if m.emptyResult {
			return "\n  ✅ All clean! No targets found.\n\n"
		}
		return fmt.Sprintf("\n  Done! Freed %s from %d targets.\n\n",
			humanSize(m.freedBytes), m.freedCount)
	}
	return ""
}

func scanCmd(eng *engine.Engine) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		ch := eng.Scan(ctx)
		results := engine.Collect(ctx, ch)
		return scanCompleteMsg{results: results}
	}
}

func flattenResults(results []domain.ScanResult) []flatTarget {
	var flat []flatTarget
	for groupIdx, r := range results {
		if r.Error != nil {
			continue
		}
		for _, t := range r.Targets {
			flat = append(flat, flatTarget{
				target:    t,
				ecosystem: r.Ecosystem,
				groupIdx:  groupIdx,
			})
		}
	}
	return flat
}

func humanSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB"}
	size := float64(bytes) / 1024
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", size, units[i])
}
