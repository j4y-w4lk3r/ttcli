package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestTaskFilterNarrowsWhileTyping(t *testing.T) {
	m := fixtureModel(100, 30)
	m.view = viewTasks
	m.paneFocus = paneTasks
	m.taskScope = TaskScopeOpen
	m.showCompleted = false
	m.showDeleted = false
	m.tasks = []ticktick.Task{
		{ID: "wake", Title: "wake up", ProjectID: "p0"},
		{ID: "milk", Title: "buy milk", ProjectID: "p0"},
		{ID: "mail", Title: "read mail", ProjectID: "p0"},
	}
	m.taskCursor = 2

	opened := pressKey(m, "/")
	if opened.mode != modeFilter {
		t.Fatalf("mode=%v", opened.mode)
	}

	typed := opened
	for _, r := range "mi" {
		next, _ := typed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		typed = next.(model)
	}
	if typed.mode != modeFilter {
		t.Fatalf("mode=%v", typed.mode)
	}
	rows := typed.visibleTaskRows()
	if len(rows) != 1 || rows[0].Task.Title != "buy milk" || typed.taskCursor != 0 {
		t.Fatalf("rows=%d cursor=%d titles=%v", len(rows), typed.taskCursor, titlesOf(rows))
	}
	view := stripANSI(typed.renderTasks(typed.layout()))
	if !strings.Contains(view, "buy milk") || strings.Contains(view, "wake up") || strings.Contains(view, "read mail") {
		t.Fatalf("live filter view:\n%s", view)
	}

	moved, _ := typed.Update(tea.KeyMsg{Type: tea.KeyDown})
	if moved.(model).mode != modeFilter {
		t.Fatal("arrows should stay in the filter")
	}

	closed, _ := typed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	kept := closed.(model)
	keptRows := kept.visibleTaskRows()
	if kept.mode != modeNormal || len(keptRows) != 1 {
		t.Fatalf("enter should keep the filter, mode=%v rows=%d", kept.mode, len(keptRows))
	}

	clearedMsg, _ := kept.Update(tea.KeyMsg{Type: tea.KeyEsc})
	cleared := clearedMsg.(model)
	if cleared.filterInput.Value() != "" || len(cleared.visibleTaskRows()) != 3 {
		t.Fatalf("esc filter=%q rows=%d", cleared.filterInput.Value(), len(cleared.visibleTaskRows()))
	}
}

func titlesOf(rows []taskListRow) []string {
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = row.Task.Title
	}
	return out
}
