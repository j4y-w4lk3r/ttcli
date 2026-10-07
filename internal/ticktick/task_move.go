package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// MoveResult is returned when a task is moved between lists. The task keeps
// its id; NewTaskID and PreviousID are the same.
type MoveResult struct {
	NewTaskID  string
	PreviousID string
}

// MoveTask moves a task to another list. TickTick ignores projectId on a
// batch update. The web client moves with POST /api/v2/batch/taskProject.
// Copying and then deleting the original lands the original in Trash,
// because that delete is TickTick's move-to-trash.
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

	items := []map[string]string{{
		"taskId":        taskID,
		"fromProjectId": fromPID,
		"toProjectId":   toPID,
	}}
	b, err := json.Marshal(items)
	if err != nil {
		return MoveResult{}, err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskProject", b)
	if err != nil {
		return MoveResult{}, err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil && len(resp.ID2Error) > 0 {
		return MoveResult{}, fmt.Errorf("move failed: %v", resp.ID2Error)
	}
	if err := c.confirmTaskMoved(fromPID, toPID, taskID); err != nil {
		return MoveResult{}, err
	}

	if wasCompleted {
		if err := c.CompleteTask(toPID, taskID); err != nil {
			return MoveResult{NewTaskID: taskID, PreviousID: taskID},
				fmt.Errorf("moved task but could not mark completed: %w", err)
		}
	}

	return MoveResult{NewTaskID: taskID, PreviousID: taskID}, nil
}

func (c *Client) confirmTaskMoved(fromPID, toPID, taskID string) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		inDest, err := c.projectHasLiveTask(toPID, taskID)
		if err != nil {
			return err
		}
		inSrc, err := c.projectHasLiveTask(fromPID, taskID)
		if err != nil {
			return err
		}
		if inDest && !inSrc {
			return nil
		}
		lastErr = fmt.Errorf("move did not apply to task %s", taskID)
	}
	return lastErr
}

func (c *Client) projectHasLiveTask(projectID, taskID string) (bool, error) {
	byID, err := c.projectTaskMap(projectID)
	if err != nil {
		return false, err
	}
	task, ok := byID[taskID]
	if !ok {
		return false, nil
	}
	return deletedNum(task["deleted"]) == 0, nil
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
