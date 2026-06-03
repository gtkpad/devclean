package tui

import (
	"fmt"
	"strings"

	"github.com/gtkpad/devclean/internal/domain"
)

func viewScanning(m model) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("  Scanning..."))
	b.WriteString("\n\n")

	for _, name := range m.ecoNames {
		status := m.ecoStatus[name]
		var icon, detail string
		switch status {
		case "waiting":
			icon = "─"
			detail = "(waiting)"
		case "scanning":
			icon = m.spinner.View()
			detail = "(scanning...)"
		case "done":
			icon = "✓"
			detail = ecosystemDoneSummary(m, name)
		case "error":
			icon = "✗"
			detail = "(error)"
		}
		line := fmt.Sprintf("  %s %s %s", icon, name, helpStyle.Render(detail))
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  q: quit"))
	b.WriteString("\n")
	return b.String()
}

func ecosystemDoneSummary(m model, ecosystem string) string {
	for _, r := range m.results {
		if r.Ecosystem == ecosystem && r.Error == nil {
			var total int64
			for _, t := range r.Targets {
				total += t.SizeBytes
			}
			return fmt.Sprintf("(%d targets, %s)", len(r.Targets), humanSize(total))
		}
	}
	return "(done)"
}

func viewSelecting(m model) string {
	var b strings.Builder

	// Dashboard header
	b.WriteString(dashboardStyle.Render(renderDashboard(m)))
	b.WriteString("\n")

	// Selected total
	totalBytes, count := m.selectedTotal()
	if count > 0 {
		b.WriteString(headerStyle.Render(fmt.Sprintf("  Selected: %d targets (%s)", count, humanSize(totalBytes))))
	} else {
		b.WriteString(helpStyle.Render("  No targets selected"))
	}
	b.WriteString("\n")

	// Filter/sort indicator
	if m.filter.isActive() {
		b.WriteString(helpStyle.Render("  " + m.filter.label()))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Grouped target list with scrolling
	visible := m.visibleHeight()
	end := m.offset + visible
	if end > len(m.visibleTargets) {
		end = len(m.visibleTargets)
	}

	lastGroup := -1
	for i := m.offset; i < end; i++ {
		ft := m.flatTargets[m.visibleTargets[i]]

		// Group separator
		if ft.groupIdx != lastGroup {
			lastGroup = ft.groupIdx
			groupSize, groupCount := ecosystemTotals(m, ft.ecosystem)
			header := fmt.Sprintf("── %s (%d targets, %s) ──", ft.ecosystem, groupCount, humanSize(groupSize))
			b.WriteString(groupHeaderStyle.Render(header))
			b.WriteString("\n")
		}

		// Target line
		line := renderTargetLine(m, i, ft)
		if i == m.cursor {
			b.WriteString(selectedStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.visibleTargets) > visible {
		b.WriteString(helpStyle.Render(fmt.Sprintf("\n  [%d/%d]", m.cursor+1, len(m.visibleTargets))))
		b.WriteString("\n")
	}

	// Filter bar
	if m.filter.fuzzyActive {
		filterLine := fmt.Sprintf("  / %s▎", m.filter.fuzzyText)
		b.WriteString(headerStyle.Render(filterLine))
		b.WriteString("\n")
	} else if m.filter.fuzzyText != "" {
		filterLine := fmt.Sprintf("  filter: %s", m.filter.fuzzyText)
		b.WriteString(helpStyle.Render(filterLine))
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("\n")
	if m.filter.fuzzyActive {
		b.WriteString(helpStyle.Render("  type to filter | Esc: exit filter | Enter: apply"))
	} else {
		b.WriteString(helpStyle.Render("  ↑↓/jk: navigate | space: toggle | /: filter | s: sort | r: risk | m: mass | a: all | enter: confirm | q: quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func renderDashboard(m model) string {
	var parts []string
	for _, r := range m.results {
		if r.Error != nil || len(r.Targets) == 0 {
			continue
		}
		var total int64
		for _, t := range r.Targets {
			total += t.SizeBytes
		}
		parts = append(parts, fmt.Sprintf("%s: %d targets (%s)", r.Ecosystem, len(r.Targets), humanSize(total)))
	}
	if len(parts) == 0 {
		return "  No ecosystems found"
	}
	return "  " + strings.Join(parts, " | ")
}

func renderTargetLine(m model, idx int, ft flatTarget) string {
	// Cursor indicator
	cursor := "  "
	if idx == m.cursor {
		cursor = "> "
	}

	// Checkbox
	checkbox := "[ ]"
	if m.selected[ft.target.ID] {
		checkbox = "[x]"
	}

	// Risk badge
	badge := riskBadge(ft.target.Risk)

	// Path (truncated if needed)
	maxPathLen := 50
	if m.width > 90 {
		maxPathLen = m.width - 40
	}
	path := ft.target.Path
	if len(path) > maxPathLen {
		path = "..." + path[len(path)-maxPathLen+3:]
	}

	// Size
	size := humanSize(ft.target.SizeBytes)

	// Smart icon
	icon := m.targetIcon(ft.target)

	return fmt.Sprintf("%s%s %s%s  %10s  %s", cursor, checkbox, icon, path, size, badge)
}

func riskBadge(risk domain.RiskLevel) string {
	switch risk {
	case domain.RiskLow:
		return riskLowStyle.Render("LOW")
	case domain.RiskMedium:
		return riskMediumStyle.Render("MED")
	case domain.RiskHigh:
		return riskHighStyle.Render("HIGH")
	default:
		return string(risk)
	}
}

func ecosystemTotals(m model, ecosystem string) (int64, int) {
	var total int64
	var count int
	for _, ft := range m.flatTargets {
		if ft.ecosystem == ecosystem {
			total += ft.target.SizeBytes
			count++
		}
	}
	return total, count
}

func viewConfirming(m model) string {
	var b strings.Builder
	totalBytes, count := m.selectedTotal()

	b.WriteString("\n")
	b.WriteString(headerStyle.Render("  Confirm deletion?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  Targets: %d selected\n", count))
	b.WriteString(fmt.Sprintf("  Space to free: %s\n", humanSize(totalBytes)))
	b.WriteString("\n  Paths:\n")

	shown := 0
	for _, ft := range m.flatTargets {
		if !m.selected[ft.target.ID] {
			continue
		}
		if shown >= 10 {
			remaining := count - shown
			b.WriteString(fmt.Sprintf("    + %d more\n", remaining))
			break
		}
		b.WriteString(fmt.Sprintf("    %s\n", ft.target.Path))
		shown++
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  [y] confirm  [n] cancel"))
	b.WriteString("\n")

	return b.String()
}

func viewDeleting(m model) string {
	return fmt.Sprintf("\n  %s Deleting targets...\n\n", m.spinner.View())
}

func viewDone(m model) string {
	if m.emptyResult {
		return "\n  ✅ All clean! No targets found.\n\n  Press any key to exit.\n\n"
	}
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(headerStyle.Render(fmt.Sprintf("  ✓ Done! Freed %s from %d targets.", humanSize(m.freedBytes), m.freedCount)))
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  Press any key to exit."))
	b.WriteString("\n")
	return b.String()
}
