package tui

import "github.com/charmbracelet/bubbletea"

type keyMap struct {
	Up        tea.Key
	Down      tea.Key
	NextGroup tea.Key
	Toggle    tea.Key
	SelectAll tea.Key
	DeselectN tea.Key
	SelectLow tea.Key
	Confirm   tea.Key
	Quit      tea.Key
}

var keys = keyMap{
	Up:        tea.Key{Type: tea.KeyRunes, Runes: []rune{'k'}},
	Down:      tea.Key{Type: tea.KeyRunes, Runes: []rune{'j'}},
	NextGroup: tea.Key{Type: tea.KeyTab},
	Toggle:    tea.Key{Type: tea.KeySpace},
	SelectAll: tea.Key{Type: tea.KeyRunes, Runes: []rune{'a'}},
	DeselectN: tea.Key{Type: tea.KeyRunes, Runes: []rune{'n'}},
	SelectLow: tea.Key{Type: tea.KeyRunes, Runes: []rune{'l'}},
	Confirm:   tea.Key{Type: tea.KeyEnter},
	Quit:      tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}},
}
