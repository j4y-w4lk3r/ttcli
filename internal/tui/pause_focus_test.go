package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestFormatPauseWorkBreakdown(t *testing.T) {
	spell := focus.PauseSpell{
		Start:      time.Date(2026, 9, 11, 10, 52, 0, 0, time.UTC),
		End:        time.Date(2026, 9, 11, 11, 48, 0, 0, time.UTC),
		WorkBefore: 8 * time.Minute,
		WorkAfter:  17 * time.Minute,
	}
	got := formatPauseWorkBreakdown(spell, time.Now())
	want := "8m work · 56m paused · 17m work"
	if got != want {
		t.Fatalf("breakdown=%q want %q", got, want)
	}
}

func TestPauseTimelineRowShowsLengthAndContext(t *testing.T) {
	short := focus.PauseSpell{
		TaskTitle:  "rui",
		Start:      time.Date(2026, 10, 10, 9, 54, 12, 0, time.Local),
		End:        time.Date(2026, 10, 10, 9, 54, 14, 0, time.Local),
		WorkBefore: 21 * time.Minute,
		WorkAfter:  20 * time.Minute,
	}
	line := stripANSI(renderHourPauseRow(short, false, 140, pomoTimelineLayout{suffixStartCol: 48}, short.End))
	for _, want := range []string{"09:54:12", "2s", "21m before", "20m after", "rui"} {
		if !strings.Contains(line, want) {
			t.Fatalf("short pause %q missing %q", line, want)
		}
	}
	if strings.Contains(line, "──▶") || strings.Contains(line, "▫") {
		t.Fatalf("short pause still uses the session bar: %q", line)
	}

	longStart := time.Date(2026, 10, 10, 10, 52, 0, 0, time.Local)
	long := focus.PauseSpell{
		TaskTitle:  "rui",
		Start:      longStart,
		End:        longStart.Add(56 * time.Minute),
		WorkBefore: 8 * time.Minute,
		WorkAfter:  17 * time.Minute,
	}
	longLine := stripANSI(renderHourPauseRow(long, false, 140, pomoTimelineLayout{suffixStartCol: 48}, long.End))
	for _, want := range []string{"10:52–11:48", "56m", "8m before", "17m after"} {
		if !strings.Contains(longLine, want) {
			t.Fatalf("long pause %q missing %q", longLine, want)
		}
	}
}
