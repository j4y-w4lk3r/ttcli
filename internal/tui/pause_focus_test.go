package tui

import (
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
