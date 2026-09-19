package tui

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestRenderFocusRingSegmentedArc(t *testing.T) {
	lines := renderFocusRing([]focusSlice{
		{Title: "CLI", Secs: 300, Color: colorPeach},
		{Title: "Read", Secs: 600, Color: colorTeal},
	}, nil, time.Now(), 30)
	if len(lines) != 6 {
		t.Fatalf("rows=%d want 6", len(lines))
	}
	plain := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(plain, "╭") || !strings.Contains(plain, "╯") || !strings.Contains(plain, "15m") {
		t.Fatalf("segmented arc missing expected shape/label:\n%s", plain)
	}
	for _, ln := range lines {
		if lipgloss.Width(ln) > 30 {
			t.Fatalf("line width=%d > 30: %q", lipgloss.Width(ln), stripANSI(ln))
		}
		for _, r := range stripANSI(ln) {
			if r >= 0x2801 && r <= 0x28ff {
				t.Fatalf("legacy braille rune %U found", r)
			}
		}
	}
}

func TestRenderFocusRingCompact(t *testing.T) {
	lines := renderFocusRing(nil, nil, time.Now(), 18)
	if len(lines) != 6 {
		t.Fatalf("rows=%d want compact arc plus sublabel", len(lines))
	}
	for _, line := range lines {
		if lipgloss.Width(line) > 18 {
			t.Fatalf("line width=%d > 18: %q", lipgloss.Width(line), stripANSI(line))
		}
	}
}

func TestFocusArcLiveProgressAndPauseColors(t *testing.T) {
	now := time.Now()
	session := &focus.Session{
		State:     focus.StateRunning,
		StartedAt: now.Add(-50 * time.Minute),
		Duration:  100 * time.Minute,
	}
	if got := focusArcColor(0, nil, session); got != colorPeach {
		t.Fatalf("elapsed segment color=%q", got)
	}
	if got := focusArcColor(1.5*math.Pi, nil, session); got != colorOverlay {
		t.Fatalf("remaining segment color=%q", got)
	}
	session.State = focus.StatePaused
	session.PausedAt = &now
	if got := focusArcColor(0, nil, session); got != colorBlue {
		t.Fatalf("paused elapsed segment color=%q", got)
	}
}

func TestColorAtAngle(t *testing.T) {
	slices := []focusSlice{
		{Secs: 50, Color: colorPeach},
		{Secs: 50, Color: colorTeal},
	}
	if colorAtAngle(0, slices, 100) != colorPeach {
		t.Fatal("top should be first slice")
	}
}
