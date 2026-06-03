package tui

import "github.com/charmbracelet/bubbletea"

type keyMap struct {
	Up          tea.Key
	Down        tea.Key
	NextGroup   tea.Key
	Toggle      tea.Key
	SelectAll   tea.Key
	DeselectN   tea.Key
	SelectLow   tea.Key
	MassSelect  tea.Key
	Filter      tea.Key
	Sort        tea.Key
	RiskCycle   tea.Key
	ClearFilter tea.Key
	Confirm     tea.Key
	Quit        tea.Key
}

var keys = keyMap{
	Up:          tea.Key{Type: tea.KeyRunes, Runes: []rune{'k'}},
	Down:        tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}},
	NextGroup:   tea.Key{Type: tea.KeyTab},
	Toggle:      tea.Key{Type: tea.KeySpace},
	SelectAll:   tea.Key{Type: tea.KeyRunes, Runes: []rune{'a'}},
	DeselectN:   tea.Key{Type: tea.KeyRunes, Runes: []rune{'n'}},
	SelectLow:   tea.Key{Type: tea.KeyRunes, Runes: []rune{'l'}},
	MassSelect:  tea.Key{Type: tea.KeyRunes, Runes: []rune{'m'}},
	Filter:      tea.Key{Type: tea.KeyRunes, Runes: []rune{'/'}},
	Sort:        tea.Key{Type: tea.KeyRunes, Runes: []rune{'s'}},
	RiskCycle:   tea.Key{Type: tea.KeyRunes, Runes: []rune{'r'}},
	ClearFilter: tea.Key{Type: tea.KeyEsc},
	Confirm:     tea.Key{Type: tea.KeyEnter},
	Quit:        tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}},
}
