package tui

import (
	"strings"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestOpenEditTaskFormPrefill(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{
		ID:        "1",
		Title:     "Buy milk",
		Content:   "Get 2% organic<br/>check expiry",
		DueDate:   "2026-03-08T14:30:00.000+0200",
		StartDate: "2026-03-08T14:00:00.000+0200",
		Priority:  3,
		Reminder:  "TRIGGER:-PT15M",
	})
	if m.editTaskID != "1" {
		t.Fatalf("edit id=%q", m.editTaskID)
	}
	if m.taskTitleInput.Value() != "Buy milk" {
		t.Fatalf("task title=%q", m.taskTitleInput.Value())
	}
	if got := m.addTaskNotesInput.Value(); !strings.Contains(got, "Get 2% organic") || !strings.Contains(got, "check expiry") {
		t.Fatalf("notes=%q", got)
	}
	if m.addTaskDueInput.Value() != "08/03/2026" {
		t.Fatalf("due=%q", m.addTaskDueInput.Value())
	}
	if m.addTaskTimeInput.Value() != "14:30" {
		t.Fatalf("time=%q", m.addTaskTimeInput.Value())
	}
	if m.addTaskDurationInput.Value() != "30" {
		t.Fatalf("duration=%q", m.addTaskDurationInput.Value())
	}
	if m.addTaskPriorityIdx != 2 {
		t.Fatalf("priority idx=%d", m.addTaskPriorityIdx)
	}
	if m.addTaskReminderIdx != 3 {
		t.Fatalf("reminder idx=%d want 15m", m.addTaskReminderIdx)
	}
}

func TestParseTaskFormScheduleClearDue(t *testing.T) {
	m := fixtureModel(120, 40)
	m.mode = modeEditTask
	m.taskTitleInput.SetValue("Title only")
	_, clearDue, err := m.parseTaskFormSchedule()
	if err != nil {
		t.Fatal(err)
	}
	if !clearDue {
		t.Fatal("empty due should clear")
	}
}

func TestTaskTitleInputViewSingleLine(t *testing.T) {
	in := newAddTaskFieldInput("")
	in.SetValue("Setup HairDress")
	in.Focus()
	view := taskTitleInputView(in, 40)
	if strings.Contains(view, "\n") {
		t.Fatalf("expected single line, got %q", view)
	}
	if !strings.Contains(view, "Setup HairDress") {
		t.Fatalf("missing title in %q", view)
	}
}

func TestOpenEditTaskFormUsesTaskTitleInput(t *testing.T) {
	m := fixtureModel(120, 40)
	m.addInput.SetValue("leftover list name")
	m.openEditTaskForm(ticktick.Task{ID: "1", Title: "Edit me"})
	if m.addInput.Value() != "leftover list name" {
		t.Fatalf("addInput should be untouched, got %q", m.addInput.Value())
	}
	if m.taskTitleInput.Value() != "Edit me" {
		t.Fatalf("taskTitleInput=%q", m.taskTitleInput.Value())
	}
}
