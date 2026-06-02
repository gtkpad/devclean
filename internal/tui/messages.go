package tui

import "github.com/gabrielvp/devclean/internal/domain"

type scanCompleteMsg struct {
	results []domain.ScanResult
}

type deleteProgressMsg struct {
	current int
	total   int
	path    string
}

type deleteCompleteMsg struct {
	freed int64
	count int
}
