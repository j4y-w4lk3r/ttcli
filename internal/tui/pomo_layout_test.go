package tui

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestComputePomoTimelineLayoutFromLongestTitle(t *testing.T) {
	now := time.Date(2026, 9, 1, 16, 0, 0, 0, time.Local)
	recs := []ticktick.FocusRecord{
		{StartTime: "2026-09-01T11:30:00+02:00", EndTime: "2026-09-01T11:55:00+02:00", Tasks: taskLink("Get the Inpost Package")},
		{StartTime: "2026-09-01T12:47:00+02:00", EndTime: "2026-09-01T13:12:00+02:00", Tasks: taskLink("Food")},
	}
	grid := buildDayGrid(recs, nil, now, nil, true)
	colors := map[string]lipgloss.Color{
		"Get the Inpost Package": colorPeach,
		"Food":                   colorTeal,
	}
	layout := computePomoTimelineLayout(grid, 120, colors, nil, now)
	if layout.suffixStartCol == 0 {
		t.Fatal("expected suffix column")
	}
	if layout.suffixStartCol > 50 {
		t.Fatalf("suffix column too far: %d", layout.suffixStartCol)
	}
}
