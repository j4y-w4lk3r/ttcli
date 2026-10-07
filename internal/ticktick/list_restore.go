package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ProjectSnapshot is the local copy used to recreate a deleted list.
type ProjectSnapshot struct {
	Project map[string]any
	Tasks   []json.RawMessage
}

// SnapshotProject reads a list and its tasks before deletion.
func (c *Client) SnapshotProject(projectRef string) (ProjectSnapshot, error) {
	ref := strings.TrimSpace(projectRef)
	if ref == "" || strings.EqualFold(ref, "inbox") || strings.HasPrefix(strings.ToLower(ref), "inbox") {
		return ProjectSnapshot{}, fmt.Errorf("inbox cannot be deleted")
	}
	project, err := c.GetProject(projectRef)
	if err != nil {
		return ProjectSnapshot{}, err
	}
	id, _ := project["id"].(string)
	if strings.HasPrefix(strings.ToLower(id), "inbox") {
		return ProjectSnapshot{}, fmt.Errorf("inbox cannot be deleted")
	}
	tasks, err := c.rawProjectTasks(id)
	if err != nil {
		return ProjectSnapshot{}, err
	}
	if completed, cerr := c.ProjectCompletedTasks(id); cerr == nil {
		tasks = mergeRawTasks(tasks, completed)
	}
	if abandoned, aerr := c.AccountAbandonedTasks(); aerr == nil {
		tasks = mergeRawTasks(tasks, tasksInProject(abandoned, id))
	}
	if trashed, terr := c.AccountTrashTasks(); terr == nil {
		tasks = mergeRawTasks(tasks, tasksInProject(trashed, id))
	}
	return ProjectSnapshot{Project: project, Tasks: tasks}, nil
}

func tasksInProject(tasks []Task, projectID string) []Task {
	var out []Task
	for _, task := range tasks {
		if task.ProjectID == projectID {
			out = append(out, task)
		}
	}
	return out
}

// RestoreProjectSnapshot creates a new list and copies the snapshotted tasks into it.
// Task status is kept, including completed (2) and won't-do (-1).
func (c *Client) RestoreProjectSnapshot(snap ProjectSnapshot) (string, int, error) {
	projectID, err := c.createProjectFromSnapshot(snap)
	if err != nil {
		return "", 0, err
	}
	n, err := c.RestoreProjectTasks(projectID, snap)
	return projectID, n, err
}

func (c *Client) createProjectFromSnapshot(snap ProjectSnapshot) (string, error) {
	name, _ := snap.Project["name"].(string)
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("archived list has no name")
	}
	color, _ := snap.Project["color"].(string)
	kind, _ := snap.Project["kind"].(string)
	folder, _ := snap.Project["groupId"].(string)
	if folder == "" {
		folder = "none"
	}
	return c.CreateProject(name, folder, color, kind)
}

// RestoreProjectTasks copies snapshotted tasks into an existing list.
func (c *Client) RestoreProjectTasks(projectID string, snap ProjectSnapshot) (int, error) {
	rewritten, err := rewriteRestoredTasks(projectID, snap.Tasks)
	if err != nil {
		return 0, err
	}
	if err := c.addTasks(rewritten); err != nil {
		return 0, err
	}
	return len(rewritten), nil
}

func (c *Client) rawProjectTasks(projectID string) ([]json.RawMessage, error) {
	raw, err := c.GetRaw("/api/v2/project/" + projectID + "/tasks")
	if err != nil {
		return nil, err
	}
	return rawTasksFromBody(raw), nil
}

func rawTasksFromBody(raw []byte) []json.RawMessage {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	var obj struct {
		Tasks        []json.RawMessage `json:"tasks"`
		SyncTaskBean struct {
			Update []json.RawMessage `json:"update"`
		} `json:"syncTaskBean"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if len(obj.Tasks) > 0 {
			return obj.Tasks
		}
		if len(obj.SyncTaskBean.Update) > 0 {
			return obj.SyncTaskBean.Update
		}
	}
	return nil
}

func mergeRawTasks(existing []json.RawMessage, completed []Task) []json.RawMessage {
	seen := map[string]struct{}{}
	for _, raw := range existing {
		var task Task
		if json.Unmarshal(raw, &task) == nil && task.ID != "" {
			seen[task.ID] = struct{}{}
		}
	}
	for _, task := range completed {
		if task.ID == "" {
			continue
		}
		if _, ok := seen[task.ID]; ok {
			continue
		}
		raw, err := json.Marshal(task)
		if err != nil {
			continue
		}
		existing = append(existing, raw)
		seen[task.ID] = struct{}{}
	}
	return existing
}

func rewriteRestoredTasks(projectID string, tasks []json.RawMessage) ([]map[string]any, error) {
	parsed := make([]map[string]any, 0, len(tasks))
	idMap := make(map[string]string, len(tasks))
	for _, raw := range tasks {
		var task map[string]any
		if err := json.Unmarshal(raw, &task); err != nil {
			return nil, fmt.Errorf("decode archived task: %w", err)
		}
		oldID, _ := task["id"].(string)
		if oldID == "" {
			continue
		}
		idMap[oldID] = generateID()
		parsed = append(parsed, task)
	}
	parents := make([]map[string]any, 0, len(parsed))
	children := make([]map[string]any, 0)
	for _, task := range parsed {
		oldID, _ := task["id"].(string)
		task["id"] = idMap[oldID]
		task["projectId"] = projectID
		delete(task, "etag")
		if parent, _ := task["parentId"].(string); parent != "" {
			if next, ok := idMap[parent]; ok {
				task["parentId"] = next
			} else {
				delete(task, "parentId")
			}
		}
		if ids, ok := task["childIds"].([]any); ok {
			next := make([]any, 0, len(ids))
			for _, id := range ids {
				text, _ := id.(string)
				if mapped, ok := idMap[text]; ok {
					next = append(next, mapped)
				}
			}
			task["childIds"] = next
		}
		if items, ok := task["items"].([]any); ok {
			for _, value := range items {
				if item, ok := value.(map[string]any); ok {
					item["id"] = generateID()
				}
			}
		}
		if parent, _ := task["parentId"].(string); parent != "" {
			children = append(children, task)
		} else {
			parents = append(parents, task)
		}
	}
	return append(parents, children...), nil
}

func (c *Client) addTasks(tasks []map[string]any) error {
	const chunk = 40
	for start := 0; start < len(tasks); start += chunk {
		end := start + chunk
		if end > len(tasks) {
			end = len(tasks)
		}
		batch := make([]any, 0, end-start)
		for _, task := range tasks[start:end] {
			batch = append(batch, task)
		}
		payload := map[string]any{
			"add": batch, "update": []any{}, "delete": []any{},
			"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
		}
		body, _ := json.Marshal(payload)
		if _, err := c.do(http.MethodPost, "/api/v2/batch/task", body); err != nil {
			return err
		}
	}
	return nil
}
