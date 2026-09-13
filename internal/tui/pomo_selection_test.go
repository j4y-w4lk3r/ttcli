package tui

import "testing"

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
		{kind: "slot"},
	}
	if got := nextPomoGridCursor(grid, 0, 1); got != 1 {
		t.Fatalf("logged day down=%d want 1 (pomo)", got)
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
