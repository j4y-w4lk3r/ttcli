package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func dueColStart(line string) int {
	plain := stripANSI(line)
	if idx := strings.Index(plain, "●"); idx >= 0 {
		return idx
	}
	return -1
}

func TestComputeTaskRowLayoutCapsLongTitles(t *testing.T) {
	w := 120
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "Morning"}},
		{Task: ticktick.Task{Title: "Create an app for the iot, same as PiKVM"}},
	}
	layout := computeTaskRowLayout(rows, w, 0)
	if layout.SuffixCol > 60 {
		t.Fatalf("suffix col too far right: %d", layout.SuffixCol)
	}
	if layout.TitleColW > maxTaskTitleColW {
		t.Fatalf("title col wider than cap: %d", layout.TitleColW)
	}
}

func TestFormatTaskLineAlignsDueDates(t *testing.T) {
	m := fixtureModel(120, 40)
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "Morning", DueDate: "2026-02-22T08:00:00.000+0200"}},
		{Task: ticktick.Task{Title: "Bed", DueDate: "2026-03-08T20:00:00.000+0200"}},
		{Task: ticktick.Task{Title: "A much longer task name that should truncate", DueDate: "2026-05-23T09:30:00.000+0200"}},
	}
	layout := computeTaskRowLayout(rows, w, 0)
	var starts []int
	for i, row := range rows {
		line := m.formatTaskLine(row.Task, 0, i == 0, false, w, layout)
		if lipgloss.Width(line) > w {
			t.Fatalf("line %d too wide: %d", i, lipgloss.Width(line))
		}
		starts = append(starts, dueColStart(line))
	}
	if starts[0] < 0 || starts[1] < 0 || starts[2] < 0 {
		t.Fatalf("missing due marker in lines")
	}
	if starts[0] != starts[1] || starts[0] != starts[2] {
		t.Fatalf("due columns misaligned: %v", starts)
	}
}

func focusInlineStart(line string) int {
	plain := stripANSI(line)
	return strings.Index(plain, iconPomodoro)
}

func TestTaskRowLayoutStableAcrossScrollWindows(t *testing.T) {
	m := fixtureModel(120, 40)
	m.taskFocusByTitle = map[string]ticktick.TaskFocusSummary{
		ticktick.NormalizeFocusTaskTitle("The God Equation"): {
			FullSessions: 2, LoggedSessions: 2, TotalSeconds: 3000,
		},
		ticktick.NormalizeFocusTaskTitle("The Millionaire Fastlane"): {
			FullSessions: 13, LoggedSessions: 13, TotalSeconds: 336*3600 + 43*60,
		},
	}
	all := make([]taskListRow, 0, 42)
	for i := 0; i < 40; i++ {
		all = append(all, taskListRow{Task: ticktick.Task{Title: "Filler Book"}})
	}
	all = append(all,
		taskListRow{Task: ticktick.Task{Title: "The God Equation"}},
		taskListRow{Task: ticktick.Task{Title: "The Millionaire Fastlane"}},
	)

	w := 80
	focusFull := maxTaskFocusInlineW(m, all)
	layoutFull := computeTaskRowLayout(all, w, focusFull)

	topWindow := all[:40]
	bottomWindow := all[38:]
	focusTop := maxTaskFocusInlineW(m, topWindow)
	layoutTop := computeTaskRowLayout(topWindow, w, focusTop)
	focusBottom := maxTaskFocusInlineW(m, bottomWindow)
	layoutBottom := computeTaskRowLayout(bottomWindow, w, focusBottom)

	if layoutTop.SuffixCol == layoutBottom.SuffixCol {
		t.Fatalf("scroll windows should produce different suffix cols without full-list layout")
	}
	if layoutFull.SuffixCol != layoutBottom.SuffixCol {
		t.Fatalf("full layout suffix=%d want long-window suffix=%d", layoutFull.SuffixCol, layoutBottom.SuffixCol)
	}

	shortLine := m.formatTaskLine(all[40].Task, 0, false, false, w, layoutFull)
	longLine := m.formatTaskLine(all[41].Task, 0, false, false, w, layoutFull)
	shortStart := focusInlineStart(shortLine)
	longStart := focusInlineStart(longLine)
	if shortStart < 0 || longStart < 0 || shortStart != longStart {
		t.Fatalf("focus stats misaligned: short=%d long=%d", shortStart, longStart)
	}
}

func TestFormatTaskLineShortTitlesStayCompact(t *testing.T) {
	m := fixtureModel(120, 40)
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "Morning", DueDate: "2026-02-22T08:00:00.000+0200"}},
		{Task: ticktick.Task{Title: "Bed", DueDate: "2026-03-08T20:00:00.000+0200"}},
	}
	layout := computeTaskRowLayout(rows, w, 0)
	if layout.SuffixCol > 24 {
		t.Fatalf("suffix col too far for short titles: %d", layout.SuffixCol)
	}
	line := m.formatTaskLine(rows[0].Task, 0, false, false, w, layout)
	if !strings.Contains(stripANSI(line), "0/1") {
		t.Fatalf("task estimate missing from row: %q", stripANSI(line))
	}
	if dueColStart(line) > 60 {
		t.Fatalf("due too far right on compact list: col=%d line=%q", dueColStart(line), stripANSI(line))
	}
}
