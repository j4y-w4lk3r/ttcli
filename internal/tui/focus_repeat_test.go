package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGlobalRFallsBackToRefreshWithoutSession(t *testing.T) {
	m := fixtureModel(100, 30)
	m.view = viewTasks
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if out.loading != true && cmd == nil {
		t.Fatal("expected refresh when no active session")
	}
}

func TestFocusAlertRRepeat(t *testing.T) {
	m := fixtureModel(80, 24)
	m.showFocusAlert = true
	m.focusAlertTitle = "Desk Setup"
	out, cmd := m.updateFocusAlert(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
	if out.showFocusAlert {
		t.Fatal("R should close alert")
	}
	if cmd == nil {
		t.Fatal("expected repeat command")
	}
}
