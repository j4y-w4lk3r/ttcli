package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestCalendarYearResponsiveMiniCalendars(t *testing.T) {
	year := time.Now().Year()
	selected := time.Date(year, time.September, 18, 0, 0, 0, 0, time.Local)
	tasks := []ticktick.Task{
		{ID: "one", Title: "One", DueDate: dateKey(selected), IsAllDay: true},
		{ID: "two", Title: "Two", DueDate: dateKey(selected.AddDate(0, 1, 0)), IsAllDay: true},
	}
	for _, size := range []struct{ width, height int }{{80, 50}, {120, 40}, {200, 40}} {
		m := fixtureModel(size.width, size.height)
		m.view, m.calMode, m.calDate = viewCalendar, calModeYear, selected
		m.calTasks = tasks
		assertViewOK(t, m, fmt.Sprintf("year %dx%d", size.width, size.height))
		view := stripANSI(m.View())
		for _, want := range []string{"September", "active days", "Mo", "Tu"} {
			if !strings.Contains(view, want) {
				t.Fatalf("%dx%d year view missing %q", size.width, size.height, want)
			}
		}
	}
}

func TestCalendarYearCompactFallbackAndGridNavigation(t *testing.T) {
	selected := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	m := fixtureModel(50, 24)
	m.view, m.calMode, m.calDate = viewCalendar, calModeYear, selected
	assertViewOK(t, m, "compact year")
	view := stripANSI(m.View())
	if !strings.Contains(view, "Jan") || !strings.Contains(view, "Dec") {
		t.Fatalf("compact year missing months:\n%s", view)
	}
	m = m.calMoveVert(1)
	if m.calDate.Month() != time.March {
		t.Fatalf("vertical move month=%s want March", m.calDate.Month())
	}
	m = m.calMoveHoriz(1)
	if m.calDate.Month() != time.April {
		t.Fatalf("horizontal move month=%s want April", m.calDate.Month())
	}
}

func TestCalendarYearUsesExtraHeightForThreeCardRows(t *testing.T) {
	if got := calYearGridColumnsForSize(246, 50); got != 4 {
		t.Fatalf("tall wide year columns=%d want 4", got)
	}
	if got := calYearGridColumnsForSize(246, 24); got != 6 {
		t.Fatalf("short wide year columns=%d want 6", got)
	}
}
