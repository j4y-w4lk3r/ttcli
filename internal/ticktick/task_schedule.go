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
	return c.patchTaskMap(task)
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
