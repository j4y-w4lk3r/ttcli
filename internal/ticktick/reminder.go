package ticktick

import (
	"sort"
	"strings"
	"time"
)

// ReminderGrace is how late a reminder may be and still notify.
// The TUI checks while it is open, so an older reminder stays quiet.
const ReminderGrace = 3 * time.Minute

// allDayReminderHour is the local hour used when an all-day task has a reminder.
const allDayReminderHour = 9

// ReminderHit is one reminder that should notify now.
type ReminderHit struct {
	Task Task
	At   time.Time
	Due  time.Time
	Key  string
}

// ReminderKey identifies one firing so a restart does not send it twice.
func ReminderKey(taskID string, at time.Time) string {
	return taskID + "@" + at.UTC().Format(time.RFC3339)
}

func (t Task) reminderTrigger() string {
	if trigger := strings.TrimSpace(t.Reminder); trigger != "" {
		return trigger
	}
	for _, reminder := range t.Reminders {
		if trigger := strings.TrimSpace(reminder.Trigger); trigger != "" {
			return trigger
		}
	}
	return ""
}

// TaskReminderAt is when this task's reminder should notify.
// A timed task fires at the due time minus the reminder offset.
// An all-day task fires at 09:00 local on the due day, minus that offset.
func TaskReminderAt(t Task) (time.Time, bool) {
	if t.Done() || t.Trashed() || strings.TrimSpace(t.ID) == "" {
		return time.Time{}, false
	}
	before, ok := ParseReminderBefore(t.reminderTrigger())
	if !ok {
		return time.Time{}, false
	}
	due, ok := taskDueInstant(t)
	if !ok {
		return time.Time{}, false
	}
	return due.Add(-before), true
}

func taskDueInstant(t Task) (time.Time, bool) {
	raw := strings.TrimSpace(t.DueDate)
	if raw == "" {
		return time.Time{}, false
	}
	if t.IsAllDay || !strings.Contains(raw, "T") {
		day := raw
		if i := strings.Index(raw, "T"); i >= 0 {
			day = raw[:i]
		}
		if len(day) < 10 {
			return time.Time{}, false
		}
		d, err := time.ParseInLocation("2006-01-02", day[:10], time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return time.Date(d.Year(), d.Month(), d.Day(), allDayReminderHour, 0, 0, 0, time.Local), true
	}
	if tm, err := ParseAPITime(raw); err == nil {
		return tm, true
	}
	if len(raw) >= 16 {
		if tm, err := time.ParseInLocation("2006-01-02T15:04", raw[:16], time.Local); err == nil {
			return tm, true
		}
	}
	return time.Time{}, false
}

// DueReminders returns reminders whose fire time is now or within grace,
// skipping any key already present in sent.
func DueReminders(tasks []Task, now time.Time, sent map[string]struct{}, grace time.Duration) []ReminderHit {
	if grace <= 0 {
		grace = ReminderGrace
	}
	var hits []ReminderHit
	for _, task := range tasks {
		at, ok := TaskReminderAt(task)
		if !ok || at.After(now) || now.Sub(at) > grace {
			continue
		}
		key := ReminderKey(task.ID, at)
		if _, seen := sent[key]; seen {
			continue
		}
		due, _ := taskDueInstant(task)
		hits = append(hits, ReminderHit{Task: task, At: at, Due: due, Key: key})
	}
	sort.Slice(hits, func(i, j int) bool {
		if !hits[i].At.Equal(hits[j].At) {
			return hits[i].At.Before(hits[j].At)
		}
		return hits[i].Task.ID < hits[j].Task.ID
	})
	return hits
}
