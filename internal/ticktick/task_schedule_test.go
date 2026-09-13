package ticktick

import (
	"testing"
	"time"
)

func TestApplyScheduleAllDayUsesDateOnly(t *testing.T) {
	task := map[string]any{"id": "t1", "timeZone": "Europe/Warsaw"}
	loc, _ := time.LoadLocation("Europe/Warsaw")
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	if err := applyScheduleToMap(task, TaskSchedule{
		Due:     due,
		HasDue:  true,
		AllDay:  true,
	}); err != nil {
		t.Fatal(err)
	}
	if task["startDate"] != "2026-11-01" || task["dueDate"] != "2026-11-01" {
		t.Fatalf("dates=%v %v", task["startDate"], task["dueDate"])
	}
	if task["isAllDay"] != true {
		t.Fatal("expected all-day")
	}
}

func TestParseReminderBefore(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
		ok   bool
	}{
		{"", 0, false},
		{"TRIGGER:PT0S", 0, true},
		{"TRIGGER:-PT15M", 15 * time.Minute, true},
		{"TRIGGER:-PT1H", time.Hour, true},
		{"TRIGGER:-PT1H30M", 90 * time.Minute, true},
	}
	for _, tc := range cases {
		got, ok := ParseReminderBefore(tc.raw)
		if ok != tc.ok || (ok && got != tc.want) {
			t.Fatalf("ParseReminderBefore(%q) = (%v, %v) want (%v, %v)", tc.raw, got, ok, tc.want, tc.ok)
		}
	}
}
