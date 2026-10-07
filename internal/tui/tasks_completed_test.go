package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestTrashTaskLineStaysReadable(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(80, 24)
	task := ticktick.Task{ID: "bin", Title: "cable", Deleted: 1}
	for _, selected := range []bool{false, true} {
		line := m.formatTaskLine(task, 0, selected, false, 80, taskRowLayout{TitleColW: 24, SuffixCol: 28})
		if strings.Contains(line, "\x1b[9m") || strings.Contains(line, ";9;") || strings.Contains(line, ";9m") {
			t.Fatalf("selected=%v struck line=%q", selected, line)
		}
		if !strings.Contains(stripANSI(line), "cable") {
			t.Fatalf("selected=%v line=%q", selected, stripANSI(line))
		}
		plain := stripANSI(line)
		if strings.Contains(plain, iconTaskSel) {
			t.Fatalf("selected=%v trash row has a chevron: %q", selected, plain)
		}
		if selected && !strings.Contains(line, "48;2;") {
			t.Fatalf("selected trash row has no focus bar: %q", line)
		}
		if selected && strings.Count(line, "48;2;") != 1 {
			t.Fatalf("selected trash bar is split: %q", line)
		}
		if selected && lipgloss.Width(stripANSI(line)) != 80 {
			t.Fatalf("selected trash bar width=%d", lipgloss.Width(stripANSI(line)))
		}
		if !selected && strings.Contains(line, "48;2;") {
			t.Fatalf("unselected trash row is filled: %q", line)
		}
		if selected {
			continue
		}
		light := 0
		if seq := foregroundSeq(line); seq != "" {
			var r, g, b int
			if _, err := fmt.Sscanf(seq, "38;2;%d;%d;%d", &r, &g, &b); err == nil {
				light = r
			}
		}
		if light < 200 {
			t.Fatalf("selected=%v red channel=%d line=%q", selected, light, line)
		}
	}
	marked := m.formatTaskLine(task, 0, false, true, 80, taskRowLayout{TitleColW: 24, SuffixCol: 28})
	teal := strings.Contains(marked, "38;2;148;226;213") || strings.Contains(marked, "38;2;147;226;213")
	if strings.Contains(marked, "48;2;") || !teal {
		t.Fatalf("marked trash row=%q", marked)
	}
}

func TestClosedTaskListsUseDistinctColorsWithoutStrike(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(100, 30)
	layout := taskRowLayout{TitleColW: 24, SuffixCol: 28}
	cases := []ticktick.Task{
		{ID: "bin", Title: "cable", Deleted: 1},
		{ID: "done", Title: "finished", Status: 2},
		{ID: "skip", Title: "abandoned", Status: -1},
	}
	seen := map[string]bool{}
	for _, task := range cases {
		for _, selected := range []bool{false, true} {
			line := m.formatTaskLine(task, 0, selected, false, 90, layout)
			plain := stripANSI(line)
			if strings.Contains(line, "\x1b[9m") || strings.Contains(line, ";9;") || strings.Contains(line, ";9m") {
				t.Fatalf("%s selected=%v struck: %q", task.ID, selected, line)
			}
			if strings.Contains(plain, iconTaskSel) {
				t.Fatalf("%s selected=%v has a chevron: %q", task.ID, selected, plain)
			}
			if selected != strings.Contains(line, "48;2;") {
				t.Fatalf("%s selected=%v focus bar: %q", task.ID, selected, line)
			}
			if !strings.Contains(plain, task.Title) {
				t.Fatalf("%s selected=%v line=%q", task.ID, selected, plain)
			}
		}
		seq := foregroundSeq(m.formatTaskLine(task, 0, false, false, 90, layout))
		if seen[seq] || seq == "" {
			t.Fatalf("%s color=%q seen=%v", task.ID, seq, seen[seq])
		}
		seen[seq] = true
	}
}

func TestVisibleTasksHidesCompletedByDefault(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	if got := len(m.visibleTasks()); got != 1 {
		t.Fatalf("want 1 open task, got %d", got)
	}
}

func TestVisibleTasksShowCompletedToggle(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	m.showCompleted = true
	if got := len(m.visibleTasks()); got != 2 {
		t.Fatalf("want 2 tasks, got %d", got)
	}
}

func TestToggleCompletedKey(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	m = pressKey(m, "c")
	if m.effectiveTaskScope() != TaskScopeDone {
		t.Fatalf("scope=%s want done", m.effectiveTaskScope())
	}
	if tasks := m.visibleTasks(); len(tasks) != 1 || tasks[0].ID != "2" {
		t.Fatalf("expected only done task, got %+v", tasks)
	}
}

func TestTaskCounts(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Status: 0},
		{ID: "2", Status: 2},
		{ID: "3", Status: 2},
		{ID: "4", Status: 0, Deleted: 1},
	}
	open, done, trashed := m.taskCounts()
	if open != 1 || done != 2 || trashed != 1 {
		t.Fatalf("open=%d done=%d trashed=%d", open, done, trashed)
	}
}

func TestVisibleTasksHideTrashedByDefault(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "trashed", Status: 0, Deleted: 1},
	}
	if got := len(m.visibleTasks()); got != 1 {
		t.Fatalf("want 1 task, got %d", got)
	}
	m.showDeleted = true
	if got := len(m.visibleTasks()); got != 2 {
		t.Fatalf("want 2 tasks, got %d", got)
	}
}

func TestToggleTrashedKey(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "trashed", Status: 0, Deleted: 2},
	}
	m = pressKey(pressKey(m, "c"), "c")
	if m.effectiveTaskScope() != TaskScopeTrash {
		t.Fatalf("scope=%s want trash", m.effectiveTaskScope())
	}
	if tasks := m.visibleTasks(); len(tasks) != 1 || tasks[0].ID != "2" {
		t.Fatalf("expected only trashed task, got %+v", tasks)
	}
}
