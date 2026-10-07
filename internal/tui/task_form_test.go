package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestOpenEditTaskFormPrefill(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{
		ID:        "1",
		Title:     "Buy milk",
		Content:   "Get 2% organic<br/>check expiry",
		DueDate:   "2026-03-08T14:30:00.000+0100",
		StartDate: "2026-03-08T14:00:00.000+0100",
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
	if m.addTaskTimeInput.Value() != "14:00" {
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

func TestEditTaskNotesFallsBackToDescriptionAndEnterPreservesText(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{
		ID: "1", Title: "Task", Desc: "existing description",
	})
	if got := m.addTaskNotesInput.Value(); got != "existing description" {
		t.Fatalf("notes=%q want description fallback", got)
	}
	m.addTaskField = addTaskFieldNotes
	m.focusAddTaskField()
	out, _ := m.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyEnter})
	if got := out.addTaskNotesInput.Value(); !strings.Contains(got, "existing description") {
		t.Fatalf("Enter removed existing notes: %q", got)
	}
	if out.mode != modeEditTask {
		t.Fatalf("Enter in notes unexpectedly left edit mode: %v", out.mode)
	}
	if encoded := taskNotesToHTML(out.addTaskNotesInput.Value()); !strings.Contains(encoded, "existing description") {
		t.Fatalf("encoded notes lost existing text: %q", encoded)
	}
}

func TestEnterSavesTheEditFromTheCurrentField(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{ID: "1", Title: "Buy milk"})
	m.addTaskField = addTaskFieldTitle
	m.focusAddTaskField()
	out, cmd := m.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should save without walking the remaining fields")
	}
	if out.addTaskField != addTaskFieldTitle || out.mode != modeEditTask {
		t.Fatalf("field=%d mode=%v", out.addTaskField, out.mode)
	}
}

func TestTabMovesBetweenTaskFields(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{ID: "1", Title: "Buy milk"})
	m.addTaskField = addTaskFieldTitle
	out, _ := m.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyTab})
	if out.addTaskField != addTaskFieldNotes {
		t.Fatalf("tab field=%d", out.addTaskField)
	}
	out, _ = out.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyShiftTab})
	if out.addTaskField != addTaskFieldTitle {
		t.Fatalf("shift+tab field=%d", out.addTaskField)
	}
}

func TestCtrlSSavesFromNotes(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{ID: "1", Title: "Buy milk", Content: "note"})
	m.addTaskField = addTaskFieldNotes
	m.focusAddTaskField()
	out, cmd := m.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyCtrlS})
	if cmd == nil || out.mode != modeEditTask {
		t.Fatalf("ctrl+s cmd=%v mode=%v", cmd != nil, out.mode)
	}
	if !strings.Contains(out.addTaskNotesInput.Value(), "note") {
		t.Fatalf("notes=%q", out.addTaskNotesInput.Value())
	}
}

func TestOpenEditTaskFormPrefillsFocusAndRecurrence(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{
		ID: "1", Title: "Food",
		DueDate:    "2026-09-17T14:00:00.000+0200",
		StartDate:  "2026-09-17T13:00:00.000+0200",
		RepeatFlag: "RRULE:FREQ=WEEKLY;INTERVAL=1;WKST=MO;BYDAY=MO,TU,WE,TH,FR",
		RepeatFrom: 0,
		FocusSummaries: []ticktick.FocusSummary{{
			EstimatedDuration: 4500,
			EstimatedPomo:     3,
		}},
	})
	if m.addTaskFocusInput.Value() != "75m" {
		t.Fatalf("focus=%q", m.addTaskFocusInput.Value())
	}
	if addTaskRepeatOpts[m.addTaskRepeatIdx] != "weekdays" {
		t.Fatalf("repeat=%q", addTaskRepeatOpts[m.addTaskRepeatIdx])
	}
}

func TestParseTaskFocusPlanAndCustomRecurrence(t *testing.T) {
	m := fixtureModel(120, 40)
	m.mode = modeAddTask
	m.taskTitleInput.SetValue("Food")
	m.addTaskDueInput.SetValue("17/09/2026")
	m.addTaskTimeInput.SetValue("13:00")
	m.addTaskDurationInput.SetValue("60")
	m.addTaskFocusInput.SetValue("3p")
	m.addTaskRepeatIdx = repeatOptionIndex("RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=TU,TH")
	m.addTaskRepeatInput.SetValue("RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR")

	schedule, _, err := m.parseTaskFormSchedule()
	if err != nil {
		t.Fatal(err)
	}
	if schedule.Start.Hour() != 13 || schedule.Duration.Minutes() != 60 {
		t.Fatalf("schedule=%+v", schedule)
	}
	focusPlan, err := m.parseTaskFocusPlan()
	if err != nil {
		t.Fatal(err)
	}
	if focusPlan.Minutes != 75 || focusPlan.Pomos != 3 {
		t.Fatalf("focus plan=%+v", focusPlan)
	}
	recurrence, clear, err := m.parseTaskRecurrence(schedule)
	if err != nil || clear || recurrence == nil || !strings.Contains(recurrence.Rule, "BYDAY=MO,TU,WE,TH,FR") {
		t.Fatalf("recurrence=%+v clear=%v err=%v", recurrence, clear, err)
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

func TestTaskPlanningFormScrollsAtShortHeights(t *testing.T) {
	for _, height := range []int{18, 24} {
		m := fixtureModel(60, height)
		m.paneFocus = paneTasks
		m.openAddTaskForm()
		m.addTaskField = addTaskFieldPriority
		m.focusAddTaskField()
		assertViewOK(t, m, "short planning form")
		if view := stripANSI(m.View()); !strings.Contains(view, "Priority") {
			t.Fatalf("height=%d active field not visible:\n%s", height, view)
		}
	}
}

func TestTaskFormShowsCustomRuleConditionally(t *testing.T) {
	m := fixtureModel(80, 32)
	m.paneFocus = paneTasks
	m.openAddTaskForm()
	if strings.Contains(stripANSI(m.View()), "Custom rule") {
		t.Fatal("custom rule should be hidden for no recurrence")
	}
	m.addTaskRepeatIdx = repeatOptionIndex("RRULE:FREQ=WEEKLY;BYDAY=TU,TH")
	m.addTaskField = addTaskFieldRepeatRule
	m.focusAddTaskField()
	if !strings.Contains(stripANSI(m.View()), "Custom rule") {
		t.Fatal("custom rule should be visible for custom recurrence")
	}
}

func TestEditSubtaskCanBecomeANormalTask(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = append(m.tasks, ticktick.Task{ID: "parent", Title: "yu0"})
	m.openEditTaskForm(ticktick.Task{ID: "child", Title: "fm2", ParentID: "parent"})
	if !strings.Contains(m.renderAddTaskForm(100, 80), "subtask of yu0") {
		t.Fatal("edit form should show the parent")
	}
	m.addTaskField = addTaskFieldPriority
	out, _ := m.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyTab})
	if out.addTaskField != addTaskFieldParent {
		t.Fatalf("tab field=%d", out.addTaskField)
	}
	out, _ = out.updateAddTaskForm(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if out.editTaskParentIdx == 0 || !strings.Contains(out.renderAddTaskForm(100, 80), "normal task") {
		t.Fatal("] should offer to save the subtask as a normal task")
	}
}

func TestEditListedChildOffersANormalTask(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{
		{ID: "fm0", Title: "fm0", ChildIDs: []string{"fm3"}},
		{ID: "fm3", Title: "fm3", ProjectID: "fm"},
	}
	m.openEditTaskForm(m.tasks[1])
	if m.editTaskParentID != "fm0" {
		t.Fatalf("parent=%q", m.editTaskParentID)
	}
	if !strings.Contains(m.renderAddTaskForm(100, 80), "subtask of fm0") {
		t.Fatal("edit form should show the listing parent")
	}
}

func TestEditCompletedTaskUsesItsOwnList(t *testing.T) {
	m := fixtureModel(120, 40)
	m.projectID = allTasksID
	m.openEditTaskForm(ticktick.Task{
		ID: "done", ProjectID: "0123456789abcdef01234567", Title: "Finished", Status: 2,
	})
	if m.editProjectRef() != "0123456789abcdef01234567" {
		t.Fatalf("project=%q", m.editProjectRef())
	}
	if m.taskTitleInput.Value() != "Finished" {
		t.Fatalf("title=%q", m.taskTitleInput.Value())
	}
}

func TestNormalTaskHidesTheParentField(t *testing.T) {
	m := fixtureModel(120, 40)
	m.openEditTaskForm(ticktick.Task{ID: "1", Title: "fm0"})
	if strings.Contains(m.renderAddTaskForm(100, 80), "Parent") {
		t.Fatal("a normal task should not show the parent field")
	}
}
