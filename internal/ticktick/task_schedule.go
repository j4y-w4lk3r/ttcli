package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TaskSchedule describes due date, reminder, and duration for a task.
type TaskSchedule struct {
	Start          time.Time
	Due            time.Time // legacy alias; Start takes precedence
	HasDue         bool
	AllDay         bool
	ReminderBefore time.Duration // 0 = at due time
	HasReminder    bool
	Duration       time.Duration
}

type TaskFocusPlan struct {
	Minutes int
	Pomos   int
	Clear   bool
}

// TaskCreateInput is used when creating a task with optional schedule metadata.
type TaskCreateInput struct {
	Title, Content, ProjectID string
	Priority                  int
	Schedule                  *TaskSchedule
	Recurrence                *TaskRecurrence
	FocusPlan                 *TaskFocusPlan
}

// CreateTask adds a task and optionally applies due date / reminder / duration.
func (c *Client) CreateTask(in TaskCreateInput) (string, error) {
	id, err := c.AddTask(in.Title, in.ProjectID, in.Priority, in.Content)
	if err != nil {
		return "", err
	}
	if (in.Schedule == nil || !in.Schedule.HasDue) && in.Recurrence == nil && in.FocusPlan == nil {
		return id, nil
	}
	task, err := c.FindTaskByID(id)
	if err != nil {
		return id, err
	}
	if in.Schedule != nil && in.Schedule.HasDue {
		if err := applyScheduleToMap(task, *in.Schedule); err != nil {
			return id, err
		}
	}
	if err := applyRecurrenceToMap(task, in.Recurrence, false); err != nil {
		return id, err
	}
	if err := applyFocusPlanToMap(task, in.FocusPlan); err != nil {
		return id, err
	}
	return id, c.patchTaskMap(task)
}

// TaskUpdateInput updates an existing task's fields and optional schedule.
type TaskUpdateInput struct {
	Title           string
	Content         string
	Priority        int
	Schedule        *TaskSchedule
	ClearDue        bool
	Recurrence      *TaskRecurrence
	ClearRecurrence bool
	FocusPlan       *TaskFocusPlan
	ClearParent     bool
	OldParentID     string
}

// UpdateTask updates task fields and due date / reminder / duration.
func (c *Client) UpdateTask(taskID, projectRef string, in TaskUpdateInput) error {
	task, err := c.findTaskRawByID(taskID, projectRef)
	if err != nil {
		return err
	}
	task["title"] = in.Title
	task["content"] = in.Content
	task["priority"] = in.Priority
	clearParent := in.ClearParent
	if in.ClearDue {
		clearScheduleOnMap(task)
	} else if in.Schedule != nil && in.Schedule.HasDue {
		if err := applyScheduleToMap(task, *in.Schedule); err != nil {
			return err
		}
	}
	if err := applyRecurrenceToMap(task, in.Recurrence, in.ClearRecurrence); err != nil {
		return err
	}
	if err := applyFocusPlanToMap(task, in.FocusPlan); err != nil {
		return err
	}
	if err := c.patchTaskMap(task); err != nil {
		return err
	}
	if clearParent {
		return c.clearTaskParent(task, in.OldParentID)
	}
	return nil
}

// ForgetStaleChildIDs removes child ids from a parent when those tasks
// already name a different parent. TickTick ignores childIds on a normal
// task update, so this reaffirms the child's real parent and names the
// stale parent as oldParentId. The child's parent field stays as it is.
func (c *Client) ForgetStaleChildIDs(parentID string, drop []string) error {
	parentID = strings.TrimSpace(parentID)
	dropSet := map[string]bool{}
	for _, id := range drop {
		id = strings.TrimSpace(id)
		if id != "" {
			dropSet[id] = true
		}
	}
	if parentID == "" || len(dropSet) == 0 {
		return nil
	}
	parent, err := c.TaskByID(parentID)
	if err != nil {
		return err
	}
	listed := map[string]bool{}
	for _, id := range parent.ChildIDs {
		listed[id] = true
	}
	keptParent := map[string]string{}
	var items []map[string]string
	for id := range dropSet {
		if !listed[id] {
			continue
		}
		child, err := c.TaskByID(id)
		if err != nil {
			continue
		}
		if child.ParentID == "" || child.ParentID == parentID || child.ProjectID == "" {
			continue
		}
		items = append(items, map[string]string{
			"taskId":      id,
			"projectId":   child.ProjectID,
			"parentId":    child.ParentID,
			"oldParentId": parentID,
		})
		keptParent[id] = child.ParentID
	}
	if len(items) == 0 {
		return nil
	}
	body, err := json.Marshal(items)
	if err != nil {
		return err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskParent", body)
	if err != nil {
		return err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil && len(resp.ID2Error) > 0 {
		return fmt.Errorf("could not drop the extra parent: %v", resp.ID2Error)
	}
	updated, err := c.TaskByID(parentID)
	if err != nil {
		return err
	}
	for _, id := range updated.ChildIDs {
		if keptParent[id] != "" {
			return fmt.Errorf("parent still lists %s", id)
		}
	}
	for id, pid := range keptParent {
		child, err := c.TaskByID(id)
		if err != nil {
			return err
		}
		if child.ParentID != pid {
			return fmt.Errorf("task %s parent changed", id)
		}
	}
	return nil
}

// TaskParentLink names a subtask and the parent to detach.
type TaskParentLink struct {
	TaskID      string
	OldParentID string
}

// MakeTasksNormal detaches every subtask in one parent update. The task
// stays on its current list.
func (c *Client) MakeTasksNormal(links []TaskParentLink) (int, error) {
	seen := map[string]bool{}
	var items []map[string]string
	type check struct {
		taskID   string
		parentID string
	}
	var checks []check
	dropped := 0
	for _, link := range links {
		taskID := strings.TrimSpace(link.TaskID)
		parentID := strings.TrimSpace(link.OldParentID)
		if taskID == "" || parentID == "" || seen[taskID] {
			continue
		}
		seen[taskID] = true
		task, err := c.TaskByID(taskID)
		if err != nil || task.ID == "" {
			if err := c.DropMissingChildIDs(parentID, []string{taskID}); err != nil {
				return 0, err
			}
			dropped++
			continue
		}
		if task.ProjectID == "" {
			continue
		}
		items = append(items, map[string]string{
			"taskId":      taskID,
			"projectId":   task.ProjectID,
			"oldParentId": parentID,
		})
		checks = append(checks, check{taskID: taskID, parentID: parentID})
	}
	if len(items) == 0 {
		return dropped, nil
	}
	body, err := json.Marshal(items)
	if err != nil {
		return 0, err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskParent", body)
	if err != nil {
		return 0, err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil && len(resp.ID2Error) > 0 {
		return 0, fmt.Errorf("could not make the task a normal task: %v", resp.ID2Error)
	}
	for _, item := range checks {
		updated, err := c.TaskByID(item.taskID)
		if err != nil {
			return 0, err
		}
		if updated.ParentID != "" {
			return 0, fmt.Errorf("task is still a subtask")
		}
		parent, err := c.TaskByID(item.parentID)
		if err != nil {
			continue
		}
		for _, id := range parent.ChildIDs {
			if id == item.taskID {
				return 0, fmt.Errorf("task is still a subtask")
			}
		}
	}
	return len(checks) + dropped, nil
}

// DropMissingChildIDs removes child ids that TickTick can no longer load.
// The parent update reports EXISTED for the missing task and still clears
// that id from the parent's subtask list.
func (c *Client) DropMissingChildIDs(parentID string, childIDs []string) error {
	parentID = strings.TrimSpace(parentID)
	if parentID == "" || len(childIDs) == 0 {
		return nil
	}
	parent, err := c.TaskByID(parentID)
	if err != nil {
		return err
	}
	if parent.ProjectID == "" {
		return fmt.Errorf("task has no list")
	}
	listed := map[string]bool{}
	for _, id := range parent.ChildIDs {
		listed[id] = true
	}
	seen := map[string]bool{}
	var items []map[string]string
	for _, id := range childIDs {
		id = strings.TrimSpace(id)
		if id == "" || id == parentID || seen[id] || !listed[id] {
			continue
		}
		if task, err := c.TaskByID(id); err == nil && task.ID != "" {
			continue
		}
		seen[id] = true
		items = append(items, map[string]string{
			"taskId":      id,
			"projectId":   parent.ProjectID,
			"oldParentId": parentID,
		})
	}
	if len(items) == 0 {
		return nil
	}
	body, err := json.Marshal(items)
	if err != nil {
		return err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskParent", body)
	if err != nil {
		return err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil {
		for id, value := range resp.ID2Error {
			if seen[id] && fmt.Sprint(value) == "EXISTED" {
				continue
			}
			if value != nil && fmt.Sprint(value) != "" && fmt.Sprint(value) != "EXISTED" {
				return fmt.Errorf("could not drop the missing subtask: %v", resp.ID2Error)
			}
		}
	}
	updated, err := c.TaskByID(parentID)
	if err != nil {
		return err
	}
	for _, id := range updated.ChildIDs {
		if seen[id] {
			return fmt.Errorf("parent still lists %s", id)
		}
	}
	return nil
}

// clearTaskParent removes a subtask link. TickTick ignores parentId on a
// normal task update; the web client uses POST /api/v2/batch/taskParent.
// oldParentID covers a child that no longer stores a parent id while the
// parent still lists it.
func (c *Client) clearTaskParent(task map[string]any, oldParentID string) error {
	taskID, _ := task["id"].(string)
	projectID, _ := task["projectId"].(string)
	parentID := strings.TrimSpace(oldParentID)
	if parentID == "" {
		parentID, _ = task["parentId"].(string)
	}
	if taskID == "" || parentID == "" {
		return nil
	}
	if projectID == "" {
		return fmt.Errorf("task has no list")
	}
	body, err := json.Marshal([]map[string]string{{
		"taskId":      taskID,
		"projectId":   projectID,
		"oldParentId": parentID,
	}})
	if err != nil {
		return err
	}
	raw, err := c.do(http.MethodPost, "/api/v2/batch/taskParent", body)
	if err != nil {
		return err
	}
	var resp struct {
		ID2Error map[string]any `json:"id2error"`
	}
	if len(strings.TrimSpace(string(raw))) > 0 && json.Unmarshal(raw, &resp) == nil && len(resp.ID2Error) > 0 {
		return fmt.Errorf("could not make the task a normal task: %v", resp.ID2Error)
	}
	updated, err := c.findTaskRawByID(taskID, projectID)
	if err != nil {
		return err
	}
	if pid, _ := updated["parentId"].(string); pid != "" {
		return fmt.Errorf("task is still a subtask")
	}
	parent, err := c.findTaskRawByID(parentID, "")
	if err != nil {
		return nil
	}
	for _, id := range childIDsOf(parent) {
		if id == taskID {
			return fmt.Errorf("task is still a subtask")
		}
	}
	return nil
}

func childIDsOf(task map[string]any) []string {
	switch ids := task["childIds"].(type) {
	case []string:
		return ids
	case []any:
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			if s, ok := id.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// ApplyTaskSchedule sets start/due, reminder, and duration on a raw task map.
func (c *Client) ApplyTaskSchedule(task map[string]any, sched TaskSchedule) error {
	if !sched.HasDue {
		return nil
	}
	if err := applyScheduleToMap(task, sched); err != nil {
		return err
	}
	return c.patchTaskMap(task)
}

func applyScheduleToMap(task map[string]any, sched TaskSchedule) error {
	if !sched.HasDue {
		return nil
	}
	id, _ := task["id"].(string)
	if id == "" {
		return fmt.Errorf("task has no id")
	}
	// Form and CLI schedule inputs are local wall-clock values. Reusing a
	// task's old timezone and calling Time.In would shift an entered 14:00 to
	// 12:00 when that task happened to carry UTC metadata.
	tz := localTZ()
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.Local
	}

	startValue := sched.Start
	if startValue.IsZero() {
		startValue = sched.Due
	}
	start := time.Date(
		startValue.Year(), startValue.Month(), startValue.Day(),
		startValue.Hour(), startValue.Minute(), startValue.Second(),
		startValue.Nanosecond(), loc,
	)
	due := start
	if sched.AllDay {
		dateStr := start.Format("2006-01-02")
		task["isAllDay"] = true
		task["isFloating"] = false
		task["timeZone"] = tz
		task["startDate"] = dateStr
		task["dueDate"] = dateStr
	} else {
		task["isAllDay"] = false
		if sched.Duration > 0 {
			due = start.Add(sched.Duration)
		}
		task["isFloating"] = false
		task["timeZone"] = tz
		task["startDate"] = start.Format(ticktickTimeLayout)
		task["dueDate"] = due.Format(ticktickTimeLayout)
	}

	if sched.HasReminder {
		trigger := formatReminderTrigger(sched.ReminderBefore)
		task["reminder"] = trigger
		task["reminders"] = []any{map[string]any{"trigger": trigger}}
	} else {
		task["reminder"] = ""
		task["reminders"] = []any{}
	}
	return nil
}

func applyRecurrenceToMap(task map[string]any, recurrence *TaskRecurrence, clear bool) error {
	if clear {
		task["repeatFlag"] = ""
		task["repeatFirstDate"] = ""
		return nil
	}
	if recurrence == nil {
		return nil
	}
	normalized, err := NormalizeRecurrenceRule(recurrence.Rule)
	if err != nil {
		return err
	}
	if normalized == "" {
		return fmt.Errorf("repeat rule is empty")
	}
	if recurrence.RepeatFrom != RepeatFromDue && recurrence.RepeatFrom != RepeatFromCompletion {
		return fmt.Errorf("invalid repeat basis %d", recurrence.RepeatFrom)
	}
	task["repeatFlag"] = normalized
	task["repeatFrom"] = recurrence.RepeatFrom
	return nil
}

func applyFocusPlanToMap(task map[string]any, plan *TaskFocusPlan) error {
	if plan == nil {
		return nil
	}
	minutes, pomos := plan.Minutes, plan.Pomos
	if plan.Clear {
		minutes, pomos = 0, 0
	}
	if minutes < 0 {
		return fmt.Errorf("planned focus minutes cannot be negative")
	}
	if pomos < 0 || pomos > 60 {
		return fmt.Errorf("planned pomos must be between 0 and 60")
	}
	if minutes > 0 && pomos == 0 {
		pomos = (minutes + StandardPomoMinutes - 1) / StandardPomoMinutes
		if pomos > 60 {
			pomos = 60
		}
	}

	summary := map[string]any{}
	if raw, ok := task["focusSummaries"].([]any); ok && len(raw) > 0 {
		if existing, ok := raw[0].(map[string]any); ok {
			for key, value := range existing {
				summary[key] = value
			}
		}
	}
	summary["estimatedDuration"] = minutes * 60
	summary["estimatedPomo"] = pomos
	task["focusSummaries"] = []any{summary}
	return nil
}

func clearScheduleOnMap(task map[string]any) {
	task["dueDate"] = ""
	task["startDate"] = ""
	task["reminder"] = ""
	task["reminders"] = []any{}
	task["isAllDay"] = false
}

func (c *Client) patchTaskMap(task map[string]any) error {
	task["modifiedTime"] = time.Now().UTC().Format(ticktickTimeLayout)
	payload := map[string]any{
		"add": []any{}, "update": []any{task}, "delete": []any{},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err := c.do(http.MethodPost, "/api/v2/batch/task", b)
	return err
}

// ParseReminderBefore decodes TickTick TRIGGER strings into a duration before due.
func ParseReminderBefore(raw string) (time.Duration, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	raw = strings.TrimPrefix(raw, "TRIGGER:")
	if raw == "PT0S" || raw == "-PT0S" {
		return 0, true
	}
	if !strings.HasPrefix(raw, "-PT") {
		return 0, false
	}
	body := strings.TrimPrefix(raw, "-PT")
	body = strings.TrimSuffix(body, "S")
	var h, m int
	if i := strings.Index(body, "H"); i >= 0 {
		fmt.Sscanf(body[:i+1], "%dH", &h)
		body = body[i+1:]
	}
	if body != "" {
		body = strings.TrimSuffix(body, "M")
		fmt.Sscanf(body, "%d", &m)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, true
}

// RescheduleTask sets due/start date and reminder to fire at due time.
func (c *Client) RescheduleTask(task map[string]any, due time.Time) error {
	return c.ApplyTaskSchedule(task, TaskSchedule{
		Due:         due,
		HasDue:      true,
		AllDay:      false,
		HasReminder: true,
	})
}

func formatReminderTrigger(before time.Duration) string {
	if before <= 0 {
		return "TRIGGER:PT0S"
	}
	d := before.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	switch {
	case h > 0 && m > 0:
		return fmt.Sprintf("TRIGGER:-PT%dH%dM", h, m)
	case h > 0:
		return fmt.Sprintf("TRIGGER:-PT%dH", h)
	default:
		return fmt.Sprintf("TRIGGER:-PT%dM", max(1, m))
	}
}
