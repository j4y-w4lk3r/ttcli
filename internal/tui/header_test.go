package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestHeaderBarFullWidthBackground(t *testing.T) {
	for _, w := range []int{80, 120, 200} {
		m := fixtureModel(w, 40)
		header := m.renderHeader()
		for i, line := range strings.Split(header, "\n") {
			if lipgloss.Width(line) != w {
				t.Fatalf("row %d width %d want %d", i, lipgloss.Width(line), w)
			}
		}
	}
}

func TestTitleRowKeepsPomoBadge(t *testing.T) {
	for _, w := range []int{80, 91, 120, 160, 200} {
		m := fixtureModel(w, 40)
		header := m.renderHeader()
		line0 := strings.Split(header, "\n")[0]
		plain := stripANSI(line0)
		t.Logf("%dx: width=%d plain=%q tail=%q", w, lipgloss.Width(line0), previewLine(plain, 50), plain[max(0, len(plain)-15):])
		if !strings.Contains(plain, "/") {
			t.Fatalf("%d: title row missing pomo slash: %q", w, plain)
		}
		if lipgloss.Width(line0) != w {
			t.Fatalf("%d: title row width %d want %d", w, lipgloss.Width(line0), w)
		}
		idxBrand := strings.Index(plain, "TickTick")
		idxPomo := strings.Index(plain, "/")
		if idxBrand < 0 || idxPomo < 0 || idxPomo-idxBrand > 20 {
			t.Fatalf("%d: pomo badge too far from brand: %q", w, plain)
		}
	}
}
