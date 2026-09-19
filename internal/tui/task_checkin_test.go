package tui

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/taskcheckin"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestCalendarIndexAddsLocalHistoryWithoutMovingDueDate(t *testing.T) {
	today := dateOnly(time.Now())
	nextWeek := today.AddDate(0, 0, 7)
	task := ticktick.Task{
		ID: "groceries", ProjectID: "home", Title: "Groceries",
		DueDate: dateKey(nextWeek), IsAllDay: true,
	}
	record := taskcheckin.Record{
		TaskID: task.ID, SeriesID: task.ID, ProjectID: task.ProjectID,
		Title: task.Title, Date: dateKey(today), CompletedAt: time.Now(),
	}
	idx := buildCalendarIndex([]ticktick.Task{task}, nil, []taskcheckin.Record{record})

	todayEntries := idx.on(today)
	if len(todayEntries) != 1 || !todayEntries[0].Done() || !todayEntries[0].LocalOnly() {
		t.Fatalf("today entries=%+v", todayEntries)
	}
	futureEntries := idx.on(nextWeek)
	if len(futureEntries) != 1 || futureEntries[0].Done() {
		t.Fatalf("future entries=%+v", futureEntries)
	}
	if futureEntries[0].Task.DueDate != dateKey(nextWeek) {
		t.Fatalf("due date moved to %q", futureEntries[0].Task.DueDate)
	}
}

func TestCalendarIndexDeduplicatesNativeHistory(t *testing.T) {
	today := dateOnly(time.Now())
	completedAt := today.Add(12 * time.Hour).UTC().Format("2006-01-02T15:04:05.000-0700")
	task := ticktick.Task{
		ID: "occurrence", ProjectID: "home", Title: "Bins",
		RepeatFlag: "RRULE:FREQ=WEEKLY", RepeatTaskID: "series",
		CompletedT: completedAt, Status: 2,
	}
	record := taskcheckin.Record{
		TaskID: task.ID, SeriesID: "series", ProjectID: "home", Title: "Bins",
		Date: dateKey(today), CompletedAt: time.Now(), Native: true,
	}
	idx := buildCalendarIndex(nil, []ticktick.Task{task}, []taskcheckin.Record{record})
	entries := idx.on(today)
	if len(entries) != 1 {
		t.Fatalf("entries=%+v", entries)
	}
	if entries[0].State != calEntryNativeDone || entries[0].LocalOnly() {
		t.Fatalf("state=%v", entries[0].State)
	}
}

func TestCalendarIndexExpandsAndDeduplicatesRecurringDays(t *testing.T) {
	start := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	open := ticktick.Task{
		ID: "food", RepeatTaskID: "food-series", Title: "Food",
		DueDate: "2026-09-17", RepeatFirstDate: "2026-09-14",
		RepeatFlag: "RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR",
		IsAllDay:   true,
	}
	completed := ticktick.Task{
		ID: "food-done", RepeatTaskID: "food-series", Title: "Food",
		RepeatFlag: "RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=MO,TU,WE,TH,FR",
		CompletedT: "2026-09-18T12:00:00.000+0000",
	}
	idx := buildCalendarIndexForRange(
		[]ticktick.Task{open}, []ticktick.Task{completed}, nil,
		start, start.AddDate(0, 0, 4),
	)
	if len(idx.on(start)) != 1 || len(idx.on(start.AddDate(0, 0, 1))) != 1 ||
		len(idx.on(start.AddDate(0, 0, 4))) != 1 {
		t.Fatalf("dates=%+v", idx.byDate)
	}
	if !idx.on(start.AddDate(0, 0, 1))[0].Done() {
		t.Fatal("native completion should replace generated open occurrence")
	}
}

func TestCalendarExpansionRetainsCurrentOverdueOccurrence(t *testing.T) {
	today := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	task := ticktick.Task{
		ID: "daily", Title: "Daily",
		DueDate:         today.AddDate(0, 0, -1).Format("2006-01-02"),
		RepeatFirstDate: today.AddDate(0, 0, -5).Format("2006-01-02"),
		RepeatFlag:      "RRULE:FREQ=DAILY;INTERVAL=1", IsAllDay: true,
	}
	idx := buildCalendarIndexForRange([]ticktick.Task{task}, nil, nil, today, today)
	if len(idx.on(today)) != 1 || len(idx.overdueBefore(today)) != 1 {
		t.Fatalf("dates=%+v", idx.byDate)
	}
	if idx.on(today)[0].NativeCheckin() {
		t.Fatal("generated today occurrence must not advance overdue native occurrence")
	}
	if !idx.overdueBefore(today)[0].NativeCheckin() {
		t.Fatal("current overdue occurrence should retain native completion")
	}
}

func TestGeneratedRecurringOccurrenceUsesLocalCheckin(t *testing.T) {
	task := ticktick.Task{
		ID: "food", RepeatFlag: "RRULE:FREQ=DAILY;INTERVAL=1",
	}
	if !(calEntry{Task: task}).NativeCheckin() {
		t.Fatal("current open occurrence should use native completion")
	}
	if (calEntry{Task: task, Generated: true}).NativeCheckin() {
		t.Fatal("generated historical occurrence must not advance current TickTick occurrence")
	}
}

func TestOrdinaryDoneActionGuardsRecurringSeries(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.taskCursor = 0
	m.tasks[0].RepeatFlag = "RRULE:FREQ=DAILY;INTERVAL=1"
	m.applyFilter()
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil || !strings.Contains(out.errMsg, "use x") {
		t.Fatalf("cmd=%v error=%q", cmd != nil, out.errMsg)
	}
}

func TestTaskXCreatesAndUndoesLocalCheckin(t *testing.T) {
	m := fixtureModel(100, 30)
	m.checkinStore = taskcheckin.NewStoreAt(filepath.Join(t.TempDir(), "checkins.json"))
	m.paneFocus = paneTasks
	task := m.tasks[0]

	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil || out.pendingCheckin == "" {
		t.Fatal("expected pending check-in command")
	}
	msg := cmd()
	updatedModel, _ := out.Update(msg)
	updated := updatedModel.(model)
	if len(updated.taskCheckins) != 1 || updated.taskCheckins[0].TaskID != task.ID {
		t.Fatalf("checkins=%+v", updated.taskCheckins)
	}

	out, cmd = updated.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("expected undo command")
	}
	msg = cmd()
	updatedModel, _ = out.Update(msg)
	updated = updatedModel.(model)
	if len(updated.taskCheckins) != 0 {
		t.Fatalf("checkins after undo=%+v", updated.taskCheckins)
	}
}

func TestCalendarCheckinRejectsFutureDate(t *testing.T) {
	m := fixtureModel(100, 30)
	m.view = viewCalendar
	m.calMode = calModeDay
	m.calDate = dateOnly(time.Now()).AddDate(0, 0, 1)
	task := ticktick.Task{
		ID: "future", ProjectID: "home", Title: "Future",
		DueDate: dateKey(m.calDate), IsAllDay: true,
	}
	out, cmd := m.beginTaskCheckin(task, m.calDate, false, false)
	if cmd != nil || out.errMsg == "" {
		t.Fatalf("cmd=%v err=%q", cmd, out.errMsg)
	}
}
