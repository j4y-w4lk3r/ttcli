package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestUpdateTaskClearsParentAndUnlinksTheChild(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const childID = "fedcba9876543210fedcba98"
	const parentID = "abcdef0123456789abcdef01"
	var parentCleared bool
	var keptParent any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload struct {
				Update []map[string]any `json:"update"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			keptParent = payload.Update[0]["parentId"]
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskParent":
			var items []map[string]string
			if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(items) != 1 || items[0]["taskId"] != childID || items[0]["projectId"] != projectID || items[0]["oldParentId"] != parentID {
				t.Errorf("parent clear body=%v", items)
			}
			parentCleared = true
			_, _ = w.Write([]byte(`{"id2error":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			parent := parentID
			if parentCleared {
				parent = ""
			}
			_, _ = w.Write([]byte(`[{"id":"` + childID + `","projectId":"` + projectID + `","title":"fm2","parentId":"` + parent + `","status":0}]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.UpdateTask(childID, projectID, TaskUpdateInput{
		Title: "fm2", ClearParent: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !parentCleared {
		t.Fatal("expected taskParent request")
	}
	if keptParent != parentID {
		t.Fatalf("task update changed parentId to %v", keptParent)
	}
}

func TestUpdateTaskClearsAParentThatOnlyListsTheChild(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const childID = "fedcba9876543210fedcba98"
	const parentID = "abcdef0123456789abcdef01"
	parentCleared := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskParent":
			var items []map[string]string
			if err := json.NewDecoder(r.Body).Decode(&items); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(items) != 1 || items[0]["oldParentId"] != parentID || items[0]["taskId"] != childID {
				t.Errorf("parent clear body=%v", items)
			}
			parentCleared = true
			_, _ = w.Write([]byte(`{"id2error":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/task/"+parentID:
			children := `["` + childID + `"]`
			if parentCleared {
				children = `[]`
			}
			_, _ = w.Write([]byte(`{"id":"` + parentID + `","title":"fm0","childIds":` + children + `}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/task/"+childID:
			_, _ = w.Write([]byte(`{"id":"` + childID + `","projectId":"` + projectID + `","title":"fm3","parentId":"","status":0}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.UpdateTask(childID, projectID, TaskUpdateInput{
		Title: "fm3", ClearParent: true, OldParentID: parentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !parentCleared {
		t.Fatal("expected taskParent request")
	}
}

func TestUpdateTaskRenamesACompletedTask(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "fedcba9876543210fedcba98"
	var updated map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/task/"+taskID:
			if r.URL.Query().Get("projectId") != projectID {
				http.Error(w, "project", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","projectId":"` + projectID + `","title":"Old title","status":2,"content":"kept","etag":"e1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload struct {
				Update []map[string]any `json:"update"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(payload.Update) != 1 {
				t.Errorf("updates=%d", len(payload.Update))
				return
			}
			updated = payload.Update[0]
			_, _ = w.Write([]byte(`{}`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.UpdateTask(taskID, projectID, TaskUpdateInput{
		Title: "New title", Content: "kept",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated["title"] != "New title" || updated["status"] != float64(2) || updated["content"] != "kept" || updated["etag"] != "e1" {
		t.Fatalf("update=%v", updated)
	}
}

func TestUpdateTaskRenamesAWontDoTaskByID(t *testing.T) {
	const projectID = "inboxfixture"
	const taskID = "0123456789abcdef01234567"
	var updated map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/task/"+taskID && r.URL.RawQuery == "":
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","projectId":"` + projectID + `","title":"Abandoned","status":-1,"etag":"e1"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload struct {
				Update []map[string]any `json:"update"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updated = payload.Update[0]
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "missing", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.UpdateTask(taskID, projectID, TaskUpdateInput{Title: "Reopened title"})
	if err != nil {
		t.Fatal(err)
	}
	if updated["title"] != "Reopened title" || updated["status"] != float64(-1) {
		t.Fatalf("update=%v", updated)
	}
}

func TestForgetStaleChildIDsKeepsTheRealParent(t *testing.T) {
	const (
		fm0     = "aaaaaaaaaaaaaaaaaaaaaaaa"
		finance = "bbbbbbbbbbbbbbbbbbbbbbbb"
		amazon  = "cccccccccccccccccccccccc"
		real    = "dddddddddddddddddddddddd"
		gone    = "eeeeeeeeeeeeeeeeeeeeeeee"
	)
	childIDs := []string{amazon, real, gone}
	parentOf := map[string]string{amazon: finance, real: fm0}
	projectOf := map[string]string{amazon: "fm-project", real: "fm-project"}
	var parentBody []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			http.Error(w, "task update cannot change childIds", http.StatusBadRequest)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskParent":
			if err := json.NewDecoder(r.Body).Decode(&parentBody); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			childIDs = []string{real, gone}
			_, _ = w.Write([]byte(`{"id2error":{}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/task/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v2/task/")
			if id == fm0 {
				body, _ := json.Marshal(map[string]any{"id": fm0, "title": "fm0", "childIds": childIDs})
				_, _ = w.Write(body)
				return
			}
			if pid, ok := parentOf[id]; ok {
				body, _ := json.Marshal(map[string]any{
					"id": id, "parentId": pid, "projectId": projectOf[id], "title": id, "status": 0,
				})
				_, _ = w.Write(body)
				return
			}
			http.NotFound(w, r)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	if err := client.ForgetStaleChildIDs(fm0, []string{amazon, real}); err != nil {
		t.Fatal(err)
	}
	if len(parentBody) != 1 || parentBody[0]["taskId"] != amazon || parentBody[0]["parentId"] != finance || parentBody[0]["oldParentId"] != fm0 {
		t.Fatalf("taskParent=%v", parentBody)
	}
	if strings.Join(childIDs, ",") != real+","+gone {
		t.Fatalf("childIds=%v", childIDs)
	}
	if parentOf[amazon] != finance || parentOf[real] != fm0 {
		t.Fatalf("parents=%v", parentOf)
	}
}

func TestMakeTasksNormalDetachesSeveralSubtasks(t *testing.T) {
	const (
		parent = "aaaaaaaaaaaaaaaaaaaaaaaa"
		swift  = "bbbbbbbbbbbbbbbbbbbbbbbb"
		iso    = "cccccccccccccccccccccccc"
		proj   = "dddddddddddddddddddddddd"
	)
	parentOf := map[string]string{swift: parent, iso: parent}
	childIDs := []string{swift, iso}
	var body []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskParent":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			parentOf[swift] = ""
			parentOf[iso] = ""
			childIDs = nil
			_, _ = w.Write([]byte(`{"id2error":{}}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/task/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v2/task/")
			switch id {
			case parent:
				payload, _ := json.Marshal(map[string]any{"id": parent, "title": "curiosity", "childIds": childIDs})
				_, _ = w.Write(payload)
			case swift, iso:
				payload, _ := json.Marshal(map[string]any{"id": id, "projectId": proj, "parentId": parentOf[id], "title": id, "status": 0})
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	n, err := client.MakeTasksNormal([]TaskParentLink{
		{TaskID: swift, OldParentID: parent},
		{TaskID: iso, OldParentID: parent},
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(body) != 2 {
		t.Fatalf("n=%d body=%v", n, body)
	}
	got := map[string]string{}
	for _, item := range body {
		got[item["taskId"]] = item["oldParentId"]
		if item["projectId"] != proj {
			t.Fatalf("project=%v", item)
		}
	}
	if got[swift] != parent || got[iso] != parent {
		t.Fatalf("body=%v", body)
	}
}

func TestDropMissingChildIDsClearsADeletedSubtask(t *testing.T) {
	const (
		parent = "aaaaaaaaaaaaaaaaaaaaaaaa"
		gone   = "eeeeeeeeeeeeeeeeeeeeeeee"
		proj   = "dddddddddddddddddddddddd"
	)
	const live = "ffffffffffffffffffffffff"
	childIDs := []string{gone, live}
	var body []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskParent":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			childIDs = []string{live}
			_, _ = fmt.Fprintf(w, `{"id2error":{"%s":"EXISTED"}}`, gone)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/v2/task/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/v2/task/")
			if id == parent {
				payload, _ := json.Marshal(map[string]any{
					"id": parent, "projectId": proj, "title": "curiosity", "childIds": childIDs,
				})
				_, _ = w.Write(payload)
				return
			}
			if id == live {
				payload, _ := json.Marshal(map[string]any{"id": live, "projectId": proj, "title": "still here"})
				_, _ = w.Write(payload)
				return
			}
			http.Error(w, `{"errorCode":"task_not_found","errorMessage":"task not exists"}`, http.StatusInternalServerError)
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	if err := client.DropMissingChildIDs(parent, []string{gone, live}); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0]["taskId"] != gone || body[0]["projectId"] != proj || body[0]["oldParentId"] != parent {
		t.Fatalf("body=%v", body)
	}
	n, err := client.MakeTasksNormal([]TaskParentLink{{TaskID: gone, OldParentID: parent}})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("n=%d", n)
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
