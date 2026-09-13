package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Hardcoded app fill colors we no longer paint (terminal default is used instead).
const (
	ansiBGBase    = "rgb(30,30,46)"  // #1e1e2e
	ansiBGSurface = "rgb(49,50,68)"  // #313244
	ansiBGOverlay = "rgb(69,71,90)"  // #45475a
	ansiBGMauve   = "rgb(203,166,247)" // selection highlight
)

func withTrueColor(t *testing.T) {
	t.Helper()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() { lipgloss.SetColorProfile(termenv.Ascii) })
}

func visibleBackgrounds(s string) map[string]int {
	counts := map[string]int{}
	cur := "default"
	esc := false
	var seq strings.Builder
	for _, r := range s {
		if esc {
			seq.WriteRune(r)
			if r == 'm' {
				if bg := extractBackground(seq.String()); bg != "" {
					cur = bg
				}
				seq.Reset()
				esc = false
			}
			continue
		}
		if r == '\x1b' {
			esc = true
			seq.WriteRune(r)
			continue
		}
		if r == '\n' || r == '\r' {
			continue
		}
		counts[cur]++
	}
	return counts
}

func extractBackground(code string) string {
	if code == "\x1b[49m" || code == "\x1b[0m" {
		return "reset"
	}
	if !strings.HasPrefix(code, "\x1b[") || !strings.HasSuffix(code, "m") {
		return ""
	}
	inner := code[2 : len(code)-1]
	parts := strings.Split(inner, ";")
	for i := 0; i < len(parts); i++ {
		if parts[i] != "48" {
			continue
		}
		if i+1 < len(parts) && parts[i+1] == "2" && i+4 < len(parts) {
			return fmt.Sprintf("rgb(%s,%s,%s)", parts[i+2], parts[i+3], parts[i+4])
		}
		if i+1 < len(parts) {
			return "idx:" + parts[i+1]
		}
	}
	return ""
}

func assertNoHardcodedFillBackgrounds(t *testing.T, line string, allowAccent map[string]bool, label string) {
	t.Helper()
	bgs := visibleBackgrounds(line)
	for bg, n := range bgs {
		if n == 0 {
			continue
		}
		if bg == "default" || bg == "reset" {
			continue
		}
		if allowAccent != nil && allowAccent[bg] {
			continue
		}
		if bg == ansiBGBase || bg == ansiBGSurface {
			t.Fatalf("%s: painted hardcoded fill %s on %d cells", label, bg, n)
		}
	}
}

func isHelpBoxLine(plain string) bool {
	return strings.Contains(plain, "│") || strings.Contains(plain, "╭") ||
		strings.Contains(plain, "╰") || strings.Contains(plain, "╮") ||
		strings.Contains(plain, "╯")
}
