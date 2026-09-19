package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestPomoFocusDesignCycle(t *testing.T) {
	if PomoFocusArc.Next() != PomoFocusBar ||
		PomoFocusBar.Next() != PomoFocusCard ||
		PomoFocusCard.Next() != PomoFocusArc {
		t.Fatal("unexpected focus design cycle")
	}
}

func TestPomoFocusDesignsFitResponsiveWidths(t *testing.T) {
	for _, design := range []PomoFocusDesign{PomoFocusArc, PomoFocusBar, PomoFocusCard} {
		for _, width := range []int{18, 40} {
			m := model{uiSettings: uiSettings{PomoFocusDesign: design}}
			lines := m.renderPomoFocusVisual(
				[]focusSlice{{Title: "Work", Secs: 25 * 60, Color: colorPeach}},
				nil,
				time.Now().AddDate(0, 0, -1),
				width,
			)
			if len(lines) == 0 {
				t.Fatalf("design=%s width=%d rendered no lines", design, width)
			}
			for _, line := range lines {
				if got := lipgloss.Width(line); got > width {
					t.Fatalf("design=%s width=%d line width=%d: %q", design, width, got, stripANSI(line))
				}
			}
		}
	}
}

func TestPomoDesignKeyPersistsSelection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := fixtureModel(80, 30)
	m.view = viewPomodoro
	m.uiSettings.PomoFocusDesign = PomoFocusArc
	out, cmd := m.updatePomoKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}})
	if cmd != nil {
		t.Fatal("design switch should not start a command")
	}
	if out.uiSettings.PomoFocusDesign != PomoFocusBar {
		t.Fatalf("design=%q", out.uiSettings.PomoFocusDesign)
	}
	if !strings.Contains(out.toast, "focus bar") {
		t.Fatalf("toast=%q", out.toast)
	}
}

func TestPomoGoalKeysPersistAndStatusCardUpdates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := fixtureModel(80, 30)
	m.view = viewPomodoro
	m.uiSettings.PomoFocusDesign = PomoFocusCard
	m.uiSettings.PomoDailyGoal = 30
	m.focusStats = &ticktick.FocusStats{FullPomoCount: 5}

	out, cmd := m.updatePomoKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'+'}})
	if cmd != nil {
		t.Fatal("goal adjustment should not start a command")
	}
	if out.uiSettings.PomoDailyGoal != 31 || !strings.Contains(out.toast, "31") {
		t.Fatalf("goal=%d toast=%q", out.uiSettings.PomoDailyGoal, out.toast)
	}
	lines := out.renderPomoFocusVisual(nil, nil, time.Now(), 40)
	if !strings.Contains(stripANSI(strings.Join(lines, "\n")), "5 / 31 pomodoros") {
		t.Fatalf("status card did not use dynamic goal:\n%s", stripANSI(strings.Join(lines, "\n")))
	}
	if loaded := loadUISettings(); loaded.PomoDailyGoal != 31 {
		t.Fatalf("persisted goal=%d want 31", loaded.PomoDailyGoal)
	}
	out, _ = out.updatePomoKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'-'}})
	if out.uiSettings.PomoDailyGoal != 30 {
		t.Fatalf("decremented goal=%d want 30", out.uiSettings.PomoDailyGoal)
	}
}

func TestPomoBadgeUsesConfiguredGoal(t *testing.T) {
	m := model{
		todayPomos: 5,
		uiSettings: uiSettings{PomoDailyGoal: 28},
	}
	if badge := stripANSI(m.renderPomoBadge()); !strings.Contains(badge, "5/28") {
		t.Fatalf("badge=%q", badge)
	}
}
