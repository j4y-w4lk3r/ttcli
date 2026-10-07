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

// TaskByID loads one task, including a won't-do or cross-list parent that the
// open project feed omits.
func (c *Client) TaskByID(id string) (Task, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Task{}, fmt.Errorf("task id required")
	}
	raw, err := c.GetRaw("/api/v2/task/" + url.PathEscape(id))
	if err != nil {
		return Task{}, err
	}
	var task Task
	if json.Unmarshal(raw, &task) == nil && task.ID == id {
		return task, nil
	}
	for _, parsed := range parseTasks(raw) {
		if parsed.ID == id {
			return parsed, nil
		}
	}
	return Task{}, fmt.Errorf("no task with id %q", id)
}

// TasksByID loads each id and skips a task TickTick no longer has.
func (c *Client) TasksByID(ids []string) []Task {
	found, _ := c.ClassifyTaskIDs(ids)
	return found
}

// ClassifyTaskIDs splits ids into tasks TickTick still has and ids it does not.
func (c *Client) ClassifyTaskIDs(ids []string) (found []Task, missing []string) {
	seen := map[string]bool{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		task, err := c.TaskByID(id)
		if err != nil || task.ID == "" {
			missing = append(missing, id)
			continue
		}
		found = append(found, task)
	}
	return found, missing
}

// MissingParentTasks loads parent tasks that are not already in the list.
// A missing or deleted parent is skipped so the list can still render.
func (c *Client) MissingParentTasks(ids []string) []Task {
	var out []Task
	seen := map[string]bool{}
	queue := append([]string{}, ids...)
	for len(queue) > 0 {
		id := strings.TrimSpace(queue[0])
		queue = queue[1:]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		task, err := c.TaskByID(id)
		if err != nil || task.ID == "" {
			continue
		}
		out = append(out, task)
		if task.ParentID != "" && !seen[task.ParentID] {
			queue = append(queue, task.ParentID)
		}
	}
	return out
}

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

const (
	completedPageLimit = 500
	completedMaxPages  = 40
)

// CompletedTasksInRange asks TickTick to limit closed tasks to a time range.
// The local filter in TasksCompletedOn remains authoritative if the private
// endpoint returns a wider window.
func (c *Client) CompletedTasksInRange(start, end time.Time) ([]Task, error) {
	return c.fetchClosedTasks(start, end, completedPageLimit)
}

// AllCompletedTasks returns completed tasks account-wide.
// A project task feed only embeds a few finished tasks, so list views merge
// this history to show every completion in that list.
func (c *Client) AllCompletedTasks() ([]Task, error) {
	start := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now().Add(48 * time.Hour)
	return c.completedTasksPaged(start, end, completedPageLimit)
}

func (c *Client) completedTasksPaged(start, end time.Time, limit int) ([]Task, error) {
	if limit < 1 {
		limit = completedPageLimit
	}
	var all []Task
	seen := map[string]struct{}{}
	cursor := end
	for page := 0; page < completedMaxPages; page++ {
		if !cursor.After(start) {
			break
		}
		batch, err := c.fetchClosedTasks(start, cursor, limit)
		if err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		added := 0
		for _, task := range batch {
			if task.ID != "" {
				if _, ok := seen[task.ID]; ok {
					continue
				}
				seen[task.ID] = struct{}{}
			}
			all = append(all, task)
			added++
		}
		if len(batch) < limit || added == 0 {
			break
		}
		oldest, ok := oldestCompletedTime(batch)
		if !ok || !oldest.Before(cursor) {
			break
		}
		cursor = oldest.Add(-time.Millisecond)
	}
	return all, nil
}

func (c *Client) fetchClosedTasks(start, end time.Time, limit int) ([]Task, error) {
	if limit < 1 {
		limit = completedPageLimit
	}
	values := url.Values{}
	values.Set("from", start.UTC().Format(ticktickTimeLayout))
	values.Set("to", end.UTC().Format(ticktickTimeLayout))
	values.Set("limit", strconv.Itoa(limit))
	// TickTick now requires the closed-task status discriminator. Omitting it
	// produces an opaque HTTP 500 from the private endpoint.
	values.Set("status", "Completed")
	var tasks []Task
	if err := c.getJSON("/api/v2/project/all/closed?"+values.Encode(), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

const (
	projectCompletedPageLimit = 100
	completedQueryLayout      = "2006-01-02 15:04:05"
)

// ProjectCompletedTasks returns every completed task in one list.
// The project task feed only embeds a few of them.
func (c *Client) ProjectCompletedTasks(projectID string) ([]Task, error) {
	return c.projectCompletedPaged(projectID, projectCompletedPageLimit)
}

// AccountCompletedTasks returns completed tasks across every list.
func (c *Client) AccountCompletedTasks() ([]Task, error) {
	return c.completedFeedPaged("/api/v2/project/all/completed/", "", projectCompletedPageLimit)
}

// AccountAbandonedTasks returns won't-do tasks (TickTick status -1).
func (c *Client) AccountAbandonedTasks() ([]Task, error) {
	return c.completedFeedPaged("/api/v2/project/all/closed", "Abandoned", projectCompletedPageLimit)
}

// AccountTrashTasks returns trashed tasks, paging TickTick's trash feed.
func (c *Client) AccountTrashTasks() ([]Task, error) {
	const limit = 100
	start := 0
	var all []Task
	seen := map[string]struct{}{}
	for page := 0; page < completedMaxPages; page++ {
		values := url.Values{}
		values.Set("start", strconv.Itoa(start))
		values.Set("limit", strconv.Itoa(limit))
		var parsed struct {
			Tasks     []Task `json:"tasks"`
			NextStart int    `json:"nextStart"`
		}
		if err := c.getJSON("/api/v2/project/all/trash/pagination?"+values.Encode(), &parsed); err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		added := 0
		for _, task := range parsed.Tasks {
			if task.ID != "" {
				if _, ok := seen[task.ID]; ok {
					continue
				}
				seen[task.ID] = struct{}{}
			}
			all = append(all, task)
			added++
		}
		if added == 0 || parsed.NextStart < 0 || parsed.NextStart <= start {
			break
		}
		start = parsed.NextStart
	}
	return all, nil
}

func (c *Client) projectCompletedPaged(projectID string, limit int) ([]Task, error) {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return nil, fmt.Errorf("project id required")
	}
	if limit < 1 {
		limit = projectCompletedPageLimit
	}
	path := "/api/v2/project/" + url.PathEscape(projectID) + "/completed/"
	return c.completedFeedPaged(path, "", limit)
}

func (c *Client) completedFeedPaged(path, status string, limit int) ([]Task, error) {
	if limit < 1 {
		limit = projectCompletedPageLimit
	}
	to := time.Now().UTC().Add(time.Minute)
	var all []Task
	seen := map[string]struct{}{}
	for page := 0; page < completedMaxPages; page++ {
		batch, err := c.fetchCompletedFeed(path, status, to, limit)
		if err != nil {
			if page == 0 {
				return nil, err
			}
			break
		}
		added := 0
		for _, task := range batch {
			if task.ID != "" {
				if _, ok := seen[task.ID]; ok {
					continue
				}
				seen[task.ID] = struct{}{}
			}
			all = append(all, task)
			added++
		}
		if len(batch) < limit || added == 0 {
			break
		}
		oldest, ok := oldestCompletedTime(batch)
		if !ok {
			break
		}
		next := oldest.UTC()
		if !next.Before(to) {
			next = to.Add(-time.Second)
		}
		to = next
	}
	return all, nil
}

func (c *Client) fetchCompletedFeed(path, status string, to time.Time, limit int) ([]Task, error) {
	values := url.Values{}
	values.Set("from", "")
	values.Set("to", to.UTC().Format(completedQueryLayout))
	values.Set("limit", strconv.Itoa(limit))
	if status != "" {
		values.Set("status", status)
	}
	var tasks []Task
	if err := c.getJSON(path+"?"+values.Encode(), &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func oldestCompletedTime(tasks []Task) (time.Time, bool) {
	var oldest time.Time
	found := false
	for _, task := range tasks {
		if task.CompletedT == "" {
			continue
		}
		completedAt, err := ParseAPITime(task.CompletedT)
		if err != nil {
			continue
		}
		if !found || completedAt.Before(oldest) {
			oldest = completedAt
			found = true
		}
	}
	return oldest, found
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
// /api/v2/project/all/tasks, so a real list is checked first, then the
// single-task read and that list's completed feed.
func (c *Client) findTaskRawByID(id, projectRef string) (map[string]any, error) {
	var projectID string
	if projectRef != "" {
		if pid, err := c.ResolveProject(projectRef); err == nil {
			projectID = pid
		}
	}
	var endpoints []string
	if projectID != "" {
		endpoints = append(endpoints, "/api/v2/project/"+projectID+"/tasks")
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
	if projectID != "" {
		if task, err := c.rawTaskByProject(projectID, id); err == nil {
			return task, nil
		}
		if task, ok := c.findRawInCompletedFeed("/api/v2/project/"+url.PathEscape(projectID)+"/completed/", id); ok {
			return task, nil
		}
	} else if task, ok := c.findRawInCompletedFeed("/api/v2/project/all/completed/", id); ok {
		return task, nil
	}
	// Won't Do parents are omitted from the list feed. The same id read that
	// draws the row still returns them.
	if task, err := c.rawTaskByID(id); err == nil {
		return task, nil
	}
	return nil, fmt.Errorf("no task with id %q", id)
}

func (c *Client) rawTaskByID(id string) (map[string]any, error) {
	raw, err := c.GetRaw("/api/v2/task/" + url.PathEscape(id))
	if err != nil {
		return nil, err
	}
	var task map[string]any
	if json.Unmarshal(raw, &task) == nil {
		if got, _ := task["id"].(string); got == id {
			return task, nil
		}
	}
	tasks, err := parseTasksRaw(raw)
	if err != nil {
		return nil, fmt.Errorf("no task with id %q", id)
	}
	if task, ok := findTaskInRawList(tasks, id); ok {
		return task, nil
	}
	return nil, fmt.Errorf("no task with id %q", id)
}

func (c *Client) rawTaskByProject(projectID, taskID string) (map[string]any, error) {
	raw, err := c.GetRaw("/api/v2/task/" + url.PathEscape(taskID) + "?projectId=" + url.QueryEscape(projectID))
	if err != nil {
		return nil, err
	}
	var task map[string]any
	if json.Unmarshal(raw, &task) != nil {
		return nil, fmt.Errorf("no task with id %q", taskID)
	}
	if got, _ := task["id"].(string); got != taskID {
		return nil, fmt.Errorf("no task with id %q", taskID)
	}
	return task, nil
}

func (c *Client) findRawInCompletedFeed(path, id string) (map[string]any, bool) {
	to := time.Now().UTC().Add(time.Minute)
	for page := 0; page < completedMaxPages; page++ {
		values := url.Values{}
		values.Set("from", "")
		values.Set("to", to.UTC().Format(completedQueryLayout))
		values.Set("limit", strconv.Itoa(projectCompletedPageLimit))
		raw, err := c.GetRaw(path + "?" + values.Encode())
		if err != nil {
			return nil, false
		}
		tasks, err := parseTasksRaw(raw)
		if err != nil || len(tasks) == 0 {
			return nil, false
		}
		if task, ok := findTaskInRawList(tasks, id); ok {
			return task, true
		}
		if len(tasks) < projectCompletedPageLimit {
			return nil, false
		}
		oldest, ok := oldestRawCompletedTime(tasks)
		if !ok {
			return nil, false
		}
		next := oldest.UTC()
		if !next.Before(to) {
			next = to.Add(-time.Second)
		}
		to = next
	}
	return nil, false
}

func oldestRawCompletedTime(tasks []map[string]any) (time.Time, bool) {
	var oldest time.Time
	found := false
	for _, task := range tasks {
		raw, _ := task["completedTime"].(string)
		if raw == "" {
			continue
		}
		completedAt, err := ParseAPITime(raw)
		if err != nil {
			continue
		}
		if !found || completedAt.Before(oldest) {
			oldest = completedAt
			found = true
		}
	}
	return oldest, found
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
