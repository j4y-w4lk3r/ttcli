package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMoveTaskUsesTaskProjectAndKeepsTheID(t *testing.T) {
	const fromID = "0123456789abcdef01234567"
	const toID = "abcdef0123456789abcdef01"
	const taskID = "fedcba9876543210fedcba98"
	var moved []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskProject":
			if err := json.NewDecoder(r.Body).Decode(&moved); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id2etag":{},"id2error":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+fromID+"/tasks":
			if len(moved) > 0 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"` + taskID + `","projectId":"` + fromID + `","title":"Move me","status":0}]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+toID+"/tasks":
			if len(moved) == 0 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[{"id":"` + taskID + `","projectId":"` + toID + `","title":"Move me","status":0}]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)

	res, err := client.MoveTask(taskID, fromID, toID)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewTaskID != taskID || res.PreviousID != taskID {
		t.Fatalf("result=%+v", res)
	}
	if len(moved) != 1 || moved[0]["taskId"] != taskID || moved[0]["fromProjectId"] != fromID || moved[0]["toProjectId"] != toID {
		t.Fatalf("move=%+v", moved)
	}
}

func TestMoveTasksPostsOneBatch(t *testing.T) {
	const fromID = "0123456789abcdef01234567"
	const toID = "abcdef0123456789abcdef01"
	const first = "111111111111111111111111"
	const second = "222222222222222222222222"
	posts := 0
	var moved []map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskProject":
			posts++
			if err := json.NewDecoder(r.Body).Decode(&moved); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"id2etag":{},"id2error":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+fromID+"/tasks":
			if posts > 0 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[
				{"id":"` + first + `","projectId":"` + fromID + `","status":0},
				{"id":"` + second + `","projectId":"` + fromID + `","status":0}
			]`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+toID+"/tasks":
			if posts == 0 {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(`[
				{"id":"` + first + `","projectId":"` + toID + `","status":0},
				{"id":"` + second + `","projectId":"` + toID + `","status":0}
			]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	n, err := client.MoveTasks([]string{first, second}, fromID, toID)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || posts != 1 || len(moved) != 2 {
		t.Fatalf("n=%d posts=%d moved=%v", n, posts, moved)
	}
}

func TestMoveTaskRejectsASilentNoOp(t *testing.T) {
	const fromID = "0123456789abcdef01234567"
	const toID = "abcdef0123456789abcdef01"
	const taskID = "fedcba9876543210fedcba98"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/batch/taskProject":
			_, _ = w.Write([]byte(`{"id2etag":{},"id2error":{}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v2/project/"+fromID+"/tasks":
			_, _ = w.Write([]byte(`[{"id":"` + taskID + `","projectId":"` + fromID + `","status":0}]`))
		case r.Method == http.MethodGet:
			_, _ = w.Write([]byte(`[]`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer server.Close()
	client := repositoryTestClient(server)
	_, err := client.MoveTask(taskID, fromID, toID)
	if err == nil || !strings.Contains(err.Error(), taskID) {
		t.Fatal(err)
	}
}
