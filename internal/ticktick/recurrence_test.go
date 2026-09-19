package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestTaskParsesRecurrenceAndSeriesID(t *testing.T) {
	var task Task
	err := json.Unmarshal([]byte(`{
		"id":"occurrence-id",
		"repeatFlag":"RRULE:FREQ=WEEKLY;INTERVAL=1",
		"repeatFrom":0,
		"repeatFirstDate":"2026-09-17T08:00:00.000+0000",
		"repeatTaskId":"series-id"
	}`), &task)
	if err != nil {
		t.Fatal(err)
	}
	if !task.Repeating() {
		t.Fatal("expected recurring task")
	}
	if task.SeriesID() != "series-id" {
		t.Fatalf("series id=%q", task.SeriesID())
	}
}

func TestTaskParsesNativeFocusEstimate(t *testing.T) {
	var task Task
	err := json.Unmarshal([]byte(`{
		"id":"task",
		"focusSummaries":[{
			"estimatedDuration":4500,
			"estimatedPomo":3,
			"pomoCount":1,
			"pomoDuration":1500
		}]
	}`), &task)
	if err != nil {
		t.Fatal(err)
	}
	seconds, pomos, ok := task.FocusEstimate()
	if !ok || seconds != 4500 || pomos != 3 {
		t.Fatalf("seconds=%d pomos=%d ok=%v", seconds, pomos, ok)
	}
}

func TestRecurrenceRulesAndPresets(t *testing.T) {
	anchor := time.Date(2026, 9, 17, 13, 0, 0, 0, time.Local)
	weekdays := RecurrencePresetRule("weekdays", anchor)
	if RecurrencePreset(weekdays) != "weekdays" {
		t.Fatalf("preset=%q rule=%q", RecurrencePreset(weekdays), weekdays)
	}
	weekly := RecurrencePresetRule("weekly", anchor)
	if !strings.Contains(weekly, "BYDAY=TH") || RecurrencePreset(weekly) != "weekly" {
		t.Fatalf("weekly=%q", weekly)
	}
	if _, err := NormalizeRecurrenceRule("RRULE:FREQ=WEEKLY;BYDAY=NO"); err == nil {
		t.Fatal("expected invalid BYDAY")
	}
	if got, err := NormalizeRecurrenceRule("erule:name=custom;bydate=20260917,20260921"); err != nil ||
		got != "ERULE:NAME=CUSTOM;BYDATE=20260917,20260921" {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestExpandTaskOccurrencesWeekdaysPreservesBlock(t *testing.T) {
	loc := time.FixedZone("CEST", 2*60*60)
	task := Task{
		ID: "food", DueDate: "2026-09-17T14:00:00.000+0200",
		StartDate:       "2026-09-17T13:00:00.000+0200",
		RepeatFirstDate: "2026-09-14T14:00:00.000+0200",
		RepeatFlag:      "RRULE:FREQ=WEEKLY;INTERVAL=1;WKST=MO;BYDAY=MO,TU,WE,TH,FR",
	}
	got, supported := ExpandTaskOccurrences(
		task,
		time.Date(2026, 9, 17, 0, 0, 0, 0, loc),
		time.Date(2026, 9, 21, 0, 0, 0, 0, loc),
	)
	if !supported || len(got) != 3 {
		t.Fatalf("supported=%v occurrences=%+v", supported, got)
	}
	if got[0].StartDate != "2026-09-17T13:00:00.000+0200" ||
		got[2].DueDate != "2026-09-21T14:00:00.000+0200" {
		t.Fatalf("occurrences=%+v", got)
	}
}

func TestExpandTaskOccurrencesERuleAndUnsupportedFallback(t *testing.T) {
	loc := time.UTC
	task := Task{
		ID: "custom", DueDate: "2026-09-17",
		RepeatFlag: "ERULE:NAME=CUSTOM;BYDATE=20260917,20260920",
		IsAllDay:   true,
	}
	got, supported := ExpandTaskOccurrences(
		task,
		time.Date(2026, 9, 17, 0, 0, 0, 0, loc),
		time.Date(2026, 9, 21, 0, 0, 0, 0, loc),
	)
	if !supported || len(got) != 2 || got[1].DueDate != "2026-09-20" {
		t.Fatalf("supported=%v occurrences=%+v", supported, got)
	}
	task.RepeatFlag = "ERULE:NAME=CHINESE_LUNAR;INTERVAL=1"
	got, supported = ExpandTaskOccurrences(
		task,
		time.Date(2026, 9, 17, 0, 0, 0, 0, loc),
		time.Date(2026, 9, 21, 0, 0, 0, 0, loc),
	)
	if supported || len(got) != 1 || got[0].DueDate != task.DueDate {
		t.Fatalf("unsupported fallback=%+v supported=%v", got, supported)
	}
}

func TestCompleteTaskOccurrencePreservesRecurrenceFields(t *testing.T) {
	const projectID = "507f1f77bcf86cd799439011"
	const taskID = "507f191e810c19729de860ea"
	var update map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_, _ = w.Write([]byte(`[{
				"id":"` + taskID + `",
				"projectId":"` + projectID + `",
				"title":"Bins",
				"repeatFlag":"RRULE:FREQ=WEEKLY;INTERVAL=1",
				"repeatTaskId":"series-id",
				"status":0
			}]`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload struct {
				Update []map[string]any `json:"update"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(payload.Update) != 1 {
				http.Error(w, "expected one update", http.StatusBadRequest)
				return
			}
			update = payload.Update[0]
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := repositoryTestClient(server)
	if err := client.CompleteTaskOccurrence(projectID, taskID); err != nil {
		t.Fatal(err)
	}
	if update["repeatFlag"] != "RRULE:FREQ=WEEKLY;INTERVAL=1" {
		t.Fatalf("repeatFlag lost: %+v", update)
	}
	if update["repeatTaskId"] != "series-id" {
		t.Fatalf("repeatTaskId lost: %+v", update)
	}
	if status, ok := update["status"].(float64); !ok || status != 2 {
		t.Fatalf("status=%v", update["status"])
	}
	if completed, _ := update["completedTime"].(string); !strings.Contains(completed, "T") {
		t.Fatalf("completedTime=%q", completed)
	}
}

func TestTasksCompletedOnUsesDateRangeQuery(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	var from, to, limit, status string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from = r.URL.Query().Get("from")
		to = r.URL.Query().Get("to")
		limit = r.URL.Query().Get("limit")
		status = r.URL.Query().Get("status")
		completed := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local).
			UTC().Format(ticktickTimeLayout)
		_ = json.NewEncoder(w).Encode([]Task{{ID: "done", CompletedT: completed}})
	}))
	defer server.Close()

	client := repositoryTestClient(server)
	tasks, err := client.TasksCompletedOn(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks=%+v", tasks)
	}
	if from == "" || to == "" || limit != "500" || status != "Completed" {
		t.Fatalf("query from=%q to=%q limit=%q status=%q", from, to, limit, status)
	}
}

func TestTasksCompletedOnFallsBackWhenRangeQueryFails(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	rangeRequests, fallbackRequests := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") != "" {
			rangeRequests++
			http.Error(w, `{"errorCode":"unknown_exception"}`, http.StatusInternalServerError)
			return
		}
		fallbackRequests++
		completed := day.Add(12 * time.Hour).UTC().Format(ticktickTimeLayout)
		_ = json.NewEncoder(w).Encode([]Task{{ID: "done", CompletedT: completed}})
	}))
	defer server.Close()

	client := repositoryTestClient(server)
	tasks, err := client.TasksCompletedOn(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 || rangeRequests != 1 || fallbackRequests != 1 {
		t.Fatalf("tasks=%+v range=%d fallback=%d", tasks, rangeRequests, fallbackRequests)
	}
}
