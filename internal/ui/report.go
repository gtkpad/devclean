package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// EcosystemMetric armazena métricas agregadas por ecossistema de limpeza.
type EcosystemMetric struct {
	Name       string `json:"name"`
	FreedBytes int64  `json:"freed_bytes"`
	Count      int    `json:"count"`
}

// ReportData consolida os dados necessários para renderizar o relatório pós-limpeza.
type ReportData struct {
	Title          string            `json:"title"`
	TotalFreed     int64             `json:"total_freed"`
	TotalCount     int               `json:"total_count"`
	TrashCount     int               `json:"trash_count"`
	PermanentCount int               `json:"permanent_count"`
	Duration       time.Duration     `json:"duration"`
	DiskBefore     *uint64           `json:"disk_before,omitempty"`
	DiskAfter      *uint64           `json:"disk_after,omitempty"`
	Ecosystems     []EcosystemMetric `json:"ecosystems,omitempty"`
}

// CleanReportJSON representa as métricas completas de limpeza em formato JSON estruturado (D-11).
type CleanReportJSON struct {
	Title           string            `json:"title"`
	TotalFreed      int64             `json:"total_freed"`
	TotalFreedHuman string            `json:"total_freed_human"`
	TotalCount      int               `json:"total_count"`
	TrashCount      int               `json:"trash_count"`
	PermanentCount  int               `json:"permanent_count"`
	DurationMs      int64             `json:"duration_ms"`
	DiskBefore      *uint64           `json:"disk_before,omitempty"`
	DiskAfter       *uint64           `json:"disk_after,omitempty"`
	DiskDiff        *int64            `json:"disk_diff,omitempty"`
	Ecosystems      []EcosystemMetric `json:"ecosystems,omitempty"`
}

var forceTTYForTest *bool

func isOutputTerminal(w io.Writer) bool {
	if forceTTYForTest != nil {
		return *forceTTYForTest
	}
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(int(f.Fd()))
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// RenderReport renderiza um card estilizado com Lip Gloss pós-limpeza inspirado no Mole (tw93/mole).
// Respeita NO_COLOR e terminais não-TTY gerando texto sem códigos de escape ANSI.
func RenderReport(w io.Writer, data ReportData) {
	isTTY := isOutputTerminal(w)
	noColor := os.Getenv("NO_COLOR") != ""

	renderer := lipgloss.NewRenderer(w)
	if noColor || !isTTY {
		renderer.SetColorProfile(termenv.Ascii)
	} else if renderer.ColorProfile() == termenv.Ascii {
		renderer.SetColorProfile(termenv.TrueColor)
	}

	var (
		cardStyle      lipgloss.Style
		headerStyle    lipgloss.Style
		labelStyle     lipgloss.Style
		valueStyle     lipgloss.Style
		barFilledStyle lipgloss.Style
		barEmptyStyle  lipgloss.Style
	)

	if noColor || !isTTY {
		cardStyle = renderer.NewStyle().
			Border(lipgloss.NormalBorder()).
			Padding(0, 1)
		headerStyle = renderer.NewStyle()
		labelStyle = renderer.NewStyle()
		valueStyle = renderer.NewStyle()
		barFilledStyle = renderer.NewStyle()
		barEmptyStyle = renderer.NewStyle()
	} else {
		cardStyle = renderer.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(0, 1)
		headerStyle = renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
		labelStyle = renderer.NewStyle().Foreground(lipgloss.Color("245"))
		valueStyle = renderer.NewStyle().Bold(true)
		barFilledStyle = renderer.NewStyle().Foreground(lipgloss.Color("42"))
		barEmptyStyle = renderer.NewStyle().Foreground(lipgloss.Color("238"))
	}

	var sb strings.Builder
	title := data.Title
	if title == "" {
		title = "DevClean Summary"
	}
	sb.WriteString(headerStyle.Render("✨ " + title) + "\n\n")

	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Espaço Recuperado:"), valueStyle.Render(formatBytes(data.TotalFreed))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Duração Total:    "), valueStyle.Render(fmt.Sprintf("%d ms", data.Duration.Milliseconds()))))
	sb.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Alvos Processados:"), valueStyle.Render(fmt.Sprintf("%d (Lixeira: %d, Permanente: %d)", data.TotalCount, data.TrashCount, data.PermanentCount))))

	if data.DiskBefore != nil && data.DiskAfter != nil {
		diff := int64(*data.DiskAfter) - int64(*data.DiskBefore)
		sb.WriteString(fmt.Sprintf("%s %s ➔ %s (%s)\n",
			labelStyle.Render("Volume em Disco:  "),
			formatBytesUint64(*data.DiskBefore),
			formatBytesUint64(*data.DiskAfter),
			formatDiff(diff),
		))
	}

	if len(data.Ecosystems) > 0 && data.TotalFreed > 0 {
		sb.WriteString("\n" + labelStyle.Render("Detalhamento por Ecossistema:") + "\n")

		ecos := make([]EcosystemMetric, len(data.Ecosystems))
		copy(ecos, data.Ecosystems)
		sort.Slice(ecos, func(i, j int) bool {
			return ecos[i].FreedBytes > ecos[j].FreedBytes
		})

		for _, eco := range ecos {
			ratio := float64(eco.FreedBytes) / float64(data.TotalFreed)
			bar := renderMiniBar(ratio, 10, barFilledStyle, barEmptyStyle)
			pct := int(math.Round(ratio * 100))
			sb.WriteString(fmt.Sprintf("  %-10s %s  %8s (%2d%%)\n",
				eco.Name, bar, formatBytes(eco.FreedBytes), pct))
		}
	}

	fmt.Fprintln(w, cardStyle.Render(sb.String()))
}

func renderMiniBar(ratio float64, width int, filledStyle, emptyStyle lipgloss.Style) string {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	filled := int(math.Round(ratio * float64(width)))
	if filled > width {
		filled = width
	}
	empty := width - filled
	return "[" + filledStyle.Render(strings.Repeat("█", filled)) + emptyStyle.Render(strings.Repeat("░", empty)) + "]"
}

// FormatReportJSON serializa estruturadamente a struct CleanReportJSON com indentação de 2 espaços (D-11).
func FormatReportJSON(w io.Writer, data ReportData) error {
	var diskDiff *int64
	if data.DiskBefore != nil && data.DiskAfter != nil {
		diff := int64(*data.DiskAfter) - int64(*data.DiskBefore)
		diskDiff = &diff
	}

	title := data.Title
	if title == "" {
		title = "DevClean Summary"
	}

	report := CleanReportJSON{
		Title:           title,
		TotalFreed:      data.TotalFreed,
		TotalFreedHuman: formatBytes(data.TotalFreed),
		TotalCount:      data.TotalCount,
		TrashCount:      data.TrashCount,
		PermanentCount:  data.PermanentCount,
		DurationMs:      data.Duration.Milliseconds(),
		DiskBefore:      data.DiskBefore,
		DiskAfter:       data.DiskAfter,
		DiskDiff:        diskDiff,
		Ecosystems:      data.Ecosystems,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func formatBytes(bytes int64) string {
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

func formatBytesUint64(bytes uint64) string {
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

func formatDiff(diff int64) string {
	if diff > 0 {
		return "+" + formatBytes(diff)
	}
	if diff < 0 {
		return "-" + formatBytes(-diff)
	}
	return "0 B"
}
