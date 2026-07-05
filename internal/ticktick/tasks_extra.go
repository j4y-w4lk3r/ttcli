package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Ping checks that the API accepts authenticated requests.
func (c *Client) Ping() error {
	_, err := c.ListProjects()
	return err
}

// CompletedTasks returns recently completed tasks account-wide.
func (c *Client) CompletedTasks() ([]Task, error) {
	var tasks []Task
	if err := c.getJSON("/api/v2/project/all/closed", &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// AllOpenTasks returns active tasks across all lists.
func (c *Client) AllOpenTasks() ([]Task, error) {
	raw, err := c.GetRaw("/api/v2/project/all/tasks")
	if err != nil {
		return nil, err
	}
	tasks := parseTasks(raw)
	out := tasks[:0]
	for _, t := range tasks {
		if t.Deleted == 0 && !t.Done() {
			out = append(out, t)
		}
	}
	return out, nil
}

// FindTask locates a task by exact title (case-insensitive) across all lists.
func (c *Client) FindTask(title string) (map[string]any, error) {
	raw, err := c.GetRaw("/api/v2/project/all/tasks")
	if err != nil {
		return nil, err
	}
	tasks, err := parseTasksRaw(raw)
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(strings.TrimSpace(title))
	var matches []map[string]any
	for _, t := range tasks {
		got, _ := t["title"].(string)
		if strings.EqualFold(got, title) || strings.ToLower(strings.TrimSpace(got)) == want {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		for _, t := range tasks {
			got, _ := t["title"].(string)
			if strings.Contains(strings.ToLower(got), want) {
				matches = append(matches, t)
			}
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no task matching %q", title)
	}
	if len(matches) > 1 {
		var titles []string
		for _, m := range matches {
			titles = append(titles, fmt.Sprintf("%q in %v", m["title"], m["projectId"]))
		}
		return nil, fmt.Errorf("multiple tasks match %q:\n  %s\n  pass a task id instead", title, strings.Join(titles, "\n  "))
	}
	return matches[0], nil
}

// FindTaskByID loads a task by id from the account-wide task list.
func (c *Client) FindTaskByID(id string) (map[string]any, error) {
	raw, err := c.GetRaw("/api/v2/project/all/tasks")
	if err != nil {
		return nil, err
	}
	tasks, err := parseTasksRaw(raw)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		if got, _ := t["id"].(string); got == id {
			return t, nil
		}
	}
	return nil, fmt.Errorf("no task with id %q", id)
}

func parseTasksRaw(raw []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr, nil
	}
	var obj struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj.Tasks, nil
}

// RescheduleTask sets due/start date and reminder to fire at due time.
func (c *Client) RescheduleTask(task map[string]any, due time.Time) error {
	id, _ := task["id"].(string)
	if id == "" {
		return fmt.Errorf("task has no id")
	}
	tz, _ := task["timeZone"].(string)
	if tz == "" {
		tz = localTZ()
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	due = time.Date(due.Year(), due.Month(), due.Day(), due.Hour(), due.Minute(), 0, 0, loc)
	formatted := due.Format(ticktickTimeLayout)

	task["startDate"] = formatted
	task["dueDate"] = formatted
	task["isAllDay"] = false
	task["isFloating"] = false
	task["timeZone"] = tz
	task["reminder"] = "TRIGGER:PT0S"
	if rems, ok := task["reminders"].([]any); ok && len(rems) > 0 {
		if m, ok := rems[0].(map[string]any); ok {
			m["trigger"] = "TRIGGER:PT0S"
		}
	} else {
		task["reminders"] = []any{map[string]any{"trigger": "TRIGGER:PT0S"}}
	}
	task["modifiedTime"] = time.Now().In(loc).Format(ticktickTimeLayout)

	payload := map[string]any{
		"add": []any{}, "update": []any{task}, "delete": []any{},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/batch/task", b)
	return err
}
