package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestFormatTaskLineKeepsShortTitleWithFocusColumn(t *testing.T) {
	m := fixtureModel(120, 40)
	m.taskFocusByTitle = map[string]ticktick.TaskFocusSummary{
		"garbage": {FullSessions: 4, LoggedSessions: 4, TotalSeconds: 6000},
	}
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "One Time Setup"}},
		{Task: ticktick.Task{Title: "Garbage"}},
	}
	focusColW := maxTaskFocusInlineW(m, rows)
	layout := computeTaskRowLayout(rows, w, focusColW)
	line := m.formatTaskLine(rows[0].Task, 0, false, false, w, layout)
	plain := stripANSI(line)
	if strings.Contains(plain, "One Time S…") {
		t.Fatalf("short title over-truncated: %q", plain)
	}
	if !strings.Contains(plain, "One Time Setup") {
		t.Fatalf("expected full title, got %q", plain)
	}
	if lipgloss.Width(line) > w {
		t.Fatalf("line too wide: %d > %d", lipgloss.Width(line), w)
	}
}

func TestDueInlineConsistentWidth(t *testing.T) {
	future := dueInlineTask(ticktick.Task{DueDate: "2026-09-02T04:30:00.000+0200"})
	past := dueInlineTask(ticktick.Task{DueDate: "2020-01-01T04:30:00.000+0200"})
	if lipgloss.Width(future) != dueColWidth || lipgloss.Width(past) != dueColWidth {
		t.Fatalf("widths differ future=%d past=%d want %d", lipgloss.Width(future), lipgloss.Width(past), dueColWidth)
	}
	if !strings.Contains(stripANSI(future), "●") {
		t.Fatalf("future due should include marker: %q", stripANSI(future))
	}
}

func TestFormatTaskLineAlignsOverdueAndFuture(t *testing.T) {
	m := fixtureModel(120, 40)
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "Overdue", DueDate: "2020-01-01T08:00:00.000+0200"}},
		{Task: ticktick.Task{Title: "Future", DueDate: "2026-09-02T04:30:00.000+0200"}},
	}
	layout := computeTaskRowLayout(rows, w, 0)
	a := dueColStart(m.formatTaskLine(rows[0].Task, 0, false, false, w, layout))
	b := dueColStart(m.formatTaskLine(rows[1].Task, 0, false, false, w, layout))
	if a < 0 || b < 0 || a != b {
		t.Fatalf("due markers misaligned: %d vs %d", a, b)
	}
}

func TestMaxVisibleRowsIsForty(t *testing.T) {
	if maxVisibleRows != 40 {
		t.Fatalf("maxVisibleRows=%d want 40", maxVisibleRows)
	}
}
