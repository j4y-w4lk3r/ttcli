package ticktick

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
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
	if err := c.getJSON("/api/v2/project/all/closed?status=Completed", &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// CompletedTasksInRange asks TickTick to limit closed tasks to a time range.
// The local filter in TasksCompletedOn remains authoritative if the private
// endpoint returns a wider window.
func (c *Client) CompletedTasksInRange(start, end time.Time) ([]Task, error) {
	values := url.Values{}
	values.Set("from", start.UTC().Format(ticktickTimeLayout))
	values.Set("to", end.UTC().Format(ticktickTimeLayout))
	values.Set("limit", strconv.Itoa(500))
	// TickTick now requires the closed-task status discriminator. Omitting it
	// produces an opaque HTTP 500 from the private endpoint.
	values.Set("status", "Completed")
	var tasks []Task
	if err := c.getJSON("/api/v2/project/all/closed?"+values.Encode(), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// TasksCompletedBetween returns tasks whose completion timestamp falls within
// the inclusive local date range.
func (c *Client) TasksCompletedBetween(startDay, endDay time.Time) ([]Task, error) {
	start, _ := LocalDayBounds(startDay)
	_, end := LocalDayBounds(endDay)
	tasks, err := c.CompletedTasksInRange(start, end)
	if err != nil {
		// The private API has changed its accepted range parameters before.
		// Fall back to the unbounded completed feed and retain the authoritative
		// local date filter below instead of breaking Calendar.
		tasks, err = c.CompletedTasks()
		if err != nil {
			return nil, err
		}
	}
	out := tasks[:0]
	for _, task := range tasks {
		if task.CompletedT == "" {
			continue
		}
		completedAt, err := ParseAPITime(task.CompletedT)
		if err != nil {
			continue
		}
		local := completedAt.In(time.Local)
		if local.Before(start) || local.After(end) {
			continue
		}
		out = append(out, task)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CompletedT < out[j].CompletedT
	})
	return out, nil
}

// TasksCompletedOn returns tasks completed on the given local calendar day.
func (c *Client) TasksCompletedOn(day time.Time) ([]Task, error) {
	if day.IsZero() {
		day = time.Now()
	}
	return c.TasksCompletedBetween(day, day)
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
		if t.Deleted.Int() == 0 && !t.Done() {
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

// FindTaskByID loads a task by id from open, completed, or project task lists.
func (c *Client) FindTaskByID(id string) (map[string]any, error) {
	return c.findTaskRawByID(id, "")
}

// findTaskRawByID locates raw task JSON by id. Completed tasks live outside
// /api/v2/project/all/tasks, so projectRef is checked first when provided.
func (c *Client) findTaskRawByID(id, projectRef string) (map[string]any, error) {
	var endpoints []string
	if projectRef != "" {
		if pid, err := c.ResolveProject(projectRef); err == nil {
			endpoints = append(endpoints, "/api/v2/project/"+pid+"/tasks")
		}
	}
	endpoints = append(endpoints,
		"/api/v2/project/all/tasks",
		"/api/v2/project/all/closed",
	)
	for _, ep := range endpoints {
		raw, err := c.GetRaw(ep)
		if err != nil {
			continue
		}
		tasks, err := parseTasksRaw(raw)
		if err != nil {
			continue
		}
		if task, ok := findTaskInRawList(tasks, id); ok {
			return task, nil
		}
	}
	return nil, fmt.Errorf("no task with id %q", id)
}

func findTaskInRawList(tasks []map[string]any, id string) (map[string]any, bool) {
	for _, t := range tasks {
		if got, _ := t["id"].(string); got == id {
			return t, true
		}
	}
	return nil, false
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
