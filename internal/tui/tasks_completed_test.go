package tui

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

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
