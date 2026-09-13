package tui

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestToggleTaskMarkAndAdvance(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	m.taskCursor = 0
	tasks := m.visibleTasks()
	if len(tasks) < 3 {
		t.Fatal("need at least 3 tasks")
	}

	m = m.toggleTaskMarkAndAdvance()
	if !m.isTaskMarked(tasks[0].ID) {
		t.Fatal("expected first task marked")
	}
	if m.taskCursor != 1 {
		t.Fatalf("cursor=%d want 1", m.taskCursor)
	}

	m = m.toggleTaskMarkAndAdvance()
	if !m.isTaskMarked(tasks[1].ID) {
		t.Fatal("expected second task marked")
	}
	if m.taskCursor != 2 {
		t.Fatalf("cursor=%d want 2", m.taskCursor)
	}
}

func TestTaskMarkToggleAndMove(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	tasks := m.visibleTasks()
	if len(tasks) < 2 {
		t.Fatal("need at least 2 tasks")
	}

	m = m.toggleTaskMark(tasks[0].ID)
	m = m.toggleTaskMark(tasks[1].ID)
	if m.markedTaskCount() != 2 {
		t.Fatalf("marked=%d want 2", m.markedTaskCount())
	}

	toMove := m.tasksToMove()
	if len(toMove) != 2 {
		t.Fatalf("tasksToMove=%d want 2", len(toMove))
	}

	m = m.clearTaskMarks()
	toMove = m.tasksToMove()
	if len(toMove) != 1 {
		t.Fatalf("fallback move=%d want 1", len(toMove))
	}
	if toMove[0].ID != tasks[m.taskCursor].ID {
		t.Fatalf("fallback task id mismatch")
	}
}

func TestMarkAllVisibleTasks(t *testing.T) {
	m := fixtureModel(120, 40)
	m = m.markAllVisibleTasks()
	if m.markedTaskCount() != len(m.visibleTasks()) {
		t.Fatalf("marked=%d visible=%d", m.markedTaskCount(), len(m.visibleTasks()))
	}
}

func TestMarkAllVisibleTasksIncludesDone(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	m.showCompleted = true
	m = m.markAllVisibleTasks()
	if m.markedTaskCount() != 2 {
		t.Fatalf("marked=%d want 2", m.markedTaskCount())
	}
}

func TestTasksToMoveDoneTask(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	m.showCompleted = true
	m.taskCursor = 1
	toMove := m.tasksToMove()
	if len(toMove) != 1 || toMove[0].ID != "2" {
		t.Fatalf("tasksToMove=%v want done task", toMove)
	}

	m = m.toggleTaskMark("2")
	toMove = m.tasksToMove()
	if len(toMove) != 1 || !toMove[0].Done() {
		t.Fatalf("marked done move=%v", toMove)
	}
}

func TestTasksToMoveMarkedDoneWhileHidden(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done_a", Status: 2},
		{ID: "3", Title: "done_b", Status: 2},
	}
	m.showCompleted = true
	m = m.toggleTaskMark("2")
	m = m.toggleTaskMark("3")
	m.showCompleted = false

	toMove := m.tasksToMove()
	if len(toMove) != 2 {
		t.Fatalf("tasksToMove=%d want 2 marked done while hidden", len(toMove))
	}
}

func TestMarkAllVisibleDoneTasks(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done", Status: 2},
	}
	m.showCompleted = true
	m = m.markAllVisibleDoneTasks()
	if m.markedTaskCount() != 1 {
		t.Fatalf("marked=%d want 1 done task", m.markedTaskCount())
	}
	if m.isTaskMarked("1") {
		t.Fatal("open task should not be marked")
	}
}

func TestTasksToDeleteMarked(t *testing.T) {
	m := fixtureModel(120, 40)
	tasks := m.visibleTasks()
	m = m.toggleTaskMark(tasks[0].ID)
	m = m.toggleTaskMark(tasks[1].ID)
	toDelete := m.tasksToDelete()
	if len(toDelete) != 2 {
		t.Fatalf("tasksToDelete=%d want 2", len(toDelete))
	}
}

func TestTasksToDeleteMarkedWhileHidden(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "1", Title: "open", Status: 0},
		{ID: "2", Title: "done_a", Status: 2},
		{ID: "3", Title: "done_b", Status: 2},
	}
	m.showCompleted = true
	m = m.toggleTaskMark("2")
	m = m.toggleTaskMark("3")
	m.showCompleted = false

	toDelete := m.tasksToDelete()
	if len(toDelete) != 2 {
		t.Fatalf("tasksToDelete=%d want 2 marked while hidden", len(toDelete))
	}
}

func TestTasksToCompleteMarked(t *testing.T) {
	m := fixtureModel(120, 40)
	tasks := m.visibleTasks()
	m = m.toggleTaskMark(tasks[0].ID)
	toComplete := m.tasksToComplete()
	if len(toComplete) != 1 {
		t.Fatalf("toComplete=%d want 1", len(toComplete))
	}
}
