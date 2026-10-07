package ticktick

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSnapshotProjectRefusesInbox(t *testing.T) {
	client := &Client{}
	_, err := client.SnapshotProject("inboxfixture")
	if err == nil || !strings.Contains(err.Error(), "inbox") {
		t.Fatalf("err=%v", err)
	}
}

func TestSnapshotProjectMergesCompletedHistory(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const openID = "aaaaaaaaaaaaaaaaaaaaaaaa"
	const doneID = "bbbbbbbbbbbbbbbbbbbbbbbb"
	const wontID = "dddddddddddddddddddddddd"
	const trashID = "eeeeeeeeeeeeeeeeeeeeeeee"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/projects":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": projectID, "name": "Zero", "color": "#f38ba8", "kind": "TASK", "groupId": "NONE",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+projectID+"/tasks":
			_ = json.NewEncoder(w).Encode([]map[string]any{{
				"id": openID, "projectId": projectID, "title": "Open", "status": 0, "content": "still open",
			}})
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/completed/"):
			_ = json.NewEncoder(w).Encode([]Task{{
				ID: doneID, ProjectID: projectID, Title: "Finished", Status: 2, Content: "done notes",
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/all/closed" && r.URL.Query().Get("status") == "Abandoned":
			_ = json.NewEncoder(w).Encode([]Task{{
				ID: wontID, ProjectID: projectID, Title: "Nope", Status: -1,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/all/trash/pagination":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tasks": []Task{{ID: trashID, ProjectID: projectID, Title: "Gone", Deleted: 1}},
			})
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	snap, err := repositoryTestClient(server).SnapshotProject(projectID)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Project["name"] != "Zero" {
		t.Fatalf("project=%v", snap.Project)
	}
	ids := map[string]float64{}
	for _, raw := range snap.Tasks {
		var task map[string]any
		if err := json.Unmarshal(raw, &task); err != nil {
			t.Fatal(err)
		}
		id, _ := task["id"].(string)
		status, _ := task["status"].(float64)
		ids[id] = status
	}
	_, haveTrash := ids[trashID]
	if ids[openID] != 0 || ids[doneID] != 2 || ids[wontID] != -1 || !haveTrash || len(ids) != 4 {
		t.Fatalf("tasks=%v", ids)
	}
}

func TestRestoreProjectSnapshotKeepsClosedStatus(t *testing.T) {
	const parentID = "aaaaaaaaaaaaaaaaaaaaaaaa"
	const childID = "bbbbbbbbbbbbbbbbbbbbbbbb"
	const newProjectID = "cccccccccccccccccccccccc"
	var created map[string]any
	var added []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/project":
			var payload struct {
				Add []map[string]any `json:"add"`
			}
			if err := json.Unmarshal(body, &payload); err != nil || len(payload.Add) != 1 {
				http.Error(w, "bad project batch", http.StatusBadRequest)
				return
			}
			created = payload.Add[0]
			_, _ = w.Write([]byte(`{"id2etag":{"` + newProjectID + `":"etag"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/task":
			var payload struct {
				Add []map[string]any `json:"add"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			added = append(added, payload.Add...)
			_, _ = w.Write([]byte(`{}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()

	snap := ProjectSnapshot{
		Project: map[string]any{
			"name": "Zero", "color": "#f38ba8", "kind": "TASK", "groupId": "NONE",
		},
		Tasks: []json.RawMessage{
			json.RawMessage(`{"id":"` + parentID + `","title":"Parent","status":2,"content":"done notes","etag":"old","childIds":["` + childID + `"],"items":[{"id":"item-old","title":"step","status":0}]}`),
			json.RawMessage(`{"id":"` + childID + `","title":"Child","status":-1,"parentId":"` + parentID + `","content":"abandoned notes","etag":"old2","projectId":"old-project"}`),
		},
	}
	id, n, err := repositoryTestClient(server).RestoreProjectSnapshot(snap)
	if err != nil {
		t.Fatal(err)
	}
	if id != newProjectID || n != 2 {
		t.Fatalf("id=%s n=%d", id, n)
	}
	if created["name"] != "Zero" || created["color"] != "#f38ba8" || created["groupId"] != "NONE" {
		t.Fatalf("created=%v", created)
	}
	if len(added) != 2 {
		t.Fatalf("added=%v", added)
	}
	parent, child := added[0], added[1]
	if parent["status"] != float64(2) || child["status"] != float64(-1) {
		t.Fatalf("statuses parent=%v child=%v", parent["status"], child["status"])
	}
	if parent["projectId"] != newProjectID || child["projectId"] != newProjectID {
		t.Fatalf("project ids parent=%v child=%v", parent["projectId"], child["projectId"])
	}
	if parent["content"] != "done notes" || child["content"] != "abandoned notes" {
		t.Fatalf("notes parent=%v child=%v", parent["content"], child["content"])
	}
	if _, ok := parent["etag"]; ok {
		t.Fatalf("etag kept on %+v", parent)
	}
	if child["parentId"] != parent["id"] || child["parentId"] == parentID {
		t.Fatalf("parent link parent=%v child=%v", parent["id"], child["parentId"])
	}
	childIDs, _ := parent["childIds"].([]any)
	if len(childIDs) != 1 || childIDs[0] != child["id"] {
		t.Fatalf("childIds=%v want %v", childIDs, child["id"])
	}
	items, _ := parent["items"].([]any)
	item, _ := items[0].(map[string]any)
	if item["id"] == "item-old" || item["id"] == "" {
		t.Fatalf("item=%v", item)
	}
}
