package tui

import (
	"strings"
	"testing"
	"time"
)

func TestSelectedFocusLegendUsesSingleIndicator(t *testing.T) {
	line := stripANSI(renderFocusLegendRow(
		focusSlice{Title: "send package back", Secs: 28 * 60, Color: colorBlue},
		48, true, 28*60,
	))
	if strings.Contains(line, "▸") || strings.Count(line, "●") != 1 {
		t.Fatalf("selected legend has duplicate indicators: %q", line)
	}
	if !strings.Contains(line, "● send package back") {
		t.Fatalf("selected legend indicator is misplaced: %q", line)
	}
}

func TestSelectedPomoTimelineStartsWithSelectionIndicator(t *testing.T) {
	line := stripANSI(renderHourPomoRow(
		0, 1, true, "send package back", colorBlue,
		"10:08", "10:37", 29, 100, pomoTimelineLayout{suffixStartCol: 45},
	))
	if !strings.HasPrefix(line, "●") {
		t.Fatalf("selected timeline row has blank gutter before indicator: %q", line)
	}
}

func TestSelectedPomoRowsDoNotPaintGreyBackground(t *testing.T) {
	withTrueColor(t)
	line := renderHourPomoRow(
		0, 1, true, "send package back", colorBlue,
		"10:08", "10:37", 29, 100, pomoTimelineLayout{suffixStartCol: 45},
	)
	for background, count := range visibleBackgrounds(line) {
		if count > 0 && background == ansiBGOverlay {
			t.Fatalf("selected row painted grey background %s on %d cells", background, count)
		}
	}
}

func TestSelectedFocusLegendDoesNotPaintGreyBackground(t *testing.T) {
	withTrueColor(t)
	line := renderFocusLegendRow(
		focusSlice{Title: "send package back", Secs: 28 * 60, Color: colorBlue},
		48, true, 28*60,
	)
	if count := visibleBackgrounds(line)[ansiBGOverlay]; count > 0 {
		t.Fatalf("selected legend painted grey background on %d cells", count)
	}
}

func TestStatusCardDoesNotRepeatFocusedDuration(t *testing.T) {
	lines := renderFocusCard(
		[]focusSlice{{Title: "work", Secs: 28 * 60, Color: colorBlue}},
		nil,
		time.Now(),
		48,
		nil,
		1,
		30,
	)
	output := stripANSI(strings.Join(lines, "\n"))
	if count := strings.Count(output, "28m"); count != 1 {
		t.Fatalf("focused duration repeated %d times:\n%s", count, output)
	}
	if !strings.Contains(output, "29 pomodoros remaining") {
		t.Fatalf("status card missing non-duplicated progress context:\n%s", output)
	}
}
