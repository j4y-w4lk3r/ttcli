package tui

import (
	"strings"
	"testing"
)

func TestComposeViewExactDimensions(t *testing.T) {
	header := "title\nnav"
	body := strings.Repeat("body\n", 5)
	footer := "footer"
	out := composeView(header, body, footer, 80, 10, 7)
	lines := strings.Split(out, "\n")
	if len(lines) != 10 {
		t.Fatalf("got %d lines want 10", len(lines))
	}
	for i, line := range lines {
		if strings.Contains(line, "\n") {
			t.Fatalf("line %d contains embedded newline", i)
		}
	}
}

func TestCalendarToTasksHeaderIntact(t *testing.T) {
	// Wide terminal like the user's setup.
	for _, w := range []int{120, 200, 240} {
		m := fixtureModel(w, 50)
		m = switchToView(m, viewCalendar)
		assertViewOK(t, m, "calendar")
		m = switchToView(m, viewTasks)
		assertViewOK(t, m, "tasks after calendar")

		view := m.View()
		lines := strings.Split(view, "\n")
		if len(lines) != 50 {
			t.Fatalf("%dw: got %d lines want 50", w, len(lines))
		}
		plain0 := stripANSI(lines[0])
		plain1 := stripANSI(lines[1])
		if strings.Contains(lines[0], "╭") || strings.Contains(lines[0], "│") {
			t.Fatalf("%dw: body border on header line 0: %q", w, previewLine(plain0, 80))
		}
		if !strings.Contains(plain0, "TickTick") {
			t.Fatalf("%dw: line 0 missing brand: %q", w, plain0)
		}
		if !strings.Contains(plain1, "Tasks") {
			t.Fatalf("%dw: line 1 missing nav: %q", w, plain1)
		}
		if strings.Contains(lines[1], "╭") {
			t.Fatalf("%dw: body border on nav row", w)
		}
		if !strings.HasPrefix(lines[2], "╭") {
			t.Fatalf("%dw: body should start line 2, got %q", w, previewLine(stripANSI(lines[2]), 40))
		}
	}
}
