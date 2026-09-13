package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func TestAlignPomoRowColumnsAlignedAndNearOnWidePanel(t *testing.T) {
	short := "▌12:47 Food"
	long := "▌11:30 Get the Inpost Package"
	suffix := formatPomoSuffixColumns("11:55", 25, false, "Get the Inpost Package", colorPeach)
	layout := pomoTimelineLayout{suffixStartCol: lipgloss.Width(long) + pomoColGap}

	shortLine := alignPomoRowColumns(short, suffix, 120, layout)
	longLine := alignPomoRowColumns(long, suffix, 120, layout)

	shortIdx := strings.Index(shortLine, "▶")
	longIdx := strings.Index(longLine, "▶")
	if shortIdx < 0 || longIdx < 0 {
		t.Fatalf("missing suffix: short=%q long=%q", shortLine, longLine)
	}
	if shortIdx != longIdx {
		t.Fatalf("suffix columns misaligned: short@%d long@%d", shortIdx, longIdx)
	}
	if shortIdx > 50 {
		t.Fatalf("suffix too far right: col=%d line=%q", shortIdx, shortLine)
	}
}

func TestAlignPomoRowColumnsFitsNarrowPanel(t *testing.T) {
	title := "▌11:30 Get the Inpost Package"
	suffix := formatPomoSuffixColumns("11:55", 25, false, "Task", colorPeach)
	layout := pomoTimelineLayout{}
	got := alignPomoRowColumns(title, suffix, 40, layout)
	if lipgloss.Width(got) > 40 {
		t.Fatalf("row wider than panel: %q", got)
	}
	if !strings.Contains(got, "11:55") {
		t.Fatalf("missing end time in %q", got)
	}
}

func TestFullPomoBarFillsFor25Minutes(t *testing.T) {
	got := pomoDurationBarStyled(25, pomoSessionFull, colorPeach, false)
	if strings.Count(got, "▮") != 8 {
		t.Fatalf("expected full bar for 25m, got %q", got)
	}
}

func TestUnclaimedBarScaledToStandardPomo(t *testing.T) {
	got := pomoDurationBarStyled(5, pomoSessionUnclaimed, colorRed, false)
	if strings.Count(got, "▪") < 2 {
		t.Fatalf("expected visible unclaimed bar for 5m, got %q", got)
	}
}

func testNow() time.Time {
	return time.Date(2026, 8, 30, 15, 47, 0, 0, time.Local)
}
