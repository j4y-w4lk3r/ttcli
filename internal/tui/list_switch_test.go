package tui

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestListSwitchClearsTasksImmediately(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneLists
	if len(m.tasks) == 0 {
		t.Fatal("fixture should have tasks")
	}

	m = pressKey(m, "j")
	if len(m.tasks) != 0 {
		t.Fatalf("tasks should clear on list switch, got %d", len(m.tasks))
	}
	if !m.loading {
		t.Fatal("expected loading=true after list switch")
	}
	row, ok := m.currentListRow()
	if !ok {
		t.Fatal("no list row after j")
	}
	if m.projectID != row.node.ID {
		t.Fatalf("projectID=%q want %q", m.projectID, row.node.ID)
	}
	if m.projectName != row.node.Name {
		t.Fatalf("projectName=%q want %q", m.projectName, row.node.Name)
	}
}

func TestStaleTasksLoadedIgnored(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneLists
	oldID := m.projectID

	m = pressKey(m, "j")
	row, ok := m.currentListRow()
	if !ok {
		t.Fatal("no list row")
	}
	if row.node.ID == oldID {
		t.Fatal("expected different list after j")
	}

	out, _ := m.Update(tasksLoadedMsg{
		projectID:   oldID,
		projectName: "List0",
		tasks:       fixtureModel(120, 40).tasks,
	})
	m = out.(model)
	if len(m.tasks) != 0 {
		t.Fatalf("stale load applied %d tasks", len(m.tasks))
	}
	if !m.loading {
		t.Fatal("should still be loading after stale response")
	}
	if m.projectID != row.node.ID {
		t.Fatalf("projectID changed to %q", m.projectID)
	}

	out, _ = m.Update(tasksLoadedMsg{
		projectID:   row.node.ID,
		projectName: row.node.Name,
		tasks: []ticktick.Task{
			{ID: "x1", Title: "reloaded task", DueDate: "2026-08-30"},
		},
	})
	m = out.(model)
	if m.loading {
		t.Fatal("expected loading=false after current list loads")
	}
	if len(m.tasks) != 1 || m.tasks[0].Title != "reloaded task" {
		t.Fatalf("got tasks: %+v", m.tasks)
	}
}

func TestStaleTasksLoadedAfterCurrentIgnored(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneLists
	oldID := m.projectID
	oldTasks := append([]ticktick.Task(nil), m.tasks...)

	m = pressKey(m, "j")
	row, ok := m.currentListRow()
	if !ok {
		t.Fatal("no list row")
	}

	out, _ := m.Update(tasksLoadedMsg{
		projectID:   row.node.ID,
		projectName: row.node.Name,
		tasks: []ticktick.Task{
			{ID: "new1", Title: "new list task"},
		},
	})
	m = out.(model)

	out, _ = m.Update(tasksLoadedMsg{
		projectID:   oldID,
		projectName: "List0",
		tasks:       oldTasks,
	})
	m = out.(model)
	if len(m.tasks) != 1 || m.tasks[0].ID != "new1" {
		t.Fatalf("stale load overwrote current tasks: %+v", m.tasks)
	}
}
