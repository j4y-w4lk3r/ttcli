package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskTrashRestoreSnapshotDeleteAndRecreate(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	var deletes []any
	var recreated map[string]any
	var restored []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": taskID, "projectId": projectID, "title": "Food",
				"content": "notes", "tags": []string{"home"},
				"repeatFlag": "RRULE:FREQ=DAILY", "serverOnly": "keep-me",
				"deleted": 1, "status": 0,
			}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/trash/restore":
			if err := json.NewDecoder(r.Body).Decode(&restored); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id2etag":{"` + taskID + `":"etag"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if add, _ := payload["add"].([]any); len(add) > 0 {
				recreated = add[0].(map[string]any)
				_, _ = w.Write([]byte(`{"id2etag":{"server-new":"etag"}}`))
				return
			}
			if del, _ := payload["delete"].([]any); len(del) > 0 {
				deletes = append(deletes, del...)
			}
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)

	if err := client.TrashTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if len(deletes) != 1 {
		t.Fatalf("trash delete=%+v", deletes)
	}
	if err := client.RestoreTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0]["taskId"] != taskID || restored[0]["fromProjectId"] != projectID || restored[0]["toProjectId"] != projectID {
		t.Fatalf("restore=%+v", restored)
	}

	raw, err := client.TaskSnapshot(projectID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["serverOnly"] != "keep-me" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
	newID, err := client.RecreateTaskSnapshot(raw, projectID)
	if err != nil {
		t.Fatal(err)
	}
	if newID != "server-new" || recreated["serverOnly"] != "keep-me" ||
		recreated["deleted"] != float64(0) || recreated["status"] != float64(0) {
		t.Fatalf("newID=%q recreated=%+v", newID, recreated)
	}
	if _, found := recreated["repeatTaskId"]; found {
		t.Fatalf("recreated task retained old series identity: %+v", recreated)
	}
	if err := client.DeleteTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if len(deletes) != 2 {
		t.Fatalf("delete payload=%+v", deletes)
	}
}

func TestRestoreUsesTrashRestoreEndpoint(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	var restored []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && r.URL.Path == "/api/v2/trash/restore" {
			if err := json.NewDecoder(r.Body).Decode(&restored); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id2etag":{"` + taskID + `":"etag"}}`))
			return
		}
		http.Error(w, "unexpected request "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}))
	defer server.Close()
	client := repositoryTestClient(server)

	if err := client.RestoreTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if len(restored) != 1 || restored[0]["taskId"] != taskID || restored[0]["fromProjectId"] != projectID || restored[0]["toProjectId"] != projectID {
		t.Fatalf("restore=%+v", restored)
	}
}

func TestTrashRejectsATaskThatStaysLive(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": taskID, "projectId": projectID, "deleted": 0,
			}})
			return
		}
		_, _ = w.Write([]byte(`{"id2etag":{},"id2error":{}}`))
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.TrashTasks(projectID, []string{taskID})
	if err == nil || !strings.Contains(err.Error(), taskID) {
		t.Fatal(err)
	}
}

func TestRestoreRejectsASilentEmptyResponse(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id2etag":{},"id2error":{}}`))
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	if err := client.RestoreTasks(projectID, []string{taskID}); err == nil {
		t.Fatal("empty restore envelope should be an error")
	}
}

func TestTaskSnapshotReadsATrashedTask(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_, _ = w.Write([]byte(`[]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/task/"+taskID:
			if r.URL.Query().Get("projectId") != projectID {
				http.Error(w, "project", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id":"` + taskID + `","projectId":"` + projectID + `","title":"Gone","deleted":1,"serverOnly":"keep-me"}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	raw, err := client.TaskSnapshot(projectID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot map[string]any
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot["serverOnly"] != "keep-me" || snapshot["deleted"] != float64(1) {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestPurgeTrashTasksUsesDeleteForever(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	var body []map[string]string
	purged := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v2/task" && r.URL.Query().Get("deleteforever") == "true":
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			purged = true
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/all/trash/pagination":
			if purged {
				_, _ = w.Write([]byte(`{"tasks":[],"nextStart":-1}`))
				return
			}
			_, _ = w.Write([]byte(`{"tasks":[{"id":"` + taskID + `","projectId":"` + projectID + `","deleted":1}],"nextStart":-1}`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	if err := client.PurgeTrashTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0]["taskId"] != taskID || body[0]["projectId"] != projectID {
		t.Fatalf("body=%+v", body)
	}
}

func TestPurgeTrashTasksRejectsATaskThatStaysInTrash(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"tasks":[{"id":"` + taskID + `","deleted":1}],"nextStart":-1}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	err := client.PurgeTrashTasks(projectID, []string{taskID})
	if err == nil || !strings.Contains(err.Error(), taskID) {
		t.Fatal(err)
	}
}

func TestAbandonTasksSetsAbandonedStatus(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	var status any
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
			status = payload.Update[0]["status"]
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_, _ = w.Write([]byte(`[{"id":"` + taskID + `","projectId":"` + projectID + `","title":"Later","status":0}]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	if err := client.AbandonTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if status != float64(-1) {
		t.Fatalf("status=%v", status)
	}
}
