package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestCalDayKeyJumpsToToday(t *testing.T) {
	m := fixtureModel(100, 30)
	m.view = viewCalendar
	m.calMode = calModeMonth
	m.calDate = time.Date(2026, 1, 15, 0, 0, 0, 0, time.Local)

	out, _ := m.updateCalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = out
	if m.calMode != calModeDay {
		t.Fatalf("mode=%v", m.calMode)
	}
	if dateKey(m.calDate) != dateKey(time.Now()) {
		t.Fatalf("calDate=%v want today", m.calDate)
	}
}

func TestCalDayTimelinePlacesDateOnlyOnRail(t *testing.T) {
	day := time.Date(2026, 8, 30, 0, 0, 0, 0, time.Local)
	idx := buildCalIndex([]ticktick.Task{
		{Title: "anytime", DueDate: "2026-08-30", IsAllDay: true},
		{Title: "timed", DueDate: "2026-08-30T14:00:00.000+0200"},
	})
	m := model{calDate: day}
	rows, _ := m.calDayRows(idx)
	var allDayHour string
	for i, r := range rows {
		if r.kind == "allday" {
			for j := i - 1; j >= 0; j-- {
				if rows[j].kind == "slot" {
					allDayHour = rows[j].hourLabel
					break
				}
			}
			break
		}
	}
	if allDayHour != "08:00" {
		t.Fatalf("date-only task rail=%q want 08:00", allDayHour)
	}
}

func TestFocusAlertEscalatedOverlay(t *testing.T) {
	m := fixtureModel(100, 30)
	m.showFocusAlert = true
	m.focusNotifyEscalated = true
	m.focusAlertTitle = "Big task"
	out := stripANSI(m.renderFocusAlertOverlay())
	if !strings.Contains(out, "Pomodoro complete") {
		t.Fatal("expected escalated headline")
	}
	if !strings.Contains(out, "TIME IS UP") {
		t.Fatal("expected escalated time label")
	}
	if !strings.Contains(out, "▒") && !strings.Contains(out, "░") {
		t.Fatal("expected dim backdrop")
	}
}

func TestPreviewFocusAlert(t *testing.T) {
	out := PreviewFocusAlert(80, 20, false, "Task")
	if !strings.Contains(stripANSI(out), "Pomodoro complete") {
		t.Fatal("preview should render alert")
	}
}

func TestFocusAlertOverlayBlocksView(t *testing.T) {
	m := fixtureModel(100, 30)
	m.showFocusAlert = true
	m.focusAlertTitle = "Write tests"
	out := m.View()
	if !strings.Contains(stripANSI(out), "Pomodoro complete") {
		t.Fatal("expected alert overlay")
	}
	if lipgloss.Width(strings.Split(out, "\n")[0]) != 100 {
		t.Fatal("alert should fill terminal width")
	}
}

func TestDismissFocusAlert(t *testing.T) {
	m := fixtureModel(80, 24)
	m.showFocusAlert = true
	m.focusAlertTitle = "Task"
	out, _ := m.dismissFocusAlert()
	m = out
	if m.showFocusAlert {
		t.Fatal("alert should close")
	}
	if !m.focusAlertDismissed {
		t.Fatal("should remember dismiss")
	}
}

func TestFocusAlertChordDismiss(t *testing.T) {
	m := fixtureModel(80, 24)
	m.showFocusAlert = true
	m, _ = m.updateFocusAlert(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if !m.focusAlertChord {
		t.Fatal("expected chord mode after n")
	}
	m, _ = m.updateFocusAlert(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.showFocusAlert {
		t.Fatal("n t should dismiss alert")
	}
}

func TestFocusAlertChordRepeat(t *testing.T) {
	m := fixtureModel(80, 24)
	m.showFocusAlert = true
	m, _ = m.updateFocusAlert(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m, cmd := m.updateFocusAlert(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if m.showFocusAlert {
		t.Fatal("n r should close alert")
	}
	if cmd == nil {
		t.Fatal("expected repeat command")
	}
}
