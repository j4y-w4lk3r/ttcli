package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestApplyScheduleAllDayUsesDateOnly(t *testing.T) {
	task := map[string]any{"id": "t1", "timeZone": "Europe/Warsaw"}
	loc, _ := time.LoadLocation("Europe/Warsaw")
	due := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	if err := applyScheduleToMap(task, TaskSchedule{
		Due:    due,
		HasDue: true,
		AllDay: true,
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

func TestApplyScheduleUsesStartTimeAndDuration(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	task := map[string]any{"id": "t1", "timeZone": "Europe/Warsaw"}
	loc, _ := time.LoadLocation("Europe/Warsaw")
	start := time.Date(2026, 9, 17, 13, 0, 0, 0, loc)
	if err := applyScheduleToMap(task, TaskSchedule{
		Start: start, HasDue: true, Duration: 75 * time.Minute,
	}); err != nil {
		t.Fatal(err)
	}
	if task["startDate"] != "2026-09-17T13:00:00.000+0200" {
		t.Fatalf("startDate=%v", task["startDate"])
	}
	if task["dueDate"] != "2026-09-17T14:15:00.000+0200" {
		t.Fatalf("dueDate=%v", task["dueDate"])
	}
}

func TestApplySchedulePreservesEnteredWallTimeAcrossOldTimezone(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	loc, _ := time.LoadLocation("Europe/Warsaw")
	task := map[string]any{"id": "t1", "timeZone": "UTC"}
	if err := applyScheduleToMap(task, TaskSchedule{
		Start:  time.Date(2026, 9, 18, 14, 0, 0, 0, loc),
		HasDue: true,
	}); err != nil {
		t.Fatal(err)
	}
	if task["startDate"] != "2026-09-18T14:00:00.000+0200" ||
		task["dueDate"] != "2026-09-18T14:00:00.000+0200" {
		t.Fatalf("schedule shifted: start=%v due=%v", task["startDate"], task["dueDate"])
	}
	if task["timeZone"] != "Europe/Warsaw" {
		t.Fatalf("timeZone=%v", task["timeZone"])
	}
}

func TestApplyRecurrenceAndNativeFocusPlan(t *testing.T) {
	task := map[string]any{
		"id": "t1",
		"focusSummaries": []any{map[string]any{
			"pomoCount": 2,
		}},
	}
	recurrence, err := NewTaskRecurrence(RecurrencePresetRule("weekdays", time.Now()), RepeatFromDue)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyRecurrenceToMap(task, recurrence, false); err != nil {
		t.Fatal(err)
	}
	if err := applyFocusPlanToMap(task, &TaskFocusPlan{Minutes: 75}); err != nil {
		t.Fatal(err)
	}
	if task["repeatFlag"] != "RRULE:FREQ=WEEKLY;INTERVAL=1;WKST=MO;BYDAY=MO,TU,WE,TH,FR" {
		t.Fatalf("repeatFlag=%v", task["repeatFlag"])
	}
	summaries := task["focusSummaries"].([]any)
	summary := summaries[0].(map[string]any)
	if summary["estimatedDuration"] != 4500 || summary["estimatedPomo"] != 3 || summary["pomoCount"] != 2 {
		t.Fatalf("focus summary=%+v", summary)
	}
}

func TestCreateTaskWritesScheduleRecurrenceAndNativeFocusTogether(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	const projectID = "0123456789abcdef01234567"
	var createdID string
	var update map[string]any
	postCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			postCount++
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if postCount == 1 {
				add := payload["add"].([]any)
				createdID = add[0].(map[string]any)["id"].(string)
			} else {
				update = payload["update"].([]any)[0].(map[string]any)
			}
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/all/tasks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": createdID, "projectId": projectID, "title": "Food",
				"timeZone": "UTC", "serverOnly": "preserve-me",
				"focusSummaries": []any{map[string]any{"pomoCount": 2}},
			}})
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	start := time.Date(2026, 9, 17, 13, 0, 0, 0, time.UTC)
	recurrence, err := NewTaskRecurrence(
		"RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=TU,TH",
		RepeatFromDue,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.CreateTask(TaskCreateInput{
		ProjectID: projectID, Title: "Food",
		Schedule:   &TaskSchedule{Start: start, HasDue: true, Duration: time.Hour},
		Recurrence: recurrence,
		FocusPlan:  &TaskFocusPlan{Minutes: 75, Pomos: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if update["startDate"] != "2026-09-17T13:00:00.000+0200" ||
		update["dueDate"] != "2026-09-17T14:00:00.000+0200" {
		t.Fatalf("schedule=%v → %v", update["startDate"], update["dueDate"])
	}
	if update["repeatFlag"] != recurrence.Rule || update["repeatFrom"] != float64(RepeatFromDue) {
		t.Fatalf("recurrence=%v repeatFrom=%v", update["repeatFlag"], update["repeatFrom"])
	}
	summary := update["focusSummaries"].([]any)[0].(map[string]any)
	if summary["estimatedDuration"] != float64(4500) || summary["estimatedPomo"] != float64(3) ||
		summary["pomoCount"] != float64(2) {
		t.Fatalf("focus summary=%+v", summary)
	}
	if update["serverOnly"] != "preserve-me" {
		t.Fatalf("unknown raw field was not preserved: %+v", update)
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
