package mcp

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/tasktext"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// TaskView is the task shape returned to the model. Notes are plain text.
type TaskView struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes"`
	ProjectID string `json:"projectId,omitempty"`
	Project   string `json:"project,omitempty"`
	Status    string `json:"status"`
	Priority  string `json:"priority,omitempty"`
	Due       string `json:"due,omitempty"`
}

// TaskAPI is the TickTick surface the MCP tools use.
type TaskAPI interface {
	SearchTasks(query string, limit int) ([]TaskView, error)
	GetTask(id string) (TaskView, error)
	UpdateTaskText(id string, title, notes *string) (TaskView, error)
}

func toolDefs() []map[string]any {
	return []map[string]any{
		{
			"name":        "search_tasks",
			"description": "Search open tasks by title or notes. Use the returned id with get_task and update_task_text.",
			"inputSchema": objectSchema(map[string]any{
				"query": map[string]any{"type": "string", "description": "Case-insensitive text matched against the title and notes."},
				"limit": map[string]any{"type": "integer", "description": "Maximum results. Default 20, maximum 50."},
			}, "query"),
		},
		{
			"name":        "get_task",
			"description": "Read one task, including its plain-text notes.",
			"inputSchema": objectSchema(map[string]any{
				"id": map[string]any{"type": "string", "description": "Task id from search_tasks."},
			}, "id"),
		},
		{
			"name":        "update_task_text",
			"description": "Set a task's title, notes, or both. Requires a task id. Does not change the date, priority, checklist, or status. Notes are plain text.",
			"inputSchema": objectSchema(map[string]any{
				"id":    map[string]any{"type": "string", "description": "Task id from search_tasks or get_task."},
				"title": map[string]any{"type": "string", "description": "New title. Omit to leave the title unchanged."},
				"notes": map[string]any{"type": "string", "description": "New notes as plain text. Omit to leave the notes unchanged. An empty string clears them."},
			}, "id"),
		},
	}
}

func objectSchema(props map[string]any, required ...string) map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

func searchTool(api TaskAPI, raw json.RawMessage) (string, error) {
	var args struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.Query) == "" {
		return "", fmt.Errorf("query is required")
	}
	tasks, err := api.SearchTasks(args.Query, args.Limit)
	if err != nil {
		return "", err
	}
	return encodeJSON(map[string]any{"tasks": tasks, "count": len(tasks)})
}

func getTool(api TaskAPI, raw json.RawMessage) (string, error) {
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.ID) == "" {
		return "", fmt.Errorf("id is required")
	}
	task, err := api.GetTask(args.ID)
	if err != nil {
		return "", err
	}
	return encodeJSON(task)
}

func updateTool(api TaskAPI, raw json.RawMessage) (string, error) {
	var args struct {
		ID    string  `json:"id"`
		Title *string `json:"title"`
		Notes *string `json:"notes"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return "", err
	}
	if strings.TrimSpace(args.ID) == "" {
		return "", fmt.Errorf("id is required")
	}
	if args.Title == nil && args.Notes == nil {
		return "", fmt.Errorf("pass title, notes, or both")
	}
	if args.Title != nil && strings.TrimSpace(*args.Title) == "" {
		return "", fmt.Errorf("title cannot be empty")
	}
	task, err := api.UpdateTaskText(args.ID, args.Title, args.Notes)
	if err != nil {
		return "", err
	}
	return encodeJSON(task)
}

func encodeJSON(v any) (string, error) {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// TickTickAPI adapts the session client to TaskAPI.
type TickTickAPI struct {
	client *ticktick.Client
}

func NewTickTickAPI(client *ticktick.Client) *TickTickAPI {
	return &TickTickAPI{client: client}
}

func (a *TickTickAPI) SearchTasks(query string, limit int) ([]TaskView, error) {
	if a == nil || a.client == nil {
		return nil, fmt.Errorf("TickTick client unavailable")
	}
	limit = clampLimit(limit)
	tasks, err := a.client.AllOpenTasks()
	if err != nil {
		return nil, err
	}
	names := a.projectNames()
	needle := strings.ToLower(strings.TrimSpace(query))
	var titleHits, noteHits []TaskView
	for _, task := range tasks {
		view := taskToView(task, names)
		title := strings.ToLower(task.Title)
		notes := strings.ToLower(tasktext.Strip(task.Content))
		switch {
		case strings.Contains(title, needle):
			view.Notes = preview(view.Notes)
			titleHits = append(titleHits, view)
		case strings.Contains(notes, needle):
			view.Notes = preview(view.Notes)
			noteHits = append(noteHits, view)
		}
	}
	out := append(titleHits, noteHits...)
	if len(out) > limit {
		out = out[:limit]
	}
	if out == nil {
		out = []TaskView{}
	}
	return out, nil
}

func (a *TickTickAPI) GetTask(id string) (TaskView, error) {
	if a == nil || a.client == nil {
		return TaskView{}, fmt.Errorf("TickTick client unavailable")
	}
	raw, err := a.client.FindTaskByID(id)
	if err != nil {
		return TaskView{}, err
	}
	task, err := taskFromMap(raw)
	if err != nil {
		return TaskView{}, err
	}
	return taskToView(task, a.projectNames()), nil
}

func (a *TickTickAPI) UpdateTaskText(id string, title, notes *string) (TaskView, error) {
	if a == nil || a.client == nil {
		return TaskView{}, fmt.Errorf("TickTick client unavailable")
	}
	edit := taskTextEdit(title, notes)
	if err := a.client.EditTask(id, edit); err != nil {
		return TaskView{}, err
	}
	return a.GetTask(id)
}

func (a *TickTickAPI) projectNames() map[string]string {
	names := map[string]string{}
	if a == nil || a.client == nil {
		return names
	}
	projects, err := a.client.ListProjects()
	if err != nil {
		return names
	}
	for _, project := range projects {
		if project.ID != "" {
			names[project.ID] = project.Name
		}
	}
	return names
}

func taskToView(task ticktick.Task, names map[string]string) TaskView {
	view := TaskView{
		ID:        task.ID,
		Title:     task.Title,
		Notes:     tasktext.Strip(task.Content),
		ProjectID: task.ProjectID,
		Project:   names[task.ProjectID],
		Status:    statusLabel(task),
		Due:       task.DueDate,
	}
	if label := task.PriorityLabel(); label != "-" {
		view.Priority = label
	}
	if view.Project == "" && strings.HasPrefix(strings.ToLower(task.ProjectID), "inbox") {
		view.Project = "Inbox"
	}
	return view
}

func statusLabel(task ticktick.Task) string {
	switch {
	case task.Trashed():
		return "trash"
	case task.WontDo():
		return "won't do"
	case task.Done():
		return "completed"
	default:
		return "open"
	}
}

func taskFromMap(raw map[string]any) (ticktick.Task, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return ticktick.Task{}, err
	}
	var task ticktick.Task
	if err := json.Unmarshal(encoded, &task); err != nil {
		return ticktick.Task{}, err
	}
	return task, nil
}

func taskTextEdit(title, notes *string) ticktick.TaskEdit {
	edit := ticktick.TaskEdit{Title: title}
	if notes != nil {
		html := tasktext.ToHTML(*notes)
		edit.Content = &html
	}
	return edit
}

func preview(notes string) string {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return ""
	}
	if i := strings.IndexByte(notes, '\n'); i >= 0 {
		notes = notes[:i]
	}
	const maxRunes = 80
	runes := []rune(notes)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return notes
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 50 {
		return 50
	}
	return limit
}
