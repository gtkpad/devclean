package tui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	riskLowStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	riskMediumStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	riskHighStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	groupHeaderStyle = lipgloss.NewStyle().Bold(true)
	selectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dashboardStyle   = lipgloss.NewStyle().Bold(true).MarginBottom(1)

	// Analyzer and Purge styles
	protectedBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Background(lipgloss.Color("52")).Padding(0, 1)
	largeFileBadgeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")).Background(lipgloss.Color("58")).Padding(0, 1)
	symlinkBadgeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	barFilledStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	barEmptyStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	modalBoxStyle       = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("9")).
				Padding(1, 2)
	modalTitleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	statusAlertStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	searchPromptStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
)
