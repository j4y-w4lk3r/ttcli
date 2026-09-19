package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskTrashRestoreSnapshotDeleteAndRecreate(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	var updates []map[string]any
	var deletes []any
	var recreated map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": taskID, "projectId": projectID, "title": "Food",
				"content": "notes", "tags": []string{"home"},
				"repeatFlag": "RRULE:FREQ=DAILY", "serverOnly": "keep-me",
				"deleted": 1, "status": 0,
			}})
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
			if update, _ := payload["update"].([]any); len(update) > 0 {
				updates = append(updates, update[0].(map[string]any))
			}
			deletes, _ = payload["delete"].([]any)
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
	if updates[len(updates)-1]["deleted"] != float64(1) && updates[len(updates)-1]["deleted"] != 1 {
		t.Fatalf("trash update=%+v", updates[len(updates)-1])
	}
	if err := client.RestoreTasks(projectID, []string{taskID}); err != nil {
		t.Fatal(err)
	}
	if updates[len(updates)-1]["deleted"] != float64(0) {
		t.Fatalf("restore update=%+v", updates[len(updates)-1])
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
	if len(deletes) != 1 {
		t.Fatalf("delete payload=%+v", deletes)
	}
}
