package tui

import (
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestNextSelectablePomoGridRowSkipsGaps(t *testing.T) {
	grid := []dayGridRow{
		{kind: "slot"},
		{kind: "gap"},
		{kind: "pomo", recIdx: 0},
		{kind: "gap"},
		{kind: "pause"},
		{kind: "slot"},
	}
	if got := nextSelectablePomoGridRow(grid, 0, 1); got != 2 {
		t.Fatalf("from slot down=%d want 2", got)
	}
	if got := nextSelectablePomoGridRow(grid, 2, 1); got != 4 {
		t.Fatalf("from pomo down=%d want 4", got)
	}
	if got := nextSelectablePomoGridRow(grid, 4, -1); got != 2 {
		t.Fatalf("from pause up=%d want 2", got)
	}
}

func TestNextPomoGridCursorLinearWhenEmpty(t *testing.T) {
	grid := []dayGridRow{
		{kind: "slot"},
		{kind: "gap"},
		{kind: "now"},
		{kind: "slot"},
	}
	if got := nextPomoGridCursor(grid, 0, 1); got != 1 {
		t.Fatalf("empty day down=%d want 1", got)
	}
	if got := nextPomoGridCursor(grid, 2, -1); got != 1 {
		t.Fatalf("empty day up from now=%d want 1", got)
	}
}

func TestNextPomoGridCursorSessionWhenLogged(t *testing.T) {
	grid := []dayGridRow{
		{kind: "slot"},
		{kind: "pomo", recIdx: 0},
		{kind: "gap"},
		{kind: "pomo", recIdx: 1},
		{kind: "slot"},
	}
	if got := nextPomoGridCursor(grid, 0, 1); got != 1 {
		t.Fatalf("logged day down=%d want 1 (pomo)", got)
	}
	if got := nextPomoGridCursor(grid, 1, 1); got != 2 {
		t.Fatalf("line scroll skipped gap: got=%d want 2", got)
	}
	if got := nextSelectablePomoGridRow(grid, 1, 1); got != 3 {
		t.Fatalf("session jump=%d want 3", got)
	}
}

func TestNearestSelectablePrefersDirection(t *testing.T) {
	grid := []dayGridRow{
		{kind: "slot"},
		{kind: "pomo", recIdx: 0},
		{kind: "now"},
		{kind: "pomo", recIdx: 1},
	}
	if got := nearestSelectablePomoGridRow(grid, 2, -1); got != 1 {
		t.Fatalf("nearest up=%d want 1", got)
	}
	if got := nearestSelectablePomoGridRow(grid, 2, 1); got != 3 {
		t.Fatalf("nearest down=%d want 3", got)
	}
}

func TestNonSessionLineDoesNotTargetStalePomodoro(t *testing.T) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.Local)
	end := start.Add(25 * time.Minute)
	m := fixtureModel(100, 30)
	m.view = viewPomodoro
	m.pomoViewDate = dateOnly(now)
	m.pomoNowTick = now
	m.uiSettings.PomoTimelineDensity = PomoDensityStretch
	m.focusStats = &ticktick.FocusStats{Records: []ticktick.FocusRecord{{
		ID:        "focus-1",
		StartTime: start.Format("2006-01-02T15:04:05.000-0700"),
		EndTime:   end.Format("2006-01-02T15:04:05.000-0700"),
	}}}
	grid := m.pomoDayGrid()
	for i, row := range grid {
		if row.kind == "gap" {
			m.pomoGridCursor = i
			break
		}
	}
	m.pomoCursor = 0
	if _, ok := m.selectedPomoRecord(); ok {
		t.Fatal("gap row must not target the previously selected pomodoro")
	}
}
