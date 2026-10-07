package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func allListModel() model {
	m := fixtureModel(100, 30)
	m.projectID = allTasksID
	m.projectName = "All"
	m.paneFocus = paneTasks
	for i, row := range m.listRows {
		if row.node.ID == allTasksID {
			m.listCursor = i
			break
		}
	}
	m.tasks = []ticktick.Task{
		{ID: "open", Title: "Open task", ProjectID: "p1"},
		{ID: "done", Title: "Finished task", ProjectID: "p1", Status: 2},
		{ID: "later", Title: "Left it", ProjectID: "p1", Status: -1},
	}
	return m
}

func TestAllScopeShowsOpenCompletedAndWontDo(t *testing.T) {
	m := allListModel()
	m.taskScope = TaskScopeAll
	got := map[string]bool{}
	for _, row := range m.visibleTaskRows() {
		got[row.Task.ID] = true
	}
	for _, id := range []string{"open", "done", "later"} {
		if !got[id] {
			t.Fatalf("missing %s in %v", id, got)
		}
	}
	if hint := m.openDoneHint(); !strings.Contains(hint, "All · 3 total") {
		t.Fatalf("hint=%q", hint)
	}
}

func TestAllOpenScopeHidesCompletedTasks(t *testing.T) {
	m := allListModel()
	m.taskScope = TaskScopeOpen
	rows := m.visibleTaskRows()
	if len(rows) != 1 || rows[0].Task.ID != "open" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestAllDoneScopeShowsOnlyCompletedTasks(t *testing.T) {
	m := allListModel()
	m.taskScope = TaskScopeDone
	rows := m.visibleTaskRows()
	if len(rows) != 1 || rows[0].Task.ID != "done" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestChangingAllScopeReloadsTheFeed(t *testing.T) {
	m := allListModel()
	m.taskScope = TaskScopeOpen
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	if cmd == nil || out.effectiveTaskScope() != TaskScopeDone || !out.loading {
		t.Fatalf("scope=%s loading=%v cmd=%v", out.effectiveTaskScope(), out.loading, cmd != nil)
	}
	msg, ok := cmd().(tasksLoadedMsg)
	if !ok || msg.projectID != allTasksID || msg.err == nil {
		t.Fatalf("msg=%+v ok=%v", msg, ok)
	}
}
