package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/engine"
	"github.com/gtkpad/devclean/internal/hooks"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/trash"
)

type viewState int

const (
	stateScanning viewState = iota
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
	offset      int
	width       int
	height      int
	spinner     spinner.Model
	engine      *engine.Engine
	blacklist   *safety.Blacklist
	cfg         *config.Config
	coordinator *deleter.Coordinator

	// filter/sort state
	filter         filterState
	visibleTargets []int

	// streaming scan state
	scanCh     <-chan domain.ScanResult
	scanCtx    context.Context
	scanCancel context.CancelFunc
	ecoStatus  map[string]string
	ecoNames   []string

	// deletion progress
	freedBytes     int64
	freedCount     int

	// empty result flag
	emptyResult bool

	// smart mode
	SmartMode      bool
	SmartThreshold float64
}

func New(eng *engine.Engine, bl *safety.Blacklist, cfg *config.Config) model {
	s := spinner.New()
	s.Spinner = spinner.Dot

	ctx, cancel := context.WithCancel(context.Background())
	var ecoNames []string
	if eng != nil {
		ecoNames = eng.Ecosystems()
	}
	ecoStatus := make(map[string]string, len(ecoNames))
	for _, name := range ecoNames {
		ecoStatus[name] = "scanning"
	}
	var ch <-chan domain.ScanResult
	if eng != nil {
		ch = eng.Scan(ctx)
	}

	resolver := platform.New()
	paths := resolver.Resolve()
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".local", "share", "devclean")

	var trashMgr *trash.Manager
	var hookRunner *hooks.Runner
	var configDir string
	if cfg != nil {
		trashMgr = trash.New(cfg.Safety)
		hookRunner = hooks.New(cfg.Hooks)
		configDir = paths.ConfigDir
	}
	coord := deleter.New(eng, cfg, bl, trashMgr, hookRunner, configDir, dataDir)

	return model{
		state:       stateScanning,
		selected:    make(map[string]bool),
		spinner:     s,
		engine:      eng,
		blacklist:   bl,
		cfg:         cfg,
		coordinator: coord,
		filter:      filterState{sort: sortSize},
		scanCh:      ch,
		scanCtx:     ctx,
		scanCancel:  cancel,
		ecoStatus:   ecoStatus,
		ecoNames:    ecoNames,
	}
}

func (m model) targetIcon(t domain.Target) string {
	if !m.SmartMode {
		return ""
	}
	if t.SmartScore >= m.SmartThreshold {
		return "⚡ "
	}
	return "🛡️ "
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.waitForScanResult())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case stateScanning:
			if msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q") {
				m.scanCancel()
				return m, tea.Quit
			}
		case stateSelecting:
			return m.updateSelecting(msg)
		case stateConfirming:
			return m.updateConfirming(msg)
		case stateDone:
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

	case ecosystemResultMsg:
		m.results = append(m.results, msg.result)
		if msg.result.Error != nil {
			m.ecoStatus[msg.result.Ecosystem] = "error"
		} else {
			m.ecoStatus[msg.result.Ecosystem] = "done"
		}
		return m, m.waitForScanResult()

	case scanDoneMsg:
		m.flatTargets = flattenResults(m.results)
		if len(m.flatTargets) == 0 {
			m.state = stateDone
			m.emptyResult = true
			return m, nil
		}
		m.state = stateSelecting
		m.recompute()
		return m, nil

	case scanCompleteMsg:
		m.results = msg.results
		m.flatTargets = flattenResults(msg.results)
		if len(m.flatTargets) == 0 {
			m.state = stateDone
			m.emptyResult = true
			return m, nil
		}
		m.state = stateSelecting
		m.recompute()
		return m, nil

	case deleteCompleteMsg:
		m.freedBytes = msg.freed
		m.freedCount = msg.count
		m.state = stateDone
		return m, nil
	}

	return m, nil
}

func (m model) updateConfirming(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		return m, tea.Quit
	case msg.Type == tea.KeyRunes && string(msg.Runes) == "y":
		m.state = stateDeleting
		return m, tea.Batch(m.spinner.Tick, deleteCmd(m))
	case msg.Type == tea.KeyRunes && (string(msg.Runes) == "n" || string(msg.Runes) == "q"):
		m.state = stateSelecting
	}
	return m, nil
}

func (m model) updateSelecting(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Filter input mode intercepts all keys
	if m.filter.fuzzyActive {
		return m.updateFilterInput(msg)
	}

	switch {
	case msg.Type == tea.KeyCtrlC || (msg.Type == tea.KeyRunes && string(msg.Runes) == "q"):
		return m, tea.Quit

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "/":
		m.filter.fuzzyActive = true
		return m, nil

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "s":
		m.filter.sort = (m.filter.sort + 1) % 4
		m.recompute()

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "r":
		m.filter.risk = (m.filter.risk + 1) % 4
		m.recompute()

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "m":
		threshold := m.cfg.TUI.MassSelectThresholdMB * 1024 * 1024
		for _, idx := range m.visibleTargets {
			if m.flatTargets[idx].target.SizeBytes >= threshold {
				m.selected[m.flatTargets[idx].target.ID] = true
			}
		}

	case msg.Type == tea.KeyEsc:
		m.filter = filterState{sort: sortSize}
		m.recompute()

	case msg.Type == tea.KeyUp || (msg.Type == tea.KeyRunes && string(msg.Runes) == "k"):
		if m.cursor > 0 {
			m.cursor--
			m.adjustOffset()
		}

	case msg.Type == tea.KeyDown || (msg.Type == tea.KeyRunes && string(msg.Runes) == "j"):
		if m.cursor < len(m.visibleTargets)-1 {
			m.cursor++
			m.adjustOffset()
		}

	case msg.Type == tea.KeyTab:
		m.cursor = m.nextGroupStart()
		m.adjustOffset()

	case msg.Type == tea.KeySpace:
		if m.cursor < len(m.visibleTargets) {
			id := m.flatTargets[m.visibleTargets[m.cursor]].target.ID
			m.selected[id] = !m.selected[id]
			if !m.selected[id] {
				delete(m.selected, id)
			}
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "a":
		for _, idx := range m.visibleTargets {
			m.selected[m.flatTargets[idx].target.ID] = true
		}

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "n":
		m.selected = make(map[string]bool)

	case msg.Type == tea.KeyRunes && string(msg.Runes) == "l":
		m.selected = make(map[string]bool)
		for _, idx := range m.visibleTargets {
			if m.flatTargets[idx].target.Risk == domain.RiskLow {
				m.selected[m.flatTargets[idx].target.ID] = true
			}
		}

	case msg.Type == tea.KeyEnter:
		if len(m.selected) > 0 {
			m.state = stateConfirming
		}
	}

	return m, nil
}

func (m model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.filter.fuzzyActive = false
		return m, nil
	case tea.KeyEnter:
		m.filter.fuzzyActive = false
		return m, nil
	case tea.KeyBackspace:
		if len(m.filter.fuzzyText) > 0 {
			m.filter.fuzzyText = m.filter.fuzzyText[:len(m.filter.fuzzyText)-1]
			m.recompute()
		}
	case tea.KeyRunes:
		m.filter.fuzzyText += string(msg.Runes)
		m.recompute()
	}
	return m, nil
}

func (m *model) recompute() {
	m.visibleTargets = recomputeVisible(m.flatTargets, m.filter, m.cfg.TUI.MassSelectThresholdMB*1024*1024)
	if m.cursor >= len(m.visibleTargets) {
		m.cursor = max(0, len(m.visibleTargets)-1)
	}
	m.adjustOffset()
}

func (m model) selectedTotal() (int64, int) {
	var total int64
	var count int
	for _, ft := range m.flatTargets {
		if m.selected[ft.target.ID] {
			total += ft.target.SizeBytes
			count++
		}
	}
	return total, count
}

func (m model) nextGroupStart() int {
	if len(m.visibleTargets) == 0 {
		return 0
	}
	currentGroup := m.flatTargets[m.visibleTargets[m.cursor]].groupIdx
	for i := m.cursor + 1; i < len(m.visibleTargets); i++ {
		if m.flatTargets[m.visibleTargets[i]].groupIdx != currentGroup {
			return i
		}
	}
	return 0
}

func (m *model) adjustOffset() {
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

func (m model) visibleHeight() int {
	// dashboard ~4 lines, footer ~2 lines, padding ~2
	const overhead = 8
	h := m.height - overhead
	if h < 1 {
		h = 20
	}
	return h
}

func (m model) View() string {
	switch m.state {
	case stateScanning:
		return viewScanning(m)
	case stateSelecting:
		return viewSelecting(m)
	case stateConfirming:
		return viewConfirming(m)
	case stateDeleting:
		return viewDeleting(m)
	case stateDone:
		return viewDone(m)
	}
	return ""
}

func (m model) waitForScanResult() tea.Cmd {
	ch := m.scanCh
	return func() tea.Msg {
		r, ok := <-ch
		if !ok {
			return scanDoneMsg{}
		}
		return ecosystemResultMsg{result: r}
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

func deleteCmd(m model) tea.Cmd {
	coordinator := m.coordinator
	flatTargets := m.flatTargets
	selected := m.selected

	return func() tea.Msg {
		if coordinator == nil {
			return deleteCompleteMsg{freed: 0, count: 0}
		}

		byEcosystem := make(map[string][]domain.Target)
		for _, ft := range flatTargets {
			if selected[ft.target.ID] {
				byEcosystem[ft.ecosystem] = append(byEcosystem[ft.ecosystem], ft.target)
			}
		}

		opts := deleter.Options{
			Mode: "tui",
		}

		res, err := coordinator.Delete(context.Background(), byEcosystem, opts)
		if err != nil {
			return deleteCompleteMsg{freed: 0, count: 0}
		}
		return deleteCompleteMsg{freed: res.TotalFreed, count: res.TotalCount}
	}
}
