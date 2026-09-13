package tui

import (
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestParseClockOnDay(t *testing.T) {
	day := time.Date(2026, 9, 11, 0, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	got, err := parseClockOnDay(day, "10:19")
	if err != nil {
		t.Fatal(err)
	}
	if got.Hour() != 10 || got.Minute() != 19 {
		t.Fatalf("got %v", got)
	}
}

func TestParseAddPomoStartEndedNow(t *testing.T) {
	now := time.Now()
	m := model{
		addPomoLogDate:      dateOnly(now),
		addPomoStartUnset:   true,
		focusPickerMinutes:  25,
	}
	start, err := m.parseAddPomoStart(25*time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	diff := now.Sub(start)
	if diff < 24*time.Minute || diff > 26*time.Minute {
		t.Fatalf("start=%v diff=%v want ~25m", start, diff)
	}
}

func TestParseAddPomoStartClock(t *testing.T) {
	day := time.Date(2026, 9, 11, 0, 0, 0, 0, time.Local)
	m := model{
		addPomoLogDate:      day,
		addPomoStartMinutes: 10*60 + 19,
	}
	start, err := m.parseAddPomoStart(25*time.Minute, 0)
	if err != nil {
		t.Fatal(err)
	}
	local := start.In(time.Local)
	if local.Hour() != 10 || local.Minute() != 19 {
		t.Fatalf("got %v", local)
	}
}

func TestShiftAddPomoLogDateClampsFuture(t *testing.T) {
	today := dateOnly(time.Now())
	m := model{addPomoLogDate: today}
	m = m.shiftAddPomoLogDate(1)
	if !m.addPomoLogDate.Equal(today) {
		t.Fatalf("future clamp: got %v want %v", m.addPomoLogDate, today)
	}
}

func TestBuildLegendSlicesIncludesPause(t *testing.T) {
	pauses := []focus.PauseSpell{
		{
			TaskTitle: "desk + monitor",
			Start:     time.Date(2026, 9, 11, 8, 52, 0, 0, time.UTC),
			End:       time.Date(2026, 9, 11, 9, 48, 0, 0, time.UTC),
		},
	}
	slices := buildLegendSlices(nil, pauses, time.Now())
	if len(slices) != 1 {
		t.Fatalf("slices=%d want 1 pause row", len(slices))
	}
	if slices[0].Kind != pomoSessionPause {
		t.Fatalf("kind=%v want pause", slices[0].Kind)
	}
}
