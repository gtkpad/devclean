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
)
