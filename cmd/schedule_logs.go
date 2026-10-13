package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/gtkpad/devclean/internal/platform"
	"github.com/gtkpad/devclean/internal/scheduler"
	"github.com/spf13/cobra"
)

func newScheduleLogsCmd() *cobra.Command {
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Display scheduled cleanup history and logs",
		Long:  "Inspects automated schedule execution logs with Lip Gloss table visualization, real-time follow, JSON export, and log cleanup.",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolver := platform.New()
			paths := resolver.Resolve()

			logDir := paths.LogDir
			if logDir == "" {
				home, _ := os.UserHomeDir()
				logDir = filepath.Join(home, ".local", "state", "devclean", "logs")
			}
			logPath := filepath.Join(logDir, "schedule.log")

			fi, err := os.Stat(logPath)
			if err != nil || fi.Size() == 0 {
				return renderLogsEmptyState(cmd.OutOrStdout(), paths.ConfigDir)
			}

			file, err := os.Open(logPath)
			if err != nil {
				return fmt.Errorf("opening schedule log: %w", err)
			}
			defer file.Close()

			runs, err := scheduler.ParseLogRuns(file)
			if err != nil {
				return fmt.Errorf("parsing schedule logs: %w", err)
			}

			if len(runs) == 0 {
				return renderLogsEmptyState(cmd.OutOrStdout(), paths.ConfigDir)
			}

			renderLogsTable(cmd.OutOrStdout(), runs, lines)
			return nil
		},
	}

	cmd.Flags().IntVarP(&lines, "lines", "n", 5, "number of recent runs to display")

	return cmd
}

func renderStatusBadge(status string) string {
	switch strings.ToUpper(status) {
	case "OK":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")).Render("[SUCESSO]")
	case "PARTIAL":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render("[PARCIAL]")
	case "FAIL":
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196")).Render("[FALHA]")
	default:
		return fmt.Sprintf("[%s]", status)
	}
}

func renderLogsTable(w io.Writer, runs []scheduler.LogRun, lines int) {
	if lines <= 0 {
		lines = 5
	}

	selectedRuns := runs
	if len(runs) > lines {
		selectedRuns = runs[len(runs)-lines:]
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(borderStyle).
		Headers("DATA / HORA", "STATUS", "ALVOS", "LIBERADO", "DURAÇÃO").
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return lipgloss.NewStyle()
		})

	for _, r := range selectedRuns {
		timeStr := r.StartTime.Format("2006-01-02 15:04:05")
		badge := renderStatusBadge(r.Status)
		targetsStr := fmt.Sprintf("%d", r.TargetsCount)
		freedStr := r.FreedHuman
		if freedStr == "" {
			freedStr = "0 B"
		}
		durStr := r.Duration
		if durStr == "" {
			durStr = "—"
		}
		t.Row(timeStr, badge, targetsStr, freedStr, durStr)
	}

	fmt.Fprintln(w, headerStyle.Render("📅 Histórico de Execuções do Agendador"))
	fmt.Fprintln(w, t.Render())

	// Most recent run detail (D-13)
	recent := runs[len(runs)-1]
	fmt.Fprintf(w, "📋 Execução Mais Recente (%s):\n", recent.StartTime.Format("2006-01-02 15:04:05"))
	if recent.Version != "" || len(recent.Args) > 0 {
		argsStr := strings.Join(recent.Args, " ")
		if argsStr == "" {
			argsStr = "(padrão)"
		}
		fmt.Fprintf(w, "  Versão: %s | Argumentos: %s\n", recent.Version, argsStr)
	}
	if recent.Body != "" {
		fmt.Fprintln(w, "  Saída:")
		for _, l := range strings.Split(strings.TrimSpace(recent.Body), "\n") {
			fmt.Fprintf(w, "    %s\n", l)
		}
	}
	if len(recent.ErrorLines) > 0 {
		errStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
		fmt.Fprintln(w, errStyle.Render("  ⚠️ Erros/Alertas Detectados:"))
		for _, el := range recent.ErrorLines {
			fmt.Fprintf(w, "    %s\n", errStyle.Render(el))
		}
	}

	// Footer tips (D-13)
	footerStyle := lipgloss.NewStyle().Faint(true)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, footerStyle.Render("💡 Dicas:"))
	fmt.Fprintln(w, footerStyle.Render("  • devclean schedule logs -f           Streaming contínuo em tempo real (--follow)"))
	fmt.Fprintln(w, footerStyle.Render("  • devclean schedule logs -n 10        Exibir mais execuções (--lines)"))
	fmt.Fprintln(w, footerStyle.Render("  • devclean schedule logs --format json Exportar histórico em formato JSON"))
	fmt.Fprintln(w, footerStyle.Render("  • devclean schedule logs --clear       Remover histórico de arquivos de log"))
}

func renderLogsEmptyState(w io.Writer, configDir string) error {
	sched, err := schedulerFactory(configDir, "")
	var st *scheduler.Status
	if err == nil {
		st, _ = sched.Status()
	}
	if st == nil {
		schedNormal := scheduler.New(configDir)
		st, _ = schedNormal.Status()
	}

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	fmt.Fprintln(w, headerStyle.Render("📅 Status do Agendador"))
	if st != nil && st.Active {
		fmt.Fprintln(w, "  Status:      Ativo")
		if st.CronExpr != "" {
			fmt.Fprintf(w, "  Frequência:  %s\n", st.CronExpr)
		}
		if st.NextEstimate != nil {
			fmt.Fprintf(w, "  Próxima:     %s\n", st.NextEstimate.Format("2006-01-02 15:04:05"))
		}
	} else {
		fmt.Fprintln(w, "  Status:      Inativo (nenhum agendamento registrado)")
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Nenhum log encontrado. O agendador ainda não executou. Para executar uma limpeza imediata, execute: devclean clean")
	return nil
}
