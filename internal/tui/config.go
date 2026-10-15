package tui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gtkpad/devclean/internal/config"
	"github.com/gtkpad/devclean/internal/format"
	"github.com/gtkpad/devclean/internal/safety"
	"github.com/gtkpad/devclean/internal/scheduler"
	"gopkg.in/yaml.v3"
)

// ConfigTab represents the active horizontal tab.
type ConfigTab int

const (
	TabPaths ConfigTab = iota
	TabEcosystems
	TabSafety
	TabWatch
	TabScheduler
)

// ConfigState defines the state of the configuration TUI.
type ConfigState int

const (
	ConfigStateNormal ConfigState = iota
	ConfigStateEditingField
	ConfigStateAddingPath
	ConfigStateConfirmExit
	ConfigStateHelp
	ConfigStateOSError
)

// EcosystemMeta stores metadata for display and identification.
type EcosystemMeta struct {
	Key         string
	Name        string
	Description string
}

var ecosystemMetas = []EcosystemMeta{
	{Key: "docker", Name: "Docker", Description: "Containers, imagens, cache de build e volumes"},
	{Key: "node", Name: "Node.js", Description: "node_modules e caches de gerenciadores de pacotes"},
	{Key: "go", Name: "Go", Description: "Build cache e test cache"},
	{Key: "dotnet", Name: ".NET", Description: "bin, obj e pacotes NuGet"},
	{Key: "python", Name: "Python", Description: "Ambientes virtuais, __pycache__ e cache uv"},
	{Key: "rust", Name: "Rust", Description: "Diretórios target e cache do Cargo"},
	{Key: "java", Name: "Java", Description: "Maven target e cache do Gradle"},
	{Key: "nextjs", Name: "Next.js", Description: "Cache .next e artefatos de build"},
	{Key: "bun", Name: "Bun", Description: "Runtime cache e dependências instaladas"},
	{Key: "xcode", Name: "Xcode", Description: "DerivedData e caches de simuladores iOS"},
	{Key: "ai", Name: "AI", Description: "Caches de modelos locais (Claude Code, Ollama, HuggingFace)"},
	{Key: "pnpm", Name: "pnpm", Description: "Store global e store virtual local"},
	{Key: "homebrew", Name: "Homebrew", Description: "Cache de downloads e bottles"},
	{Key: "cpp", Name: "C/C++", Description: "Artefatos de build (CMake, vcpkg, Conan)"},
	{Key: "flutter", Name: "Flutter", Description: "Artefatos de build e cache do Pub"},
	{Key: "terraform", Name: "Terraform", Description: "Cache de plugins e diretórios .terraform"},
	{Key: "cocoapods", Name: "CocoaPods", Description: "Cache e diretórios Pods"},
	{Key: "elixir", Name: "Elixir", Description: "Diretórios _build e deps do Mix"},
	{Key: "shell", Name: "Shell & CLI", Description: "Caches do mise, GitHub CLI, pre-commit, Oh My Zsh e zcompdump"},
}

// ConfigModel is the Bubble Tea model for interactive devclean config.
type ConfigModel struct {
	cfg         *config.Config
	astRoot     *yaml.Node
	configFile  string
	sched       scheduler.Scheduler
	blacklist   *safety.Blacklist
	activeTab   ConfigTab
	state       ConfigState
	cursor      int
	offset      int
	width       int
	height      int
	textInput   textinput.Model
	isDirty     bool
	statusMsg   string
	osErrorMsg  string
	schedStatus *scheduler.Status

	// Sched & Watch states
	schedActive        bool
	initialSchedActive bool
	schedFreq          string // "daily", "weekly", "hourly", "custom"
	schedAtTime        string
	schedPreset        string // "none", "safe", "standard", "aggressive"
	schedCron          string
	watchActive        bool
	initialWatchActive bool
	watchInterval      string
	watchThreshMB      int64
	editingOriginal    string
}

// NewConfigModel constructs and initializes a ConfigModel instance.
func NewConfigModel(cfg *config.Config, node *yaml.Node, path string, sched scheduler.Scheduler, bl *safety.Blacklist) ConfigModel {
	ti := textinput.New()
	ti.Prompt = "❯ "

	var st *scheduler.Status
	var schedActive, watchActive bool
	schedCron := "0 12 * * *"
	if sched != nil {
		st, _ = sched.Status()
		schedActive = sched.IsInstalled()
		watchActive = sched.IsWatchInstalled()
		if st != nil && st.CronExpr != "" {
			schedCron = st.CronExpr
		}
	}

	watchInterval := "6h"
	var watchThresh int64 = 2048
	preset := "safe"
	if cfg != nil {
		if cfg.Watch.Interval != "" {
			watchInterval = cfg.Watch.Interval
		}
		if cfg.Watch.ThresholdMB > 0 {
			watchThresh = cfg.Watch.ThresholdMB
		}
		for i, arg := range cfg.Scheduler.Args {
			if arg == "--preset" && i+1 < len(cfg.Scheduler.Args) {
				preset = cfg.Scheduler.Args[i+1]
				break
			}
		}
	}

	return ConfigModel{
		cfg:                cfg,
		astRoot:            node,
		configFile:         path,
		sched:              sched,
		blacklist:          bl,
		activeTab:          TabPaths,
		state:              ConfigStateNormal,
		cursor:             0,
		offset:             0,
		width:              80,
		height:             24,
		textInput:          ti,
		isDirty:            false,
		schedStatus:        st,
		schedActive:        schedActive,
		initialSchedActive: schedActive,
		schedFreq:          "daily",
		schedAtTime:        "12:00",
		schedPreset:        preset,
		schedCron:          schedCron,
		watchActive:        watchActive,
		initialWatchActive: watchActive,
		watchInterval:      watchInterval,
		watchThreshMB:      watchThresh,
	}
}

// Accessors for testing and inspection
func (m ConfigModel) ActiveTab() ConfigTab     { return m.activeTab }
func (m ConfigModel) State() ConfigState       { return m.state }
func (m ConfigModel) Cursor() int              { return m.cursor }
func (m ConfigModel) IsDirty() bool            { return m.isDirty }
func (m ConfigModel) StatusMessage() string    { return m.statusMsg }

// Init initializes the Bubble Tea program.
func (m ConfigModel) Init() tea.Cmd {
	return nil
}

// Update processes incoming Bubble Tea events.
func (m ConfigModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustScroll()
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case ConfigStateHelp:
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" || msg.String() == "enter" {
				m.state = ConfigStateNormal
			}
			return m, nil

		case ConfigStateConfirmExit:
			switch msg.String() {
			case "s", "enter":
				if err := m.save(); err != nil {
					m.state = ConfigStateOSError
					m.osErrorMsg = err.Error()
					return m, nil
				}
				return m, tea.Quit
			case "d":
				return m, tea.Quit
			case "c", "esc":
				m.state = ConfigStateNormal
				return m, nil
			}
			return m, nil

		case ConfigStateOSError:
			if msg.String() == "enter" || msg.String() == "esc" {
				m.state = ConfigStateNormal
			}
			return m, nil

		case ConfigStateEditingField, ConfigStateAddingPath:
			return m.handleEditingKeys(msg)

		case ConfigStateNormal:
			switch msg.String() {
			// Tab switching (D-01)
			case "tab":
				m.activeTab = (m.activeTab + 1) % 5
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "shift+tab":
				m.activeTab = (m.activeTab + 4) % 5
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "1":
				m.activeTab = TabPaths
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "2":
				m.activeTab = TabEcosystems
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "3":
				m.activeTab = TabSafety
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "4":
				m.activeTab = TabWatch
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil
			case "5":
				m.activeTab = TabScheduler
				m.cursor = 0
				m.offset = 0
				m.statusMsg = ""
				return m, nil

			// Vertical navigation (D-02)
			case "j", "down":
				maxC := m.maxCursorForTab()
				if m.cursor < maxC {
					m.cursor++
					m.adjustScroll()
				}
				return m, nil
			case "k", "up":
				if m.cursor > 0 {
					m.cursor--
					m.adjustScroll()
				}
				return m, nil

			// Boolean switch toggle (D-05)
			case " ":
				m.toggleCurrentItem()
				return m, nil

			case "left", "right":
				if m.activeTab == TabScheduler && (m.cursor == 1 || m.cursor == 3) {
					m.toggleCurrentItem()
					return m, nil
				}

			// Inline edit (D-06)
			case "e", "enter":
				m.startEditingCurrentItem()
				return m, nil

			// Path addition (D-07)
			case "a":
				if m.activeTab == TabPaths {
					m.state = ConfigStateAddingPath
					m.textInput.Reset()
					m.textInput.Focus()
					m.statusMsg = "Digite o novo caminho e pressione Enter:"
				}
				return m, nil

			// Path deletion (D-07)
			case "d":
				if m.activeTab == TabPaths {
					m.deleteCurrentPath()
				}
				return m, nil

			// Save (D-08, D-14)
			case "ctrl+s":
				if err := m.save(); err != nil {
					if m.state != ConfigStateOSError {
						m.statusMsg = fmt.Sprintf("⚠️ %v", err)
					}
				} else {
					m.statusMsg = "✓ Configuração e agendamento sincronizados com sucesso"
				}
				return m, nil

			// Help modal (D-03)
			case "?":
				m.state = ConfigStateHelp
				return m, nil

			// Exit (D-04)
			case "q", "esc", "ctrl+c":
				if m.isDirty {
					m.state = ConfigStateConfirmExit
					return m, nil
				}
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m *ConfigModel) maxCursorForTab() int {
	if m.cfg == nil {
		return 0
	}
	switch m.activeTab {
	case TabPaths:
		total := len(m.cfg.ScanPaths) + len(m.cfg.ExcludePaths)
		if total == 0 {
			return 0
		}
		return total - 1
	case TabEcosystems:
		return len(ecosystemMetas) - 1
	case TabSafety:
		return 4 // 0: trash_enabled, 1: retention_days, 2: max_size_mb, 3: mass_select, 4: history_retention
	case TabWatch:
		return 2 // 0: watch_daemon, 1: interval, 2: threshold_mb
	case TabScheduler:
		return 4 // 0: active, 1: freq, 2: at_time, 3: preset, 4: cron
	default:
		return 0
	}
}

func (m *ConfigModel) adjustScroll() {
	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	} else if m.cursor >= m.offset+visibleHeight {
		m.offset = m.cursor - visibleHeight + 1
	}
}

func (m *ConfigModel) getEcosystem(key string) *config.EcosystemConfig {
	if m.cfg == nil {
		return nil
	}
	switch key {
	case "docker":
		return &m.cfg.Ecosystems.Docker
	case "node":
		return &m.cfg.Ecosystems.Node
	case "go":
		return &m.cfg.Ecosystems.Go
	case "dotnet":
		return &m.cfg.Ecosystems.DotNet
	case "python":
		return &m.cfg.Ecosystems.Python
	case "rust":
		return &m.cfg.Ecosystems.Rust
	case "java":
		return &m.cfg.Ecosystems.Java
	case "nextjs":
		return &m.cfg.Ecosystems.NextJS
	case "bun":
		return &m.cfg.Ecosystems.Bun
	case "xcode":
		return &m.cfg.Ecosystems.Xcode
	case "ai":
		return &m.cfg.Ecosystems.AI
	case "pnpm":
		return &m.cfg.Ecosystems.Pnpm
	case "homebrew":
		return &m.cfg.Ecosystems.Homebrew
	case "cpp":
		return &m.cfg.Ecosystems.CPP
	case "flutter":
		return &m.cfg.Ecosystems.Flutter
	case "terraform":
		return &m.cfg.Ecosystems.Terraform
	case "cocoapods":
		return &m.cfg.Ecosystems.CocoaPods
	case "elixir":
		return &m.cfg.Ecosystems.Elixir
	case "shell":
		return &m.cfg.Ecosystems.Shell
	default:
		return nil
	}
}

func (m *ConfigModel) toggleCurrentItem() {
	if m.cfg == nil {
		return
	}
	switch m.activeTab {
	case TabEcosystems:
		if m.cursor >= 0 && m.cursor < len(ecosystemMetas) {
			eco := m.getEcosystem(ecosystemMetas[m.cursor].Key)
			if eco != nil {
				eco.Enabled = !eco.Enabled
				m.isDirty = true
			}
		}
	case TabSafety:
		if m.cursor == 0 {
			m.cfg.Safety.TrashEnabled = !m.cfg.Safety.TrashEnabled
			m.isDirty = true
		}
	case TabWatch:
		if m.cursor == 0 {
			m.watchActive = !m.watchActive
			m.isDirty = true
		}
	case TabScheduler:
		switch m.cursor {
		case 0:
			m.schedActive = !m.schedActive
			m.isDirty = true
		case 1:
			switch m.schedFreq {
			case "daily":
				m.schedFreq = "weekly"
			case "weekly":
				m.schedFreq = "hourly"
			case "hourly":
				m.schedFreq = "custom"
			default:
				m.schedFreq = "daily"
			}
			m.isDirty = true
		case 3:
			switch m.schedPreset {
			case "safe":
				m.schedPreset = "standard"
			case "standard":
				m.schedPreset = "aggressive"
			case "aggressive":
				m.schedPreset = "none"
			default:
				m.schedPreset = "safe"
			}
			m.isDirty = true
		}
	}
}

func (m *ConfigModel) startEditingCurrentItem() {
	if m.cfg == nil {
		return
	}
	switch m.activeTab {
	case TabPaths:
		totalScan := len(m.cfg.ScanPaths)
		if m.cursor < totalScan {
			p := m.cfg.ScanPaths[m.cursor]
			m.editingOriginal = p
			m.textInput.SetValue(p)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando caminho de varredura:"
		} else {
			idx := m.cursor - totalScan
			if idx < len(m.cfg.ExcludePaths) {
				p := m.cfg.ExcludePaths[idx]
				m.editingOriginal = p
				m.textInput.SetValue(p)
				m.textInput.Focus()
				m.textInput.CursorEnd()
				m.state = ConfigStateEditingField
				m.statusMsg = "Editando caminho de exclusão:"
			}
		}

	case TabEcosystems:
		if m.cursor >= 0 && m.cursor < len(ecosystemMetas) {
			meta := ecosystemMetas[m.cursor]
			eco := m.getEcosystem(meta.Key)
			if eco != nil {
				val := fmt.Sprintf("%d", eco.MaxAgeDays)
				m.editingOriginal = val
				m.textInput.SetValue(val)
				m.textInput.Focus()
				m.textInput.CursorEnd()
				m.state = ConfigStateEditingField
				m.statusMsg = fmt.Sprintf("Editando TTL de %s (dias > 0):", meta.Name)
			}
		}

	case TabSafety:
		switch m.cursor {
		case 1:
			val := fmt.Sprintf("%d", m.cfg.Safety.RetentionDays)
			m.editingOriginal = val
			m.textInput.SetValue(val)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando retenção da lixeira (dias >= 1):"
		case 2:
			val := fmt.Sprintf("%d", m.cfg.Safety.MaxSizeMB)
			m.editingOriginal = val
			m.textInput.SetValue(val)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando tamanho máximo da lixeira (MB >= 100):"
		case 3:
			val := int64(1024)
			if m.cfg.TUI.MassSelectThresholdMB > 0 {
				val = m.cfg.TUI.MassSelectThresholdMB
			}
			s := fmt.Sprintf("%d", val)
			m.editingOriginal = s
			m.textInput.SetValue(s)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando limite de seleção em massa (MB > 0):"
		case 4:
			val := 90
			if m.cfg.History.RetentionDays > 0 {
				val = m.cfg.History.RetentionDays
			}
			s := fmt.Sprintf("%d", val)
			m.editingOriginal = s
			m.textInput.SetValue(s)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando retenção do histórico (dias >= 1):"
		}

	case TabWatch:
		switch m.cursor {
		case 1:
			m.editingOriginal = m.watchInterval
			m.textInput.SetValue(m.watchInterval)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando intervalo do watch daemon (ex: 30m, 1h, 6h >= 30m):"
		case 2:
			s := fmt.Sprintf("%d", m.watchThreshMB)
			m.editingOriginal = s
			m.textInput.SetValue(s)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando threshold do watch daemon (MB > 0):"
		}

	case TabScheduler:
		switch m.cursor {
		case 1:
			m.editingOriginal = m.schedFreq
			m.textInput.SetValue(m.schedFreq)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando frequência (daily, weekly, hourly, custom):"
		case 2:
			m.editingOriginal = m.schedAtTime
			m.textInput.SetValue(m.schedAtTime)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando horário de agendamento (HH:MM entre 00:00 e 23:59):"
		case 3:
			m.editingOriginal = m.schedPreset
			m.textInput.SetValue(m.schedPreset)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando preset do agendador (safe, standard, aggressive, none):"
		case 4:
			m.editingOriginal = m.schedCron
			m.textInput.SetValue(m.schedCron)
			m.textInput.Focus()
			m.textInput.CursorEnd()
			m.state = ConfigStateEditingField
			m.statusMsg = "Editando expressão cron (5 campos cron):"
		}
	}
}

func (m *ConfigModel) validateCurrentInput(val string) error {
	trimmed := strings.TrimSpace(val)
	if m.state == ConfigStateAddingPath || (m.state == ConfigStateEditingField && m.activeTab == TabPaths) {
		if trimmed == "" {
			return fmt.Errorf("caminho não pode ser vazio")
		}
		if safety.IsCoreBlacklisted(trimmed) || (m.blacklist != nil && m.blacklist.IsBlacklisted(trimmed)) {
			return fmt.Errorf("caminho proibido pela Blacklist Imutável (.git, .env, .ssh, raízes do SO)")
		}
		return nil
	}

	switch m.activeTab {
	case TabEcosystems:
		n, err := strconv.Atoi(trimmed)
		if err != nil || n <= 0 {
			return fmt.Errorf("valor inválido: max_age_days deve ser > 0")
		}
	case TabSafety:
		switch m.cursor {
		case 1:
			n, err := strconv.Atoi(trimmed)
			if err != nil || n < 1 {
				return fmt.Errorf("valor inválido: safety.retention_days deve ser >= 1")
			}
		case 2:
			n, err := strconv.Atoi(trimmed)
			if err != nil || n < 100 {
				return fmt.Errorf("valor inválido: safety.max_size_mb deve ser >= 100")
			}
		case 3:
			n, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("valor inválido: mass_select_threshold_mb deve ser > 0")
			}
		case 4:
			n, err := strconv.Atoi(trimmed)
			if err != nil || n < 1 {
				return fmt.Errorf("valor inválido: history.retention_days deve ser >= 1")
			}
		}
	case TabWatch:
		switch m.cursor {
		case 1:
			d, err := time.ParseDuration(trimmed)
			if err != nil || d < 30*time.Minute {
				return fmt.Errorf("valor inválido: watch.interval deve ser >= 30m (ex: 1h, 6h)")
			}
		case 2:
			n, err := strconv.ParseInt(trimmed, 10, 64)
			if err != nil || n <= 0 {
				return fmt.Errorf("valor inválido: watch.threshold_mb deve ser > 0")
			}
		}
	case TabScheduler:
		switch m.cursor {
		case 1:
			lower := strings.ToLower(trimmed)
			if lower != "daily" && lower != "weekly" && lower != "hourly" && lower != "custom" {
				return fmt.Errorf("frequência inválida: escolha daily, weekly, hourly ou custom")
			}
		case 2:
			parts := strings.Split(trimmed, ":")
			if len(parts) == 2 && parts[0] == "" {
				min, errM := strconv.Atoi(parts[1])
				if errM != nil || min < 0 || min > 59 {
					return fmt.Errorf("horário inválido: esperado formato HH:MM ou :MM")
				}
				return nil
			}
			if len(parts) != 2 {
				return fmt.Errorf("horário inválido: esperado formato HH:MM (00:00 a 23:59)")
			}
			h, errH := strconv.Atoi(parts[0])
			min, errM := strconv.Atoi(parts[1])
			if errH != nil || errM != nil || h < 0 || h > 23 || min < 0 || min > 59 {
				return fmt.Errorf("horário inválido: esperado formato HH:MM (00:00 a 23:59)")
			}
		case 3:
			lower := strings.ToLower(trimmed)
			if lower != "safe" && lower != "standard" && lower != "aggressive" && lower != "none" {
				return fmt.Errorf("preset inválido: escolha safe, standard, aggressive ou none")
			}
		case 4:
			if err := scheduler.ValidateCron(trimmed); err != nil {
				return fmt.Errorf("expressão cron inválida: %w", err)
			}
		}
	}
	return nil
}

func (m ConfigModel) handleEditingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = ConfigStateNormal
		m.textInput.Blur()
		m.statusMsg = ""
		return m, nil

	case "enter":
		val := m.textInput.Value()
		if m.state == ConfigStateAddingPath {
			m.commitAddedPath(val)
			m.textInput.Blur()
			return m, nil
		}
		if err := m.validateCurrentInput(val); err != nil {
			m.statusMsg = "⚠️ " + err.Error()
			return m, nil
		}
		m.commitEditing(val)
		m.state = ConfigStateNormal
		m.textInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		if err := m.validateCurrentInput(m.textInput.Value()); err != nil {
			m.statusMsg = "⚠️ " + err.Error()
		} else {
			m.statusMsg = ""
		}
		return m, cmd
	}
}

func (m *ConfigModel) commitEditing(val string) {
	trimmed := strings.TrimSpace(val)
	if m.state == ConfigStateAddingPath {
		m.commitAddedPath(trimmed)
		return
	}

	if m.activeTab == TabPaths {
		if safety.IsCoreBlacklisted(trimmed) || (m.blacklist != nil && m.blacklist.IsBlacklisted(trimmed)) {
			m.statusMsg = "⚠️ Caminho proibido pela Blacklist Imutável (.git, .env, .ssh, raízes do SO)"
			m.state = ConfigStateNormal
			return
		}
		totalScan := len(m.cfg.ScanPaths)
		if m.cursor < totalScan {
			m.cfg.ScanPaths[m.cursor] = trimmed
		} else {
			idx := m.cursor - totalScan
			if idx < len(m.cfg.ExcludePaths) {
				m.cfg.ExcludePaths[idx] = trimmed
			}
		}
		m.isDirty = true
		m.statusMsg = fmt.Sprintf("Caminho atualizado: %s", trimmed)
		return
	}

	switch m.activeTab {
	case TabEcosystems:
		if m.cursor >= 0 && m.cursor < len(ecosystemMetas) {
			eco := m.getEcosystem(ecosystemMetas[m.cursor].Key)
			if eco != nil {
				n, _ := strconv.Atoi(trimmed)
				eco.MaxAgeDays = n
				m.isDirty = true
				m.statusMsg = fmt.Sprintf("TTL de %s atualizado para %d dias", ecosystemMetas[m.cursor].Name, n)
			}
		}
	case TabSafety:
		switch m.cursor {
		case 1:
			n, _ := strconv.Atoi(trimmed)
			m.cfg.Safety.RetentionDays = n
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Retenção da lixeira atualizada para %d dias", n)
		case 2:
			n, _ := strconv.Atoi(trimmed)
			m.cfg.Safety.MaxSizeMB = n
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Tamanho da lixeira atualizado para %d MB", n)
		case 3:
			n, _ := strconv.ParseInt(trimmed, 10, 64)
			m.cfg.TUI.MassSelectThresholdMB = n
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Limite de seleção em massa atualizado para %d MB", n)
		case 4:
			n, _ := strconv.Atoi(trimmed)
			m.cfg.History.RetentionDays = n
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Retenção do histórico atualizada para %d dias", n)
		}
	case TabWatch:
		switch m.cursor {
		case 1:
			m.cfg.Watch.Interval = trimmed
			m.watchInterval = trimmed
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Intervalo do watch daemon atualizado para %s", trimmed)
		case 2:
			n, _ := strconv.ParseInt(trimmed, 10, 64)
			m.cfg.Watch.ThresholdMB = n
			m.watchThreshMB = n
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Threshold do watch daemon atualizado para %d MB", n)
		}
	case TabScheduler:
		switch m.cursor {
		case 1:
			m.schedFreq = strings.ToLower(trimmed)
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Frequência do agendador atualizada para %s", m.schedFreq)
		case 2:
			m.schedAtTime = trimmed
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Horário do agendador atualizado para %s", trimmed)
		case 3:
			m.schedPreset = strings.ToLower(trimmed)
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Preset do agendador atualizado para %s", m.schedPreset)
		case 4:
			m.schedCron = trimmed
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Expressão cron atualizada para %s", trimmed)
		}
	}
}

func (m *ConfigModel) deleteCurrentPath() {
	if m.cfg == nil {
		return
	}
	totalScan := len(m.cfg.ScanPaths)
	if m.cursor < totalScan {
		if len(m.cfg.ScanPaths) > 0 {
			removed := m.cfg.ScanPaths[m.cursor]
			m.cfg.ScanPaths = append(m.cfg.ScanPaths[:m.cursor], m.cfg.ScanPaths[m.cursor+1:]...)
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Caminho removido: %s", removed)
			if m.cursor >= len(m.cfg.ScanPaths) && m.cursor > 0 {
				m.cursor--
			}
		}
	} else {
		idx := m.cursor - totalScan
		if idx < len(m.cfg.ExcludePaths) {
			removed := m.cfg.ExcludePaths[idx]
			m.cfg.ExcludePaths = append(m.cfg.ExcludePaths[:idx], m.cfg.ExcludePaths[idx+1:]...)
			m.isDirty = true
			m.statusMsg = fmt.Sprintf("Caminho excluído removido: %s", removed)
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
}

func (m *ConfigModel) commitAddedPath(path string) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		m.state = ConfigStateNormal
		return
	}

	// Inviolability check (NFR-03, T-05-03)
	if safety.IsCoreBlacklisted(cleanPath) || (m.blacklist != nil && m.blacklist.IsBlacklisted(cleanPath)) {
		m.statusMsg = "⚠️ Caminho proibido pela Blacklist Imutável (.git, .env, .ssh, raízes do SO)"
		m.state = ConfigStateNormal
		return
	}

	totalScan := len(m.cfg.ScanPaths)
	if m.cursor >= totalScan && len(m.cfg.ExcludePaths) > 0 {
		m.cfg.ExcludePaths = append(m.cfg.ExcludePaths, cleanPath)
		m.statusMsg = fmt.Sprintf("Caminho excluído adicionado: %s", cleanPath)
	} else {
		m.cfg.ScanPaths = append(m.cfg.ScanPaths, cleanPath)
		m.statusMsg = fmt.Sprintf("Caminho adicionado: %s", cleanPath)
	}
	m.isDirty = true
	m.state = ConfigStateNormal
}

func (m *ConfigModel) save() error {
	if m.cfg == nil {
		return nil
	}
	if err := m.cfg.Validate(); err != nil {
		return err
	}

	// Sanitize scheduler args & preset (T-05-04)
	cleanArgs, err := sanitizeSchedulerArgs(m.schedPreset, m.cfg.Scheduler.Args)
	if err != nil {
		return err
	}
	m.cfg.Scheduler.Args = cleanArgs

	// Sincronização com o OS Scheduler (D-14, D-15, D-16)
	if m.sched != nil {
		// 1. Scheduler
		if m.schedActive {
			cronExpr, err := m.resolveSchedulerCron()
			if err != nil {
				return m.failOSError(err)
			}
			if err := m.sched.Install(scheduler.Schedule{CronExpr: cronExpr}); err != nil {
				return m.failOSError(err)
			}
			m.initialSchedActive = true
		} else if m.initialSchedActive || m.sched.IsInstalled() {
			if err := m.sched.Remove(); err != nil && err != scheduler.ErrNotInstalled {
				return m.failOSError(err)
			}
			m.initialSchedActive = false
		}

		// 2. Watch Daemon
		if m.watchActive {
			dur, err := time.ParseDuration(m.watchInterval)
			if err != nil {
				return m.failOSError(fmt.Errorf("intervalo de watch inválido: %w", err))
			}
			if err := m.sched.InstallWatch(scheduler.WatchSchedule{Interval: dur}); err != nil {
				return m.failOSError(err)
			}
			m.initialWatchActive = true
		} else if m.initialWatchActive || m.sched.IsWatchInstalled() {
			if err := m.sched.RemoveWatch(); err != nil {
				return m.failOSError(err)
			}
			m.initialWatchActive = false
		}
	}

	// Persist AST if root exists (D-10, T-05-01)
	if m.astRoot != nil && m.configFile != "" {
		m.syncAST()
		if err := config.SaveConfigNode(m.configFile, m.astRoot); err != nil {
			return err
		}
	}
	m.isDirty = false
	return nil
}

func (m *ConfigModel) failOSError(err error) error {
	m.state = ConfigStateOSError
	m.osErrorMsg = err.Error()
	return err
}

func (m ConfigModel) resolveSchedulerCron() (string, error) {
	switch m.schedFreq {
	case "daily":
		return applyAtTimeToCron("0 12 * * *", m.schedAtTime)
	case "weekly":
		return applyAtTimeToCron("0 0 * * 0", m.schedAtTime)
	case "hourly":
		if m.schedAtTime != "" {
			parts := strings.Split(strings.TrimPrefix(m.schedAtTime, ":"), ":")
			minStr := parts[len(parts)-1]
			min, err := strconv.Atoi(minStr)
			if err != nil || min < 0 || min > 59 {
				return "", fmt.Errorf("horário inválido para hourly (esperado minuto 00-59): %s", m.schedAtTime)
			}
			return fmt.Sprintf("%d * * * *", min), nil
		}
		return "0 * * * *", nil
	case "custom":
		if err := scheduler.ValidateCron(m.schedCron); err != nil {
			return "", fmt.Errorf("expressão cron customizada inválida: %w", err)
		}
		return m.schedCron, nil
	default:
		return "0 12 * * *", nil
	}
}

func applyAtTimeToCron(baseCron, atTime string) (string, error) {
	if atTime == "" {
		return baseCron, nil
	}
	parts := strings.Split(atTime, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("formato inválido de horário (esperado HH:MM): %s", atTime)
	}
	h, errH := strconv.Atoi(parts[0])
	min, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil || h < 0 || h > 23 || min < 0 || min > 59 {
		return "", fmt.Errorf("horário fora dos limites (HH: 00-23, MM: 00-59): %s", atTime)
	}
	fields, err := scheduler.ParseCron(baseCron)
	if err != nil {
		return "", err
	}
	fields.Minute = fmt.Sprintf("%d", min)
	fields.Hour = fmt.Sprintf("%d", h)
	return fmt.Sprintf("%s %s %s %s %s", fields.Minute, fields.Hour, fields.DayOfMonth, fields.Month, fields.DayOfWeek), nil
}

func sanitizeSchedulerArgs(preset string, existingArgs []string) ([]string, error) {
	validPresets := map[string]bool{
		"none": true, "safe": true, "standard": true, "aggressive": true, "": true,
	}
	lowerPreset := strings.ToLower(strings.TrimSpace(preset))
	if !validPresets[lowerPreset] {
		return nil, fmt.Errorf("preset inválido: %s (permitidos: none, safe, standard, aggressive)", preset)
	}

	var cleaned []string
	if lowerPreset != "" && lowerPreset != "none" {
		cleaned = append(cleaned, "--preset", lowerPreset)
	}

	for i := 0; i < len(existingArgs); i++ {
		arg := existingArgs[i]
		if arg == "--preset" && i+1 < len(existingArgs) {
			i++ // skip existing preset value
			continue
		}
		if strings.ContainsAny(arg, ";`&$|><!*?()[]{}") {
			return nil, fmt.Errorf("argumento contém caracteres de shell proibidos: %s", arg)
		}
		cleaned = append(cleaned, arg)
	}
	return cleaned, nil
}

func (m *ConfigModel) syncAST() {
	if m.astRoot == nil || m.cfg == nil {
		return
	}
	rootMap := config.GetMappingNode(m.astRoot)
	if rootMap == nil {
		return
	}

	// Paths
	config.SetSequenceValues(rootMap, "scan_paths", m.cfg.ScanPaths)
	config.SetSequenceValues(rootMap, "exclude_paths", m.cfg.ExcludePaths)

	// Safety
	safetyMap := config.FindOrCreateMapping(rootMap, "safety")
	config.SetScalarValue(safetyMap, "trash_enabled", fmt.Sprintf("%t", m.cfg.Safety.TrashEnabled), "!!bool")
	config.SetScalarValue(safetyMap, "retention_days", fmt.Sprintf("%d", m.cfg.Safety.RetentionDays), "!!int")
	config.SetScalarValue(safetyMap, "max_size_mb", fmt.Sprintf("%d", m.cfg.Safety.MaxSizeMB), "!!int")

	// Ecosystems
	ecoMap := config.FindOrCreateMapping(rootMap, "ecosystems")
	for _, meta := range ecosystemMetas {
		eco := m.getEcosystem(meta.Key)
		if eco != nil {
			subMap := config.FindOrCreateMapping(ecoMap, meta.Key)
			config.SetScalarValue(subMap, "enabled", fmt.Sprintf("%t", eco.Enabled), "!!bool")
			config.SetScalarValue(subMap, "max_age_days", fmt.Sprintf("%d", eco.MaxAgeDays), "!!int")
		}
	}

	// Watch
	watchMap := config.FindOrCreateMapping(rootMap, "watch")
	config.SetScalarValue(watchMap, "interval", m.cfg.Watch.Interval, "!!str")
	config.SetScalarValue(watchMap, "threshold_mb", fmt.Sprintf("%d", m.cfg.Watch.ThresholdMB), "!!int")

	// Scheduler
	schedMap := config.FindOrCreateMapping(rootMap, "scheduler")
	config.SetSequenceValues(schedMap, "args", m.cfg.Scheduler.Args)

	// TUI & History
	tuiMap := config.FindOrCreateMapping(rootMap, "tui")
	config.SetScalarValue(tuiMap, "mass_select_threshold_mb", fmt.Sprintf("%d", m.cfg.TUI.MassSelectThresholdMB), "!!int")

	histMap := config.FindOrCreateMapping(rootMap, "history")
	config.SetScalarValue(histMap, "retention_days", fmt.Sprintf("%d", m.cfg.History.RetentionDays), "!!int")
}

// View renders the TUI screen.
func (m ConfigModel) View() string {
	if m.state == ConfigStateConfirmExit {
		return m.viewConfirmExit()
	}
	if m.state == ConfigStateHelp {
		return m.viewHelp()
	}
	if m.state == ConfigStateOSError {
		return m.viewOSError()
	}

	var b strings.Builder

	// 1. Header & Tabs Bar
	dirty := ""
	if m.isDirty {
		dirty = " " + dirtyIndicator
	}
	title := headerStyle.Render("DevClean Configuração") + dirty
	b.WriteString("\n  " + title + "\n\n")

	tabs := []string{
		"1: Caminhos",
		"2: Ecossistemas",
		"3: Segurança",
		"4: Watch",
		"5: Scheduler",
	}
	var renderedTabs []string
	for i, t := range tabs {
		if ConfigTab(i) == m.activeTab {
			renderedTabs = append(renderedTabs, tabActiveStyle.Render(t))
		} else {
			renderedTabs = append(renderedTabs, tabInactiveStyle.Render(t))
		}
	}
	b.WriteString("  " + strings.Join(renderedTabs, " ") + "\n\n")

	// 2. Active Tab Content
	switch m.activeTab {
	case TabPaths:
		b.WriteString(m.viewPaths())
	case TabEcosystems:
		b.WriteString(m.viewEcosystems())
	case TabSafety:
		b.WriteString(m.viewSafety())
	case TabWatch:
		b.WriteString(m.viewWatch())
	case TabScheduler:
		b.WriteString(m.viewScheduler())
	}

	// 3. Status Bar / Text Input
	b.WriteString("\n")
	if m.state == ConfigStateEditingField || m.state == ConfigStateAddingPath {
		prompt := "  " + searchPromptStyle.Render("Editar valor: ") + m.textInput.View() + "\n"
		if m.state == ConfigStateAddingPath {
			prompt = "  " + searchPromptStyle.Render("Adicionar caminho: ") + m.textInput.View() + "\n"
		}
		b.WriteString(prompt)
	}

	if m.statusMsg != "" {
		switch {
		case strings.HasPrefix(m.statusMsg, "⚠️"):
			b.WriteString("  " + statusAlertStyle.Render(m.statusMsg) + "\n")
		case strings.HasPrefix(m.statusMsg, "✓"):
			b.WriteString("  " + statusSavedStyle.Render(m.statusMsg) + "\n")
		default:
			b.WriteString("  " + selectedStyle.Render(m.statusMsg) + "\n")
		}
	} else {
		b.WriteString("\n")
	}

	// 4. Footer Shortcuts
	shortcuts := "  [Tab] Abas  [j/k] Navegar  [Space] Alternar  [e/Enter] Editar  [Ctrl+S] Salvar  [?] Ajuda  [q] Sair"
	if m.state == ConfigStateEditingField || m.state == ConfigStateAddingPath {
		shortcuts = "  [Enter] Confirmar  [Esc] Cancelar"
	} else if m.activeTab == TabPaths {
		shortcuts = "  [Tab] Abas  [j/k] Navegar  [a] Adicionar  [d] Remover  [e] Editar  [Ctrl+S] Salvar  [?] Ajuda  [q] Sair"
	}
	b.WriteString(helpStyle.Render(shortcuts) + "\n")

	return b.String()
}

func (m ConfigModel) viewPaths() string {
	var b strings.Builder
	b.WriteString("  " + groupHeaderStyle.Render("Caminhos de Varredura (scan_paths):") + "\n")
	if m.cfg == nil || len(m.cfg.ScanPaths) == 0 {
		b.WriteString("    " + helpStyle.Render("(nenhum caminho configurado — pressione 'a' para adicionar)") + "\n")
	} else {
		for i, p := range m.cfg.ScanPaths {
			cursorStr := "  "
			if m.cursor == i {
				cursorStr = "❯ "
				b.WriteString("  " + selectedStyle.Render(cursorStr+p) + "\n")
			} else {
				b.WriteString("  " + cursorStr + p + "\n")
			}
		}
	}

	b.WriteString("\n  " + groupHeaderStyle.Render("Caminhos Excluídos (exclude_paths):") + "\n")
	scanLen := 0
	if m.cfg != nil {
		scanLen = len(m.cfg.ScanPaths)
	}
	if m.cfg == nil || len(m.cfg.ExcludePaths) == 0 {
		b.WriteString("    " + helpStyle.Render("(nenhuma exclusão configurada)") + "\n")
	} else {
		for i, p := range m.cfg.ExcludePaths {
			cursorStr := "  "
			idx := scanLen + i
			if m.cursor == idx {
				cursorStr = "❯ "
				b.WriteString("  " + selectedStyle.Render(cursorStr+p) + "\n")
			} else {
				b.WriteString("  " + cursorStr + p + "\n")
			}
		}
	}
	return b.String()
}

func (m ConfigModel) viewEcosystems() string {
	var b strings.Builder
	activeCount := 0
	for _, meta := range ecosystemMetas {
		eco := m.getEcosystem(meta.Key)
		if eco != nil && eco.Enabled {
			activeCount++
		}
	}
	header := fmt.Sprintf("%d ecossistemas configurados (%d ativos):", len(ecosystemMetas), activeCount)
	b.WriteString("  " + groupHeaderStyle.Render(header) + "\n\n")

	visibleHeight := m.height - 8
	if visibleHeight < 5 {
		visibleHeight = 5
	}

	end := m.offset + visibleHeight
	if end > len(ecosystemMetas) {
		end = len(ecosystemMetas)
	}

	for i := m.offset; i < end; i++ {
		meta := ecosystemMetas[i]
		eco := m.getEcosystem(meta.Key)
		badge := disabledBadgeStyle.Render("[ ]")
		maxAge := 30
		failMode := "continue"
		if eco != nil {
			if eco.Enabled {
				badge = enabledBadgeStyle.Render("[x]")
			}
			maxAge = eco.MaxAgeDays
			if eco.FailMode != "" {
				failMode = eco.FailMode
			}
		}

		cursorStr := "  "
		line := fmt.Sprintf("%-12s  %s  TTL: %2dd  Modo: %-8s  // %s", meta.Name, badge, maxAge, failMode, meta.Description)
		if m.cursor == i {
			cursorStr = "❯ "
			b.WriteString("  " + selectedStyle.Render(cursorStr+line) + "\n")
		} else {
			b.WriteString("  " + cursorStr + line + "\n")
		}
	}

	if len(ecosystemMetas) > visibleHeight {
		b.WriteString("\n  " + helpStyle.Render(fmt.Sprintf("  [Role com j/k para ver todos — exibindo %d-%d de %d]", m.offset+1, end, len(ecosystemMetas))) + "\n")
	}

	return b.String()
}

func (m ConfigModel) viewSafety() string {
	var b strings.Builder
	b.WriteString("  " + groupHeaderStyle.Render("Parâmetros de Segurança e Lixeira:") + "\n\n")

	trashEnabled := true
	retentionDays := 30
	maxSizeMB := 10240
	massSelect := int64(1024)
	histRetention := 90

	if m.cfg != nil {
		trashEnabled = m.cfg.Safety.TrashEnabled
		retentionDays = m.cfg.Safety.RetentionDays
		maxSizeMB = m.cfg.Safety.MaxSizeMB
		if m.cfg.TUI.MassSelectThresholdMB > 0 {
			massSelect = m.cfg.TUI.MassSelectThresholdMB
		}
		if m.cfg.History.RetentionDays > 0 {
			histRetention = m.cfg.History.RetentionDays
		}
	}

	items := []struct {
		label string
		value string
	}{
		{"Mover para a Lixeira (trash_enabled)", func() string {
			if trashEnabled {
				return enabledBadgeStyle.Render("[x] Ativado")
			}
			return disabledBadgeStyle.Render("[ ] Desativado (exclusão permanente)")
		}()},
		{"Dias de retenção na lixeira (retention_days)", fmt.Sprintf("%d dias", retentionDays)},
		{"Tamanho máximo da lixeira (max_size_mb)", fmt.Sprintf("%d MB", maxSizeMB)},
		{"Limite para aviso de seleção em massa (mass_select)", fmt.Sprintf("%d MB", massSelect)},
		{"Retenção de histórico de limpezas (history)", fmt.Sprintf("%d dias", histRetention)},
	}

	for i, item := range items {
		cursorStr := "  "
		line := fmt.Sprintf("%-48s : %s", item.label, item.value)
		if m.cursor == i {
			cursorStr = "❯ "
			b.WriteString("  " + selectedStyle.Render(cursorStr+line) + "\n")
		} else {
			b.WriteString("  " + cursorStr + line + "\n")
		}
	}
	return b.String()
}

func (m ConfigModel) viewWatch() string {
	var b strings.Builder
	b.WriteString("  " + groupHeaderStyle.Render("Monitoramento em Segundo Plano (Watch Daemon):") + "\n\n")

	watchBadge := disabledBadgeStyle.Render("[ ] Desativado")
	if m.watchActive {
		watchBadge = enabledBadgeStyle.Render("[x] Ativado no Sistema")
	}

	items := []struct {
		label string
		value string
	}{
		{"Status do Daemon de monitoramento", watchBadge},
		{"Intervalo de checagem (watch.interval)", m.watchInterval},
		{"Limite de disco para alerta (watch.threshold_mb)", fmt.Sprintf("%d MB", m.watchThreshMB)},
	}

	for i, item := range items {
		cursorStr := "  "
		line := fmt.Sprintf("%-48s : %s", item.label, item.value)
		if m.cursor == i {
			cursorStr = "❯ "
			b.WriteString("  " + selectedStyle.Render(cursorStr+line) + "\n")
		} else {
			b.WriteString("  " + cursorStr + line + "\n")
		}
	}
	return b.String()
}

func (m ConfigModel) viewScheduler() string {
	var b strings.Builder
	b.WriteString("  " + groupHeaderStyle.Render("Agendamento de Limpeza Automática (Scheduler):") + "\n\n")

	// Cartão superior com status em tempo real do agendador nativo (D-13)
	provider := "Systemd / Cron (Linux)"
	if runtime.GOOS == "darwin" {
		provider = "Launchd (macOS)"
	}

	statusText := disabledBadgeStyle.Render("[INATIVO]")
	if m.schedActive {
		statusText = enabledBadgeStyle.Render("[● ATIVO]")
	}

	nextRun := "Não agendada"
	if m.schedStatus != nil && m.schedStatus.NextEstimate != nil {
		nextRun = m.schedStatus.NextEstimate.Format("2006-01-02 15:04")
	}

	lastRun := "Nenhuma execução registrada"
	if m.schedStatus != nil && m.schedStatus.LastResult != nil {
		lastRun = fmt.Sprintf("%s (%d alvos, %s liberados)",
			m.schedStatus.LastResult.Timestamp.Format("2006-01-02 15:04:05"),
			m.schedStatus.LastResult.TargetsCount,
			format.HumanSize(m.schedStatus.LastResult.BytesFreed),
		)
	}

	cardContent := fmt.Sprintf(
		"Provedor do SO  : %s\nStatus no SO    : %s\nPróxima execução: %s\nÚltima execução : %s",
		provider, statusText, nextRun, lastRun,
	)
	cardBox := modalBoxStyle.BorderForeground(lipgloss.Color("63")).Render(cardContent)
	b.WriteString("  " + cardBox + "\n\n")

	schedBadge := disabledBadgeStyle.Render("[ ] Inativo")
	if m.schedActive {
		schedBadge = enabledBadgeStyle.Render("[x] Ativo")
	}

	items := []struct {
		label string
		value string
	}{
		{"Status do Agendador no Sistema", schedBadge},
		{"Frequência da execução", m.schedFreq},
		{"Horário diário (se aplicável)", m.schedAtTime},
		{"Preset de limpeza associado", m.schedPreset},
		{"Expressão Cron customizada", m.schedCron},
	}

	for i, item := range items {
		cursorStr := "  "
		line := fmt.Sprintf("%-48s : %s", item.label, item.value)
		if m.cursor == i {
			cursorStr = "❯ "
			b.WriteString("  " + selectedStyle.Render(cursorStr+line) + "\n")
		} else {
			b.WriteString("  " + cursorStr + line + "\n")
		}
	}
	return b.String()
}

func (m ConfigModel) viewConfirmExit() string {
	content := fmt.Sprintf(
		"%s\n\nExistem alterações pendentes não salvas na configuração.\nDeseja salvar antes de fechar o DevClean?\n\n%s",
		modalTitleStyle.Render("MODIFICAÇÕES PENDENTES"),
		"[s / Enter] Salvar e Sair   [d] Descartar   [c / Esc] Cancelar",
	)
	box := modalBoxStyle.Render(content)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

func (m ConfigModel) viewHelp() string {
	content := fmt.Sprintf(
		"%s\n\n"+
			"Tab / Shift+Tab : Alterna entre as 5 abas de configuração\n"+
			"1 .. 5          : Salto direto para a aba correspondente\n"+
			"j / k, Setas    : Navega verticalmente pelos campos\n"+
			"Espaço          : Alterna switches booleanos ([x] / [ ])\n"+
			"e / Enter       : Edita valor inline (quando aplicável)\n"+
			"a / d           : Adiciona / Remove caminho (Aba Caminhos)\n"+
			"Ctrl+S          : Grava alterações no disco (config.yaml)\n"+
			"?               : Abre ou fecha esta janela de ajuda\n"+
			"q / Esc         : Sai da TUI (com confirmação se modificado)\n\n"+
			"%s",
		modalTitleStyle.Render("AJUDA - ATALHOS DO DEVCLEAN CONFIG"),
		helpStyle.Render("[Pressione Esc, ? ou Enter para retornar]"),
	)
	box := modalBoxStyle.Render(content)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}

func (m ConfigModel) viewOSError() string {
	content := fmt.Sprintf(
		"%s\n\nO sistema operacional retornou um erro ao sincronizar:\n%s\n\nA TUI foi mantida aberta e suas alterações não foram perdidas.\n\n%s",
		modalTitleStyle.Render("FALHA NA OPERAÇÃO DO SISTEMA"),
		m.osErrorMsg,
		helpStyle.Render("[Pressione Enter ou Esc para fechar]"),
	)
	box := modalBoxStyle.Render(content)
	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
	}
	return box
}
