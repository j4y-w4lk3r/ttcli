package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestCalendarMonthIsMondayFirst(t *testing.T) {
	lay := computeCalMonthLayout(140, 40, 4)
	fields := strings.Fields(stripANSI(renderCalMonthDOWHeader(lay)))
	want := []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	if len(fields) < len(want) {
		t.Fatalf("headers=%v", fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("headers=%v want=%v", fields[:7], want)
		}
	}
	first := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local)
	if offset := (int(first.Weekday()) + 6) % 7; offset != 1 {
		t.Fatalf("September 2026 offset=%d want Tuesday in column 2", offset)
	}
}

func TestCalendarMonthCellShowsPomoGoalProgress(t *testing.T) {
	day := time.Date(2026, time.September, 18, 0, 0, 0, 0, time.Local)
	lines := renderCalMonthCell(
		buildCalIndex(nil), day, 28, 6, day, day,
		&ticktick.FocusStats{FullPomoCount: 3, TotalSeconds: 75 * 60},
		30,
	)
	output := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(output, "3/30") || !strings.Contains(output, "75m") {
		t.Fatalf("month focus cell:\n%s", output)
	}
	if len(lines) != 8 {
		t.Fatalf("cell lines=%d want 8", len(lines))
	}
}

func TestCalendarYearDayMarkerCarriesPomoCount(t *testing.T) {
	day := time.Date(2026, time.September, 18, 0, 0, 0, 0, time.Local)
	line := stripANSI(calYearDayCell(
		buildCalIndex(nil), day, day.AddDate(0, 0, 1), day.AddDate(0, 0, 2), 5,
		map[string]*ticktick.FocusStats{dateKey(day): {FullPomoCount: 3}},
	))
	if !strings.Contains(line, "3") {
		t.Fatalf("year focus marker=%q", line)
	}
}
