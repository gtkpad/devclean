package tui

import "github.com/gtkpad/devclean/internal/domain"

type scanCompleteMsg struct {
	results []domain.ScanResult
}

type ecosystemResultMsg struct {
	result domain.ScanResult
}

type scanDoneMsg struct{}


type deleteCompleteMsg struct {
	freed int64
	count int
}
