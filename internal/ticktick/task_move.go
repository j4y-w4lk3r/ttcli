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

// MoveTasks moves every task in one taskProject request. Completed tasks are
// reopened, moved, then completed again. Returns the number moved.
func (c *Client) MoveTasks(taskIDs []string, fromProjectRef, destProjectRef string) (int, error) {
	if len(taskIDs) == 0 {
		return 0, fmt.Errorf("no tasks to move")
	}
	fromPID, err := c.ResolveProject(fromProjectRef)
	if err != nil {
		return 0, err
	}
	toPID, err := c.ResolveProject(destProjectRef)
	if err != nil {
		return 0, err
	}
	if fromPID == toPID {
		return len(taskIDs), nil
	}
	var reopenCompleted, reopenAbandoned, recomplete, reabandon []string
	items := make([]map[string]string, 0, len(taskIDs))
	for _, id := range taskIDs {
		task, err := c.findTaskRawByID(id, fromPID)
		if err != nil {
			return 0, err
		}
		switch taskMapStatus(task) {
		case 2:
			recomplete = append(recomplete, id)
			reopenCompleted = append(reopenCompleted, id)
		case -1:
			reabandon = append(reabandon, id)
			reopenAbandoned = append(reopenAbandoned, id)
		}
		items = append(items, map[string]string{
			"taskId":        id,
			"fromProjectId": fromPID,
			"toProjectId":   toPID,
		})
	}
	if len(reopenAbandoned) > 0 {
		if err := c.ReopenAbandonedTasks(fromPID, reopenAbandoned); err != nil {
			return 0, err
		}
	}
	if len(reopenCompleted) > 0 {
		if err := c.ReopenTasks(fromPID, reopenCompleted); err != nil {
			return 0, err
		}
	}
	b, err := json.Marshal(items)
	if err != nil {
		return 0, err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskProject", b)
	if err != nil {
		return 0, err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil && len(resp.ID2Error) > 0 {
		return 0, fmt.Errorf("move failed: %v", resp.ID2Error)
	}
	if err := c.confirmTasksMoved(fromPID, toPID, taskIDs); err != nil {
		return 0, err
	}
	if len(recomplete) > 0 {
		if err := c.CompleteTasks(toPID, recomplete); err != nil {
			return len(taskIDs), fmt.Errorf("moved tasks but could not mark completed: %w", err)
		}
	}
	if len(reabandon) > 0 {
		if err := c.AbandonTasks(toPID, reabandon); err != nil {
			return len(taskIDs), fmt.Errorf("moved tasks but could not mark won't do: %w", err)
		}
	}
	return len(taskIDs), nil
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

	status := taskMapStatus(task)
	wasCompleted := status == 2
	wasAbandoned := status == -1
	if wasAbandoned {
		if err := c.ReopenAbandonedTasks(fromPID, []string{taskID}); err != nil {
			return MoveResult{}, fmt.Errorf("reopen won't do task: %w", err)
		}
	} else if status != 0 {
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
	if wasAbandoned {
		if err := c.AbandonTasks(toPID, []string{taskID}); err != nil {
			return MoveResult{NewTaskID: taskID, PreviousID: taskID},
				fmt.Errorf("moved task but could not mark won't do: %w", err)
		}
	}

	return MoveResult{NewTaskID: taskID, PreviousID: taskID}, nil
}

func (c *Client) confirmTaskMoved(fromPID, toPID, taskID string) error {
	return c.confirmTasksMoved(fromPID, toPID, []string{taskID})
}

func (c *Client) confirmTasksMoved(fromPID, toPID string, taskIDs []string) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if attempt > 0 {
			time.Sleep(200 * time.Millisecond)
		}
		dest, err := c.projectTaskMap(toPID)
		if err != nil {
			return err
		}
		src, err := c.projectTaskMap(fromPID)
		if err != nil {
			return err
		}
		lastErr = nil
		for _, taskID := range taskIDs {
			inDest := false
			if task, ok := dest[taskID]; ok {
				inDest = deletedNum(task["deleted"]) == 0
			}
			inSrc := false
			if task, ok := src[taskID]; ok {
				inSrc = deletedNum(task["deleted"]) == 0
			}
			if !inDest || inSrc {
				lastErr = fmt.Errorf("move did not apply to task %s", taskID)
				break
			}
		}
		if lastErr == nil {
			return nil
		}
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
