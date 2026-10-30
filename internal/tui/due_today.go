package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func (m model) tasksToSchedule() []ticktick.Task {
	var pool []ticktick.Task
	if m.markedTaskCount() > 0 {
		pool = m.markedTasksFromList()
	} else if task, ok := m.selectedTask(); ok {
		pool = []ticktick.Task{task}
	}
	var out []ticktick.Task
	for _, task := range pool {
		if task.ID == "" || task.Trashed() {
			continue
		}
		out = append(out, task)
	}
	return out
}

func taskDueOnDay(task ticktick.Task, day time.Time) bool {
	due, ok := parseDueDay(task.DueDate)
	return ok && due.Equal(dateOnly(day))
}

func setDueTodayCmd(c *ticktick.Client, tasks []ticktick.Task, day time.Time) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return dueTodayMsg{err: fmt.Errorf("not connected")}
		}
		var changed []ticktick.Task
		for _, task := range tasks {
			if taskDueOnDay(task, day) {
				continue
			}
			if err := c.SetDueOnDay(task.ID, task.ProjectID, day); err != nil {
				return dueTodayMsg{count: len(changed), err: err, nextUndo: undoFor(undoDue, changed)}
			}
			changed = append(changed, task)
		}
		return dueTodayMsg{count: len(changed), nextUndo: undoFor(undoDue, changed)}
	}
}

func restoreDueCmd(c *ticktick.Client, tasks []ticktick.Task) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return dueTodayMsg{err: fmt.Errorf("not connected")}
		}
		for _, task := range tasks {
			if err := c.RestoreTaskDue(task); err != nil {
				return dueTodayMsg{restored: true, err: err}
			}
		}
		return dueTodayMsg{count: len(tasks), restored: true}
	}
}
