package ticktick

import (
	"testing"
	"time"
)

func TestTaskReminderAtFiresBeforeATimedDue(t *testing.T) {
	due := time.Date(2026, 10, 7, 15, 0, 0, 0, time.Local)
	task := Task{
		ID:       "task",
		DueDate:  due.Format("2006-01-02T15:04:05.000-0700"),
		Reminder: "TRIGGER:-PT15M",
	}
	at, ok := TaskReminderAt(task)
	if !ok {
		t.Fatal("expected a reminder time")
	}
	want := due.Add(-15 * time.Minute)
	if !at.Equal(want) {
		t.Fatalf("at=%s want %s", at, want)
	}
}

func TestTaskReminderAtUsesNineForAllDay(t *testing.T) {
	task := Task{
		ID: "day", IsAllDay: true, DueDate: "2026-10-07", Reminder: "TRIGGER:PT0S",
	}
	at, ok := TaskReminderAt(task)
	want := time.Date(2026, 10, 7, 9, 0, 0, 0, time.Local)
	if !ok || !at.Equal(want) {
		t.Fatalf("at=%s ok=%v want %s", at, ok, want)
	}
}

func TestTaskReminderAtReadsTheRemindersArray(t *testing.T) {
	due := time.Date(2026, 10, 7, 18, 30, 0, 0, time.Local)
	task := Task{
		ID:      "array",
		DueDate: due.Format("2006-01-02T15:04:05.000-0700"),
		Reminders: []ReminderTrigger{
			{Trigger: "TRIGGER:-PT5M"},
		},
	}
	at, ok := TaskReminderAt(task)
	if !ok || !at.Equal(due.Add(-5*time.Minute)) {
		t.Fatalf("at=%s ok=%v", at, ok)
	}
}

func TestDueRemindersSkipsFutureDoneAndAlreadySent(t *testing.T) {
	due := time.Date(2026, 10, 7, 15, 0, 0, 0, time.Local)
	now := due
	fresh := Task{ID: "fresh", Title: "Fresh", DueDate: due.Format("2006-01-02T15:04:05.000-0700"), Reminder: "TRIGGER:PT0S"}
	late := Task{
		ID: "late", DueDate: due.Add(-10 * time.Minute).Format("2006-01-02T15:04:05.000-0700"),
		Reminder: "TRIGGER:PT0S",
	}
	done := fresh
	done.ID = "done"
	done.Status = 2
	future := fresh
	future.ID = "future"
	future.DueDate = due.Add(time.Hour).Format("2006-01-02T15:04:05.000-0700")
	sentAt, _ := TaskReminderAt(fresh)
	sent := map[string]struct{}{ReminderKey(fresh.ID, sentAt): {}}

	hits := DueReminders([]Task{fresh, late, done, future}, now, sent, ReminderGrace)
	if len(hits) != 0 {
		t.Fatalf("hits=%+v", hits)
	}

	hits = DueReminders([]Task{fresh}, now, nil, ReminderGrace)
	if len(hits) != 1 || hits[0].Task.ID != "fresh" || hits[0].Key == "" {
		t.Fatalf("hits=%+v", hits)
	}
}
