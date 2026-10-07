package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	undoReopen          = "reopen"
	undoComplete        = "complete"
	undoReopenAbandoned = "reopen-abandoned"
	undoAbandon         = "abandon"
	undoRestore         = "restore"
	undoTrash           = "trash"
	undoMoveBack        = "move-back"
	undoMoveTo          = "move-to"
)

// taskUndo is the single step u can reverse in the tasks pane.
type taskUndo struct {
	op          string
	tasks       []ticktick.Task
	fromProject string
	toProject   string
	destName    string
}

func undoFor(op string, tasks []ticktick.Task) *taskUndo {
	if len(tasks) == 0 {
		return nil
	}
	return &taskUndo{op: op, tasks: append([]ticktick.Task(nil), tasks...)}
}

func tasksWithIDs(tasks []ticktick.Task, ids []string) []ticktick.Task {
	want := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		want[id] = struct{}{}
	}
	out := make([]ticktick.Task, 0, len(ids))
	for _, task := range tasks {
		if _, ok := want[task.ID]; ok {
			out = append(out, task)
		}
	}
	return out
}

func (m *model) noteTaskUndo(undo *taskUndo, toast string) {
	if undo != nil && len(undo.tasks) > 0 {
		saved := *undo
		saved.tasks = append([]ticktick.Task(nil), undo.tasks...)
		m.taskUndo = &saved
		toast += " · u undo"
	}
	m.toast = toast
}

func (m model) runTaskUndo(undo *taskUndo) tea.Cmd {
	if undo == nil {
		return nil
	}
	switch undo.op {
	case undoReopen:
		return reopenTasksCmd(m.client, undo.tasks, "")
	case undoComplete:
		return completeTasksCmd(m.client, undo.tasks, "")
	case undoReopenAbandoned:
		return reopenAbandonedTasksCmd(m.client, undo.tasks, "")
	case undoAbandon:
		return abandonTasksCmd(m.client, undo.tasks, "")
	case undoRestore:
		return restoreTasksCmd(m.client, undo.tasks, "")
	case undoTrash:
		return trashTasksCmd(m.client, undo.tasks, "")
	case undoMoveBack:
		return moveTasksBackCmd(m.client, undo.tasks, undo.fromProject, m.undoMoveDestName(undo.tasks), undo.destName)
	case undoMoveTo:
		return moveTasksGroupedCmd(m.client, undo.tasks, undo.toProject, undo.destName)
	default:
		return nil
	}
}

func (m model) undoMoveDestName(tasks []ticktick.Task) string {
	ids := map[string]struct{}{}
	for _, task := range tasks {
		if task.ProjectID != "" {
			ids[task.ProjectID] = struct{}{}
		}
	}
	if len(ids) == 1 {
		for id := range ids {
			if name := m.listName(id); name != "" {
				return name
			}
		}
	}
	if len(ids) > 1 {
		return "their lists"
	}
	return "the previous list"
}
