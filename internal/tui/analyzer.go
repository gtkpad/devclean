package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gtkpad/devclean/internal/analyzer"
	"github.com/gtkpad/devclean/internal/deleter"
	"github.com/gtkpad/devclean/internal/domain"
	"github.com/gtkpad/devclean/internal/format"
	"github.com/gtkpad/devclean/internal/safety"
)

// AnalyzerState defines the view state of the disk analyzer.
type AnalyzerState int

const (
	StateNormal AnalyzerState = iota
	StateLargeFiles
	StateConfirmingDelete
	StateHelp
)

// SortCriteria defines sorting options for directory children.
type SortCriteria int

const (
	SortBySizeDesc SortCriteria = iota
	SortByNameAsc
	SortByItemsDesc
)

// AnalyzerModel is the Bubble Tea model for the interactive visual disk explorer.
type AnalyzerModel struct {
	root           *analyzer.Node
	currentDir     *analyzer.Node
	cursor         int
	offset         int
	width          int
	height         int
	state          AnalyzerState
	sortMode       SortCriteria
	spinner        spinner.Model
	scanner        *analyzer.Scanner
	blacklist      *safety.Blacklist
	coordinator    *deleter.Coordinator
	updateCh       chan analyzer.ScanUpdateMsg
	statusMessage  string
	largeFilesList []*analyzer.Node
	targetToDelete *analyzer.Node
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewAnalyzerModel creates and initializes an AnalyzerModel instance.
func NewAnalyzerModel(root *analyzer.Node, bl *safety.Blacklist, coord *deleter.Coordinator, width, height int, customScanner ...*analyzer.Scanner) AnalyzerModel {
	if root == nil {
		root = &analyzer.Node{
			Path:      ".",
			Name:      ".",
			Type:      analyzer.ItemDir,
			SizeBytes: 0,
		}
	}

	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	// Ensure parent pointers are connected and propagate protection
	var propagateProtection func(n *analyzer.Node)
	propagateProtection = func(n *analyzer.Node) {
		if n == nil {
			return
		}
		if (n.Parent != nil && n.Parent.IsProtected) ||
			safety.IsCoreBlacklisted(n.Path) ||
			(bl != nil && bl.IsBlacklisted(n.Path)) {
			n.IsProtected = true
		}
		for _, child := range n.Children {
			if child.Parent == nil {
				child.Parent = n
			}
			if n.IsProtected {
				child.IsProtected = true
			}
			propagateProtection(child)
		}
	}
	propagateProtection(root)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))

	ctx, cancel := context.WithCancel(context.Background())
	updateCh := make(chan analyzer.ScanUpdateMsg, 200)

	var sc *analyzer.Scanner
	if len(customScanner) > 0 && customScanner[0] != nil {
		sc = customScanner[0]
	} else if bl != nil {
		sc = analyzer.NewScanner(bl, true, 0, 0)
	}

	if sc != nil && sc.MinSize() > 0 && root != nil {
		var filtered []*analyzer.Node
		for _, c := range root.Children {
			if c.IsDir() || c.SizeBytes >= sc.MinSize() {
				filtered = append(filtered, c)
			}
		}
		root.Children = filtered
	}

	m := AnalyzerModel{
		root:           root,
		currentDir:     root,
		cursor:         0,
		offset:         0,
		width:          width,
		height:         height,
		state:          StateNormal,
		sortMode:       SortBySizeDesc,
		spinner:        sp,
		scanner:        sc,
		blacklist:      bl,
		coordinator:    coord,
		updateCh:       updateCh,
		statusMessage:  "",
		largeFilesList: make([]*analyzer.Node, 0),
		ctx:            ctx,
		cancel:         cancel,
	}

	m.collectLargeFiles(root)
	m.sortCurrentChildren()

	return m
}

func (m *AnalyzerModel) collectLargeFiles(n *analyzer.Node) {
	if n == nil {
		return
	}
	if n.IsLargeFile || (!n.IsDir() && n.SizeBytes >= analyzer.LargeFileThreshold) {
		m.largeFilesList = append(m.largeFilesList, n)
	}
	for _, child := range n.Children {
		m.collectLargeFiles(child)
	}
}

// Accessor methods for testing and inspection
func (m AnalyzerModel) Cursor() int {
	return m.cursor
}

func (m AnalyzerModel) CurrentDir() *analyzer.Node {
	return m.currentDir
}

func (m AnalyzerModel) State() AnalyzerState {
	return m.state
}

func (m AnalyzerModel) SortMode() SortCriteria {
	return m.sortMode
}

func (m AnalyzerModel) IsConfirmingDelete() bool {
	return m.state == StateConfirmingDelete
}

func (m AnalyzerModel) StatusMessage() string {
	return m.statusMessage
}

func (m AnalyzerModel) Offset() int {
	return m.offset
}

func (m AnalyzerModel) LargeFiles() []*analyzer.Node {
	return m.largeFilesList
}

// Init sets up initial commands (spinner and scan update listener).
func (m AnalyzerModel) Init() tea.Cmd {
	if m.scanner != nil && m.root != nil {
		var toScan []*analyzer.Node
		for _, child := range m.root.Children {
			if child.IsDir() && !child.Scanned {
				toScan = append(toScan, child)
			}
		}

		go func(targets []*analyzer.Node) {
			defer close(m.updateCh)
			for _, child := range targets {
				_, _, _ = m.scanner.CalculateDirSize(m.ctx, child, m.updateCh)
			}
		}(toScan)
	}

	return tea.Batch(m.spinner.Tick, waitForScanUpdate(m.updateCh))
}

func waitForScanUpdate(ch <-chan analyzer.ScanUpdateMsg) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// Update handles incoming messages and events in the Elm loop.
func (m AnalyzerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.adjustScroll()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case analyzer.ScanUpdateMsg:
		m.applyScanUpdate(msg)
		return m, waitForScanUpdate(m.updateCh)

	case deleteResultMsg:
		if msg.success && msg.target != nil {
			if m.currentDir != nil {
				var remaining []*analyzer.Node
				for _, c := range m.currentDir.Children {
					if c.Path != msg.target.Path {
						remaining = append(remaining, c)
					}
				}
				m.currentDir.Children = remaining

				// Decrement size and count from ancestors
				for p := m.currentDir; p != nil; p = p.Parent {
					p.SizeBytes -= msg.target.SizeBytes
					if p.SizeBytes < 0 {
						p.SizeBytes = 0
					}
					p.ItemCount -= msg.target.ItemCount
					if p.ItemCount < 0 {
						p.ItemCount = 0
					}
				}

				// Remove from largeFilesList if present
				var remainingLarge []*analyzer.Node
				for _, lf := range m.largeFilesList {
					if lf.Path != msg.target.Path {
						remainingLarge = append(remainingLarge, lf)
					}
				}
				m.largeFilesList = remainingLarge

				if m.cursor >= len(m.currentDir.Children) {
					if len(m.currentDir.Children) > 0 {
						m.cursor = len(m.currentDir.Children) - 1
					} else {
						m.cursor = 0
					}
				}
				m.statusMessage = "Item movido para a lixeira com sucesso."
				m.adjustScroll()
			}
		} else if msg.err != nil {
			m.statusMessage = fmt.Sprintf("Erro ao mover para a lixeira: %v", msg.err)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.state {
		case StateConfirmingDelete:
			switch msg.String() {
			case "y", "enter":
				m.state = StateNormal
				target := m.targetToDelete
				m.targetToDelete = nil
				return m, m.deleteNode(target)
			case "n", "esc", "q":
				m.state = StateNormal
				m.targetToDelete = nil
				m.statusMessage = "Exclusão cancelada."
				return m, nil
			}

		case StateLargeFiles:
			switch msg.String() {
			case "esc", "f", "tab", "q":
				m.state = StateNormal
				return m, nil
			}

		case StateHelp:
			switch msg.String() {
			case "esc", "?", "i", "q":
				m.state = StateNormal
				return m, nil
			}

		case StateNormal:
			switch msg.String() {
			case "q", "ctrl+c":
				if m.cancel != nil {
					m.cancel()
				}
				return m, tea.Quit

			case "j", "down":
				if m.currentDir != nil && m.cursor < len(m.currentDir.Children)-1 {
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

			case "l", "enter", "right":
				if m.currentDir != nil && len(m.currentDir.Children) > 0 && m.cursor < len(m.currentDir.Children) {
					selected := m.currentDir.Children[m.cursor]
					if selected.IsProtected || safety.IsCoreBlacklisted(selected.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(selected.Path)) {
						m.statusMessage = "⚠️ Diretório protegido pela Blacklist Imutável (navegação bloqueada)"
						return m, nil
					}
					if selected.IsDir() {
						if len(selected.Children) == 0 && m.scanner != nil {
							if scanned, err := m.scanner.ScanImmediate(selected.Path); err == nil && scanned != nil {
								selected.Children = scanned.Children
								for _, c := range selected.Children {
									c.Parent = selected
									if selected.IsProtected || safety.IsCoreBlacklisted(c.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(c.Path)) {
										c.IsProtected = true
									}
								}
								if m.scanner.MinSize() > 0 {
									var filtered []*analyzer.Node
									for _, c := range selected.Children {
										if c.IsDir() || c.SizeBytes >= m.scanner.MinSize() {
											filtered = append(filtered, c)
										}
									}
									selected.Children = filtered
								}
							}
						}
						m.currentDir = selected
						m.cursor = 0
						m.offset = 0
						m.sortCurrentChildren()
					}
				}
				m.statusMessage = ""
				return m, nil

			case "h", "esc", "backspace", "left":
				if m.currentDir != nil && m.currentDir.Parent != nil {
					m.currentDir = m.currentDir.Parent
					m.cursor = 0
					m.offset = 0
					m.sortCurrentChildren()
				}
				m.statusMessage = ""
				return m, nil

			case "s":
				m.sortMode = (m.sortMode + 1) % 3
				m.sortCurrentChildren()
				return m, nil

			case "f", "tab":
				m.state = StateLargeFiles
				return m, nil

			case "?", "i":
				m.state = StateHelp
				return m, nil

			case "d":
				if m.currentDir == nil || len(m.currentDir.Children) == 0 || m.cursor >= len(m.currentDir.Children) {
					return m, nil
				}
				target := m.currentDir.Children[m.cursor]
				isProt := target.IsProtected || safety.IsCoreBlacklisted(target.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(target.Path))
				if !isProt {
					for p := target.Parent; p != nil; p = p.Parent {
						if p.IsProtected || safety.IsCoreBlacklisted(p.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(p.Path)) {
							isProt = true
							break
						}
					}
				}
				if isProt {
					target.IsProtected = true
					m.statusMessage = "⚠️ Item protegido pela Blacklist Imutável (exclusão bloqueada)"
					return m, nil
				}
				m.targetToDelete = target
				m.state = StateConfirmingDelete
				return m, nil
			}
		}
	}

	return m, nil
}

func (m *AnalyzerModel) deleteNode(target *analyzer.Node) tea.Cmd {
	if target == nil {
		return nil
	}

	if target.IsProtected || safety.IsCoreBlacklisted(target.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(target.Path)) {
		return func() tea.Msg {
			return deleteResultMsg{
				success:    false,
				target:     target,
				currentDir: m.currentDir,
				err:        fmt.Errorf("item protegido pela Blacklist Imutável: exclusão bloqueada"),
			}
		}
	}

	coordinator := m.coordinator
	currentDir := m.currentDir

	return func() tea.Msg {
		if coordinator != nil {
			byEco := map[string][]domain.Target{
				"filesystem": {
					{
						ID:        target.Path,
						Name:      target.Name,
						Path:      target.Path,
						SizeBytes: target.SizeBytes,
						Risk:      domain.RiskLow,
						Category:  "disk-analysis",
					},
				},
			}
			opts := deleter.Options{
				Mode: "tui",
			}
			res, err := coordinator.Delete(context.Background(), byEco, opts)
			if err != nil {
				return deleteResultMsg{success: false, target: target, currentDir: currentDir, err: err}
			}
			if res != nil && len(res.Errors) > 0 {
				return deleteResultMsg{success: false, target: target, currentDir: currentDir, err: res.Errors[0]}
			}
			if res == nil || res.TotalCount == 0 || len(res.CleanedPaths) == 0 {
				return deleteResultMsg{
					success:    false,
					target:     target,
					currentDir: currentDir,
					err:        fmt.Errorf("nenhum arquivo foi excluído (operação bloqueada por segurança ou alvo inexistente)"),
				}
			}
		}

		return deleteResultMsg{success: true, target: target, currentDir: currentDir}
	}
}

type deleteResultMsg struct {
	success    bool
	target     *analyzer.Node
	currentDir *analyzer.Node
	err        error
}

func findNodeByPath(root *analyzer.Node, path string) *analyzer.Node {
	if root == nil {
		return nil
	}
	if root.Path == path {
		return root
	}
	for _, child := range root.Children {
		if found := findNodeByPath(child, path); found != nil {
			return found
		}
	}
	return nil
}

func (m *AnalyzerModel) applyScanUpdate(msg analyzer.ScanUpdateMsg) {
	node := findNodeByPath(m.root, msg.Path)
	if node != nil {
		node.SizeBytes = msg.SizeBytes
		node.ItemCount = msg.ItemCount
		node.Scanned = msg.Scanned
		if node.SizeBytes >= analyzer.LargeFileThreshold && !node.IsDir() {
			node.IsLargeFile = true
			found := false
			for _, lf := range m.largeFilesList {
				if lf.Path == node.Path {
					found = true
					break
				}
			}
			if !found {
				m.largeFilesList = append(m.largeFilesList, node)
			}
		}
	}

	if m.scanner != nil && m.scanner.MinSize() > 0 && m.currentDir != nil {
		var filtered []*analyzer.Node
		for _, c := range m.currentDir.Children {
			if !c.Scanned || c.SizeBytes >= m.scanner.MinSize() {
				filtered = append(filtered, c)
			}
		}
		m.currentDir.Children = filtered
		if m.cursor >= len(m.currentDir.Children) {
			if len(m.currentDir.Children) > 0 {
				m.cursor = len(m.currentDir.Children) - 1
			} else {
				m.cursor = 0
			}
		}
	}

	m.sortCurrentChildren()
}

func (m *AnalyzerModel) sortCurrentChildren() {
	if m.currentDir == nil || len(m.currentDir.Children) == 0 {
		return
	}
	sortChildren(m.currentDir.Children, m.sortMode)
}

func sortChildren(children []*analyzer.Node, mode SortCriteria) {
	sort.SliceStable(children, func(i, j int) bool {
		a, b := children[i], children[j]
		switch mode {
		case SortBySizeDesc:
			if a.SizeBytes != b.SizeBytes {
				return a.SizeBytes > b.SizeBytes
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortByNameAsc:
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		case SortByItemsDesc:
			if a.ItemCount != b.ItemCount {
				return a.ItemCount > b.ItemCount
			}
			if a.SizeBytes != b.SizeBytes {
				return a.SizeBytes > b.SizeBytes
			}
			return strings.ToLower(a.Name) < strings.ToLower(b.Name)
		default:
			return a.SizeBytes > b.SizeBytes
		}
	})
}

func (m *AnalyzerModel) adjustScroll() {
	visibleRows := m.visibleRows()
	if visibleRows <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}
}

func (m AnalyzerModel) visibleRows() int {
	const overhead = 7
	h := m.height - overhead
	if h < 1 {
		return 1
	}
	return h
}

// RenderProportionBar generates the [████    ] proportion bar.
func RenderProportionBar(sizeBytes, maxBytes int64, width int) string {
	if width <= 0 {
		return "[]"
	}
	if maxBytes <= 0 || sizeBytes <= 0 {
		return fmt.Sprintf("[%s]", barEmptyStyle.Render(strings.Repeat(" ", width)))
	}
	fraction := float64(sizeBytes) / float64(maxBytes)
	if fraction > 1.0 {
		fraction = 1.0
	}
	filled := int(fraction * float64(width))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return fmt.Sprintf("[%s%s]", barFilledStyle.Render(strings.Repeat("█", filled)), barEmptyStyle.Render(strings.Repeat(" ", empty)))
}

// View renders the TUI screen.
func (m AnalyzerModel) View() string {
	if m.height < 5 {
		return "Tela muito pequena para renderizar o explorador de disco."
	}

	switch m.state {
	case StateLargeFiles:
		return m.viewLargeFiles()
	case StateHelp:
		return m.viewHelp()
	case StateConfirmingDelete:
		return m.viewConfirmingDelete()
	default:
		return m.viewNormal()
	}
}

func (m AnalyzerModel) viewNormal() string {
	var b strings.Builder

	// Header: breadcrumbs, spinner, item totals
	dirPath := "/"
	if m.currentDir != nil {
		dirPath = m.currentDir.Breadcrumbs()
	}
	var totalSize int64
	var totalItems int
	if m.currentDir != nil {
		totalSize = m.currentDir.SizeBytes
		totalItems = m.currentDir.ItemCount
	}

	headerText := fmt.Sprintf("DevClean Analyze: %s", dirPath)
	headerMeta := fmt.Sprintf("Total: %s (%d itens)", format.HumanSize(totalSize), totalItems)
	b.WriteString(headerStyle.Render(headerText))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render(headerMeta))
	b.WriteString("\n\n")

	// Table list
	if m.currentDir == nil || len(m.currentDir.Children) == 0 {
		b.WriteString("  (diretório vazio)\n")
	} else {
		var maxBytes int64
		for _, c := range m.currentDir.Children {
			if c.SizeBytes > maxBytes {
				maxBytes = c.SizeBytes
			}
		}

		visible := m.visibleRows()
		end := m.offset + visible
		if end > len(m.currentDir.Children) {
			end = len(m.currentDir.Children)
		}

		for i := m.offset; i < end; i++ {
			c := m.currentDir.Children[i]
			cursorMark := "  "
			if i == m.cursor {
				cursorMark = "> "
			}

			// Type indicator
			prefix := "📁 "
			if !c.IsDir() {
				prefix = "📄 "
			}
			if c.IsSymlink() {
				prefix = "🔗 "
			}

			// Badges
			var badges []string
			if c.IsProtected || safety.IsCoreBlacklisted(c.Path) || (m.blacklist != nil && m.blacklist.IsBlacklisted(c.Path)) {
				badges = append(badges, protectedBadgeStyle.Render("[🔒 PROTEGIDO]"))
			}
			if c.IsLargeFile {
				badges = append(badges, largeFileBadgeStyle.Render("[>100M]"))
			}
			if c.IsSymlink() {
				badges = append(badges, symlinkBadgeStyle.Render("[@symlink]"))
			}

			badgeStr := ""
			if len(badges) > 0 {
				badgeStr = " " + strings.Join(badges, " ")
			}

			name := c.Name
			if c.IsDir() && !strings.HasSuffix(name, "/") {
				name += "/"
			}

			bar := RenderProportionBar(c.SizeBytes, maxBytes, 10)
			sizeStr := fmt.Sprintf("%8s", format.HumanSize(c.SizeBytes))

			line := fmt.Sprintf("%s%s%-28s %s %s%s", cursorMark, prefix, truncateString(name, 28), bar, sizeStr, badgeStr)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(line)
			}
			b.WriteString("\n")
		}
	}

	// Status / Alerts
	b.WriteString("\n")
	if m.statusMessage != "" {
		b.WriteString(statusAlertStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	// Footer shortcuts
	sortName := "Tamanho ↓"
	switch m.sortMode {
	case SortByNameAsc:
		sortName = "Nome ↑"
	case SortByItemsDesc:
		sortName = "Itens ↓"
	}

	shortcuts := fmt.Sprintf("[j/k] Navegar • [l/Enter] Abrir • [h/Esc] Subir • [s] Ordenar (%s) • [f] Maiores • [?] Ajuda • [d] Lixeira • [q] Sair", sortName)
	b.WriteString(helpStyle.Render(shortcuts))

	return b.String()
}

func (m AnalyzerModel) viewLargeFiles() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("ARQUIVOS GIGANTES ACUMULADOS (>100MB)"))
	b.WriteString("\n\n")

	if len(m.largeFilesList) == 0 {
		b.WriteString("  Nenhum arquivo maior que 100MB encontrado.\n\n")
	} else {
		for i, f := range m.largeFilesList {
			line := fmt.Sprintf(" %2d. %-45s %10s %s", i+1, truncateString(f.Path, 45), format.HumanSize(f.SizeBytes), largeFileBadgeStyle.Render("[>100M]"))
			b.WriteString(line)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("Pressione [Esc/Tab/f] para voltar à navegação normal."))
	return b.String()
}

func (m AnalyzerModel) viewHelp() string {
	var b strings.Builder
	b.WriteString(headerStyle.Render("DevClean Analyze — Ajuda & Atalhos Vim"))
	b.WriteString("\n\n")

	b.WriteString("  j / Seta Baixo    : Mover cursor para a próxima linha\n")
	b.WriteString("  k / Seta Cima     : Mover cursor para a linha anterior\n")
	b.WriteString("  l / Enter         : Abrir diretório selecionado\n")
	b.WriteString("  h / Backspace     : Voltar ao diretório pai\n")
	b.WriteString("  s                 : Ciclar ordenação (Tamanho ↓ / Nome ↑ / Itens ↓)\n")
	b.WriteString("  f / Tab           : Abrir/fechar painel de arquivos gigantes (>100MB)\n")
	b.WriteString("  d                 : Excluir item com confirmação para a lixeira\n")
	b.WriteString("  ? / i             : Exibir ou ocultar este painel de ajuda\n")
	b.WriteString("  q / Ctrl+C        : Sair do explorador de disco\n\n")

	b.WriteString(statusAlertStyle.Render("  🔒 Itens protegidos pela Blacklist Imutável (.git, .env, .ssh) não podem ser excluídos.\n\n"))
	b.WriteString(helpStyle.Render("Pressione [Esc/?/q] para fechar esta ajuda."))
	return b.String()
}

func (m AnalyzerModel) viewConfirmingDelete() string {
	targetName := ""
	targetPath := ""
	var targetSize int64
	if m.targetToDelete != nil {
		targetName = m.targetToDelete.Name
		targetPath = m.targetToDelete.Path
		targetSize = m.targetToDelete.SizeBytes
	}

	content := fmt.Sprintf(
		"%s\n\nAlvo: %s (%s)\nCaminho: %s\n\n%s\n\n%s",
		modalTitleStyle.Render("CONFIRMAR EXCLUSÃO PARA A LIXEIRA"),
		targetName,
		format.HumanSize(targetSize),
		targetPath,
		"O item será movido com segurança para a lixeira reversível do DevClean.",
		selectedStyle.Render("[y/Enter] Confirmar e mover para a lixeira   [n/Esc] Cancelar"),
	)

	box := modalBoxStyle.Render(content)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
