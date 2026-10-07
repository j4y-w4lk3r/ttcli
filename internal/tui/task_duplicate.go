package tui

import (
	"fmt"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func duplicateTaskTitle(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return "duplicate"
	}
	return title + "-duplicate"
}

func duplicateCreateInput(task ticktick.Task, fallbackProject string) (ticktick.TaskCreateInput, error) {
	projectID := strings.TrimSpace(task.ProjectID)
	if !realListID(projectID) {
		projectID = strings.TrimSpace(fallbackProject)
	}
	if !realListID(projectID) {
		return ticktick.TaskCreateInput{}, fmt.Errorf("task has no list")
	}
	in := ticktick.TaskCreateInput{
		Title:     duplicateTaskTitle(task.Title),
		Content:   task.Content,
		ProjectID: projectID,
		Priority:  task.Priority.Int(),
	}
	if seconds, pomos, ok := task.FocusEstimate(); ok {
		in.FocusPlan = &ticktick.TaskFocusPlan{
			Minutes: int((seconds + 59) / 60),
			Pomos:   pomos,
		}
	}
	return in, nil
}
