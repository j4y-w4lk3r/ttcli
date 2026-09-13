package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// DebugFrame renders a fixture-backed view at an explicit terminal size and
// returns the frame text plus a human-readable layout report.
func DebugFrame(termW, termH int, view string) (frame, report string, ok bool) {
	m := fixtureModel(termW, termH)
	switch view {
	case "calendar":
		m = switchToView(m, viewCalendar)
	case "pomo", "pomodoro":
		m = switchToView(m, viewPomodoro)
	case "habits":
		m = switchToView(m, viewHabits)
	default:
		view = "tasks"
	}
	frame = m.View()
	r := analyzeView(frame, termW, termH, view)
	return frame, r.String(), r.OK()
}

// DebugFrameLines is like DebugFrame but also returns per-line width previews.
func DebugFrameLines(termW, termH int, view string) (lines []string, report string, ok bool) {
	frame, report, ok := DebugFrame(termW, termH, view)
	r := analyzeView(frame, termW, termH, view)
	lines = r.Lines
	return lines, report, ok
}

// FormatFrameDump prints line index, width, and a plain preview (for logs/CI).
func FormatFrameDump(lines []string, previewCols int) string {
	var b strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&b, "%3d|%3d| %s\n", i, lipgloss.Width(line), previewLine(line, previewCols))
	}
	return b.String()
}
