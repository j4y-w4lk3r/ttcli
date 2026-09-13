package tui

import (
	"testing"
	"time"

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
	m := model{calDate: day}
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
	if foundAllDayHdr {
		t.Fatal("date-only tasks should appear on the timeline, not a separate all-day section")
	}
}

func TestCalTaskShouldBeDone(t *testing.T) {
	now := time.Date(2026, 8, 30, 16, 0, 0, 0, time.Local)
	day := dateOnly(now)
	past := ticktick.Task{DueDate: "2026-08-30T10:00:00.000+0200"}
	future := ticktick.Task{DueDate: "2026-08-30T18:00:00.000+0200"}
	old := ticktick.Task{DueDate: "2026-08-29T10:00:00.000+0200"}
	if !calTaskShouldBeDone(past, day, now) {
		t.Fatal("past today should be done")
	}
	if calTaskShouldBeDone(future, day, now) {
		t.Fatal("future today should not be done")
	}
	if !calTaskShouldBeDone(old, day, now) {
		t.Fatal("old day should be done")
	}
}
