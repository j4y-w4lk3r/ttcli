package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// MoveResult is returned when a task is moved between lists. TickTick reassigns
// a new task id during copy+delete moves.
type MoveResult struct {
	NewTaskID  string
	PreviousID string
}

// MoveTask moves a task to another list. The private API ignores projectId
// changes on batch update, so this copies the task to the destination list
// and deletes the original (same strategy as the TickTick web client's REST
// fallback). The task id changes — see MoveResult.
func (c *Client) MoveTask(taskID, fromProjectRef, destProjectRef string) (MoveResult, error) {
	task, err := c.findTaskRawByID(taskID, fromProjectRef)
	if err != nil {
		return MoveResult{}, err
	}
	return c.moveTaskMap(task, fromProjectRef, destProjectRef)
}

// MoveTasks moves multiple tasks to another list. Returns the number moved.
func (c *Client) MoveTasks(taskIDs []string, fromProjectRef, destProjectRef string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, fmt.Errorf("no tasks to move")
	}
	moved := 0
	var lastErr error
	for _, id := range taskIDs {
		if _, err := c.MoveTask(id, fromProjectRef, destProjectRef); err != nil {
			lastErr = err
			continue
		}
		moved++
	}
	if moved == 0 && lastErr != nil {
		return 0, lastErr
	}
	if lastErr != nil {
		return moved, fmt.Errorf("moved %d/%d: %w", moved, len(taskIDs), lastErr)
	}
	return moved, nil
}

func (c *Client) moveTaskMap(task map[string]any, fromProjectRef, destProjectRef string) (MoveResult, error) {
	taskID, _ := task["id"].(string)
	if taskID == "" {
		return MoveResult{}, fmt.Errorf("task has no id")
	}
	fromPID, err := c.resolveSourceProject(task, fromProjectRef)
	if err != nil {
		return MoveResult{}, err
	}
	toPID, err := c.ResolveProject(destProjectRef)
	if err != nil {
		return MoveResult{}, err
	}
	if fromPID == toPID {
		return MoveResult{NewTaskID: taskID, PreviousID: taskID}, nil
	}

	wasCompleted := taskMapStatus(task) == 2
	if taskMapStatus(task) != 0 {
		if err := c.ReopenTask(fromPID, taskID); err != nil {
			return MoveResult{}, fmt.Errorf("reopen completed task: %w", err)
		}
		task, err = c.findTaskRawByID(taskID, fromProjectRef)
		if err != nil {
			return MoveResult{}, fmt.Errorf("reload task after reopen: %w", err)
		}
	}

	copyTask := cloneTaskMap(task)
	newID := generateID()
	now := time.Now().UTC().Format(ticktickTimeLayout)
	copyTask["id"] = newID
	copyTask["projectId"] = toPID
	copyTask["status"] = 0
	copyTask["modifiedTime"] = now
	delete(copyTask, "completedTime")
	delete(copyTask, "completedUserId")
	if _, ok := copyTask["createdTime"]; ok {
		copyTask["createdTime"] = now
	}

	b, _ := json.Marshal(copyTask)
	rb, err := c.do(http.MethodPost, "/api/v2/task", b)
	if err != nil {
		return MoveResult{}, fmt.Errorf("copy task to destination: %w", err)
	}
	newID, err = parseTaskIDResponse(rb, newID)
	if err != nil {
		return MoveResult{}, fmt.Errorf("copy task to destination: %w", err)
	}

	if _, err := waitForTaskRaw(c, newID, toPID); err != nil {
		return MoveResult{}, fmt.Errorf("copy task to destination: created id not found in %q: %w", destProjectRef, err)
	}

	if err := c.DeleteTask(fromPID, taskID); err != nil {
		return MoveResult{}, fmt.Errorf("remove task from source list: %w", err)
	}

	if wasCompleted {
		if err := c.CompleteTask(toPID, newID); err != nil {
			return MoveResult{NewTaskID: newID, PreviousID: taskID},
				fmt.Errorf("moved task but could not mark completed: %w", err)
		}
	}

	return MoveResult{NewTaskID: newID, PreviousID: taskID}, nil
}

func waitForTaskRaw(c *Client, id, projectRef string) (map[string]any, error) {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		task, err := c.findTaskRawByID(id, projectRef)
		if err == nil {
			return task, nil
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	return nil, lastErr
}

func taskMapStatus(task map[string]any) int {
	switch v := task["status"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func (c *Client) resolveSourceProject(task map[string]any, fromProjectRef string) (string, error) {
	if fromProjectRef != "" {
		return c.ResolveProject(fromProjectRef)
	}
	if pid, _ := task["projectId"].(string); pid != "" {
		return pid, nil
	}
	return "", fmt.Errorf("source list unknown — pass fromProjectRef")
}

func cloneTaskMap(task map[string]any) map[string]any {
	b, err := json.Marshal(task)
	if err != nil {
		out := make(map[string]any, len(task))
		for k, v := range task {
			out[k] = v
		}
		return out
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		out = make(map[string]any, len(task))
		for k, v := range task {
			out[k] = v
		}
	}
	return out
}

func parseTaskIDResponse(rb []byte, fallback string) (string, error) {
	if id, err := canonicalID(rb, fallback); err == nil && id != "" {
		return id, nil
	}
	var task struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rb, &task); err == nil && task.ID != "" {
		return task.ID, nil
	}
	return fallback, nil
}
