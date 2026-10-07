package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type fakeTasks struct {
	query string
	limit int
	gotID string
	title *string
	notes *string
	fail  error
}

func (f *fakeTasks) SearchTasks(query string, limit int) ([]TaskView, error) {
	f.query, f.limit = query, limit
	if f.fail != nil {
		return nil, f.fail
	}
	return []TaskView{{ID: "task123456789012345678", Title: "Oat milk", Notes: "the brand", Status: "open"}}, nil
}

func (f *fakeTasks) GetTask(id string) (TaskView, error) {
	f.gotID = id
	return TaskView{ID: id, Title: "Oat milk", Notes: "the brand\n1L", Status: "open"}, nil
}

func (f *fakeTasks) UpdateTaskText(id string, title, notes *string) (TaskView, error) {
	f.gotID, f.title, f.notes = id, title, notes
	view := TaskView{ID: id, Title: "Oat milk", Status: "open"}
	if title != nil {
		view.Title = *title
	}
	if notes != nil {
		view.Notes = *notes
	}
	return view, nil
}

func TestMCPInitializeListsToolsAndSkipsNotifications(t *testing.T) {
	api := &fakeTasks{}
	in := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"search_tasks","arguments":{"query":"oat","limit":5}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"update_task_text","arguments":{"id":"task123456789012345678","title":"Oat milk","notes":"Oatly\n1L"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"update_task_text","arguments":{"id":"task123456789012345678"}}}`,
	}, "\n") + "\n"
	var out strings.Builder
	if err := Run(context.Background(), api, strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 5 {
		t.Fatalf("responses=%d\n%s", len(lines), out.String())
	}
	var init struct {
		ID     int `json:"id"`
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &init); err != nil {
		t.Fatal(err)
	}
	if init.ID != 1 || init.Result.ProtocolVersion != "2025-03-26" || init.Result.ServerInfo.Name != "ttcli" {
		t.Fatalf("init=%+v", init.Result)
	}
	if !strings.Contains(lines[1], "search_tasks") || !strings.Contains(lines[1], "get_task") || !strings.Contains(lines[1], "update_task_text") {
		t.Fatalf("tools=%s", lines[1])
	}
	if api.query != "oat" || api.limit != 5 || !strings.Contains(lines[2], "Oat milk") {
		t.Fatalf("search query=%q limit=%d body=%s", api.query, api.limit, lines[2])
	}
	if api.title == nil || *api.title != "Oat milk" || api.notes == nil || *api.notes != "Oatly\n1L" {
		t.Fatalf("update title=%v notes=%v", api.title, api.notes)
	}
	var rejected struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[4]), &rejected); err != nil {
		t.Fatal(err)
	}
	if !rejected.Result.IsError || !strings.Contains(lines[4], "pass title") {
		t.Fatalf("rejected=%s", lines[4])
	}
}

func TestTaskTextEditStoresPlainNotesAsHTML(t *testing.T) {
	title := "Oat milk"
	notes := "Oatly\n1L"
	edit := taskTextEdit(&title, &notes)
	if edit.Title == nil || *edit.Title != title || edit.Content == nil || *edit.Content != "Oatly<br/>1L" {
		t.Fatalf("edit=%+v content=%v", edit, edit.Content)
	}
	cleared := ""
	edit = taskTextEdit(nil, &cleared)
	if edit.Title != nil || edit.Content == nil || *edit.Content != "" {
		t.Fatalf("clear edit=%+v", edit)
	}
}
