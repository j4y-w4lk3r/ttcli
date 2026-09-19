package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestSortTasksByDue(t *testing.T) {
	tasks := []ticktick.Task{
		{Title: "afternoon", DueDate: "2026-08-30T15:00:00.000+0200"},
		{Title: "all day", DueDate: "2026-08-30", IsAllDay: true},
		{Title: "morning", DueDate: "2026-08-30T09:00:00.000+0200"},
	}
	sortTasksByDue(tasks)
	if tasks[0].Title != "all day" {
		t.Fatalf("all-day first, got %q", tasks[0].Title)
	}
	if tasks[1].Title != "morning" || tasks[2].Title != "afternoon" {
		t.Fatalf("order=%q,%q,%q", tasks[0].Title, tasks[1].Title, tasks[2].Title)
	}
}

func TestCalDayRowsOverdueAndTimeline(t *testing.T) {
	day := dateOnly(time.Now())
	yesterday := day.AddDate(0, 0, -1).Format("2006-01-02")
	today := day.Format("2006-01-02")
	idx := buildCalIndex([]ticktick.Task{
		{Title: "late", DueDate: yesterday + "T10:00:00.000+0200"},
		{Title: "timed", DueDate: today + "T10:00:00.000+0200"},
		{Title: "anytime", DueDate: today, IsAllDay: true},
	})
	m := model{calDate: day, uiSettings: uiSettings{CalendarDayShowOverdue: true}}
	rows, tasks := m.calDayRows(idx)
	if len(tasks) != 3 {
		t.Fatalf("tasks=%d", len(tasks))
	}
	var kinds []string
	for _, r := range rows {
		kinds = append(kinds, r.kind)
	}
	foundOverdue := false
	foundAllDay := false
	foundTask := false
	foundSlot := false
	foundAllDayHdr := false
	for _, k := range kinds {
		switch k {
		case "overdue":
			foundOverdue = true
		case "allday":
			foundAllDay = true
		case "allday-hdr":
			foundAllDayHdr = true
		case "task":
			foundTask = true
		case "slot":
			foundSlot = true
		}
	}
	if !foundOverdue || !foundAllDay || !foundTask || !foundSlot {
		t.Fatalf("kinds=%v", kinds)
	}
	if !foundAllDayHdr {
		t.Fatal("date-only tasks should appear in the pinned all-day section")
	}
}

func TestCalTaskIsPastDue(t *testing.T) {
	now := time.Date(2026, 8, 30, 16, 0, 0, 0, time.Local)
	day := dateOnly(now)
	past := ticktick.Task{DueDate: "2026-08-30T10:00:00.000+0200"}
	future := ticktick.Task{DueDate: "2026-08-30T18:00:00.000+0200"}
	old := ticktick.Task{DueDate: "2026-08-29T10:00:00.000+0200"}
	if !calTaskIsPastDue(past, day, now) {
		t.Fatal("past today should be overdue")
	}
	if calTaskIsPastDue(future, day, now) {
		t.Fatal("future today should not be overdue")
	}
	if !calTaskIsPastDue(old, day, now) {
		t.Fatal("old day should be overdue")
	}
}

func TestCalDayTimelineUsesTaskStartAndDuration(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	entries := []calEntry{{Task: ticktick.Task{
		ID:        "timed",
		Title:     "Deep work",
		StartDate: "2026-09-17T09:00:00.000+0200",
		DueDate:   "2026-09-17T10:30:00.000+0200",
	}}}
	rows := buildCalDayTimeline(entries, day, day.Add(12*time.Hour), 0, planning.DefaultConfig())
	var taskRow calDayRow
	for _, row := range rows {
		if row.kind == "task" {
			taskRow = row
			break
		}
	}
	if taskRow.hourLabel != "09:00" || taskRow.end.Format("15:04") != "10:30" {
		t.Fatalf("task row=%+v", taskRow)
	}
	if taskRow.estimate.Minutes != 90 || !taskRow.estimate.Explicit {
		t.Fatalf("estimate=%+v", taskRow.estimate)
	}
}

func TestCalDayPlanSummaryDisclosesInferredEstimates(t *testing.T) {
	plan := planning.DayPlan{
		NeededMinutes:    50,
		AvailableMinutes: 100,
		InferredTasks:    2,
		Feasibility:      planning.FeasibilityComfortable,
		Evidence:         planning.EvidenceLow,
	}
	summary := stripANSI(renderCalDayPlanSummary(plan))
	for _, want := range []string{"~50m needed", "1h40m available", "ON TRACK", "low evidence", "2 inferred"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q missing %q", summary, want)
		}
	}
}

func TestCalDayCoachingUsesDateKeyedTaskFocus(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	task := ticktick.Task{
		ID: "food", Title: "Food", DueDate: "2026-09-17",
		IsAllDay: true,
		FocusSummaries: []ticktick.FocusSummary{{
			EstimatedDuration: 3600, EstimatedPomo: 3,
		}},
	}
	var record ticktick.FocusRecord
	if err := json.Unmarshal([]byte(`{
		"id":"focus",
		"startTime":"2026-09-17T09:00:00.000+0200",
		"endTime":"2026-09-17T09:30:00.000+0200",
		"tasks":[{"taskId":"food","title":"Food"}]
	}`), &record); err != nil {
		t.Fatal(err)
	}
	m := fixtureModel(120, 40)
	m.calFocusByDate = map[string]*ticktick.FocusStats{
		dateKey(day): {Date: dateKey(day), Records: []ticktick.FocusRecord{record}},
	}
	plan := m.buildCalDayCoaching(
		buildCalendarIndex([]ticktick.Task{task}, nil, nil),
		day, day.Add(12*time.Hour),
	)
	if plan.PlannedMinutes != 60 || plan.LoggedMinutes != 30 || plan.RemainingMinutes != 30 {
		t.Fatalf("plan=%+v", plan)
	}
	if plan.PlannedPomos != 3 || plan.CompletedPomos != 1 {
		t.Fatalf("pomos=%+v", plan)
	}
}

func TestCalDayHidesOverdueRowsButStillCountsWork(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	day := dateOnly(time.Now())
	idx := buildCalIndex([]ticktick.Task{
		{
			ID: "late", Title: "Late",
			DueDate: dateKey(day.AddDate(0, 0, -1)), IsAllDay: true,
			FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 3600}},
		},
		{ID: "today", Title: "Today", DueDate: dateKey(day), IsAllDay: true},
	})
	m := model{
		calDate: day, calFocusByDate: map[string]*ticktick.FocusStats{},
		uiSettings: defaultUISettings(),
	}
	rows, tasks := m.calDayRows(idx)
	if len(tasks) != 1 {
		t.Fatalf("visible tasks=%d", len(tasks))
	}
	for _, row := range rows {
		if row.kind == "overdue" {
			t.Fatal("overdue row should be hidden by default")
		}
	}
	plan := m.buildCalDayCoaching(idx, day, time.Now())
	if plan.OverdueCount != 1 || plan.PlannedMinutes < 60 {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestCalendarDayOverdueTogglePersists(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	m := model{calMode: calModeDay, uiSettings: defaultUISettings()}
	updated, _ := m.updateCalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	if !updated.uiSettings.CalendarDayShowOverdue {
		t.Fatal("overdue toggle did not turn on")
	}
	if loaded := loadUISettings(); !loaded.CalendarDayShowOverdue {
		t.Fatal("overdue toggle was not persisted")
	}
}

func TestCalDayCoachingIncludesActiveSession(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now()
	session, err := focus.Start(25*time.Minute, "food", "Food", "home", "Home")
	if err != nil {
		t.Fatal(err)
	}
	session.StartedAt = now.Add(-10 * time.Minute)
	session.SegmentStartedAt = session.StartedAt
	if err := session.Save(); err != nil {
		t.Fatal(err)
	}
	day := dateOnly(now)
	task := ticktick.Task{
		ID: "food", Title: "Food", DueDate: dateKey(day), IsAllDay: true,
		FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 3600}},
	}
	m := fixtureModel(120, 40)
	m.calFocusByDate = map[string]*ticktick.FocusStats{}
	plan := m.buildCalDayCoaching(buildCalIndex([]ticktick.Task{task}), day, now)
	if plan.LoggedMinutes < 9 || plan.LoggedMinutes > 11 || plan.RemainingMinutes > 51 {
		t.Fatalf("active session not credited: %+v", plan)
	}
}

func TestCalDayTimelineShowsLoggedFocusForAllDayTask(t *testing.T) {
	day := time.Date(2026, 9, 14, 0, 0, 0, 0, time.Local)
	start := time.Date(2026, 9, 14, 6, 20, 0, 0, time.Local)
	end := time.Date(2026, 9, 14, 6, 45, 0, 0, time.Local)
	task := ticktick.Task{
		ID: "run", Title: "Running", DueDate: "2026-09-14", IsAllDay: true,
	}
	record := ticktick.FocusRecord{
		ID:        "focus-run",
		StartTime: start.Format("2006-01-02T15:04:05.000-0700"),
		EndTime:   end.Format("2006-01-02T15:04:05.000-0700"),
	}
	record.SetTaskTitle("Running")
	if len(record.Tasks) > 0 {
		record.Tasks[0].TaskID = "run"
		record.Tasks[0].StartTime = record.StartTime
		record.Tasks[0].EndTime = record.EndTime
	}
	m := fixtureModel(120, 40)
	m.calDate = day
	m.calFocusByDate = map[string]*ticktick.FocusStats{
		dateKey(day): {Date: dateKey(day), Records: []ticktick.FocusRecord{record}},
	}
	rows, _ := m.calDayRows(buildCalIndex([]ticktick.Task{task}))
	var logged calDayRow
	for _, row := range rows {
		if row.logged && row.entry.Task.Title == "Running" {
			logged = row
			break
		}
	}
	if logged.kind != "task" {
		t.Fatalf("logged running session missing")
	}
	if logged.hourLabel != "06:20" || logged.end.Format("15:04") != "06:45" {
		t.Fatalf("logged row=%+v", logged)
	}
}

func TestCalendarDayViewDoesNotKeepMonthCellBottoms(t *testing.T) {
	m := fixtureModel(220, 60)
	m.view = viewCalendar
	m.calMode = calModeMonth
	m.calDate = time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local)
	l := m.layout()
	month := stripANSI(m.renderCalendarView(l))
	if !strings.Contains(month, "┘└") {
		t.Fatalf("month view should join cell bottoms:\n%s", month)
	}
	m.calMode = calModeDay
	day := stripANSI(m.renderCalendarView(l))
	if strings.Contains(day, "┘└") {
		t.Fatalf("month cell bottoms leaked into day view:\n%s", day)
	}
}
