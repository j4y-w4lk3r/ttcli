package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	smartCompletedID = "smart:completed"
	smartWontDoID    = "smart:abandoned"
	smartTrashID     = "smart:trash"
	allTasksID       = "all:tasks"
)

func smartListNodes() []ticktick.ProjectTreeNode {
	return []ticktick.ProjectTreeNode{
		{Kind: "rule", Name: ""},
		{Kind: "smart", ID: smartCompletedID, Name: "Completed"},
		{Kind: "smart", ID: smartWontDoID, Name: "Won't Do"},
		{Kind: "smart", ID: smartTrashID, Name: "Trash"},
	}
}

func isSmartList(id string) bool {
	return strings.HasPrefix(id, "smart:")
}

func (m model) viewingTrash() bool {
	return m.projectID == smartTrashID || m.effectiveTaskScope() == TaskScopeTrash
}

func (m model) showsTaskListName() bool {
	if m.projectID == allTasksID {
		return true
	}
	for _, row := range m.listRows {
		if row.node.ID == m.projectID && row.node.Kind == "folder" {
			return true
		}
	}
	return false
}

func (m model) listName(projectID string) string {
	if projectID == "" {
		return ""
	}
	for _, row := range m.listRows {
		if row.node.Kind == "list" && row.node.ID == projectID {
			return row.node.Name
		}
	}
	if projectID == m.inboxID {
		return "Inbox"
	}
	return ""
}

func tasksInProjects(tasks []ticktick.Task, projectIDs []string) []ticktick.Task {
	if len(projectIDs) == 0 {
		return nil
	}
	want := make(map[string]struct{}, len(projectIDs))
	for _, id := range projectIDs {
		if id != "" {
			want[id] = struct{}{}
		}
	}
	out := make([]ticktick.Task, 0)
	for _, task := range tasks {
		if _, ok := want[task.ProjectID]; ok {
			out = append(out, task)
		}
	}
	return out
}

func (m model) selectionIsTrashed() bool {
	tasks := m.tasksToDelete()
	if len(tasks) == 0 {
		return false
	}
	for _, task := range tasks {
		if !task.Trashed() {
			return false
		}
	}
	return true
}

func smartListKind(id string) string {
	switch id {
	case smartCompletedID:
		return "completed"
	case smartWontDoID:
		return "abandoned"
	case smartTrashID:
		return "trash"
	default:
		return ""
	}
}

func smartListIcon(id string) string {
	switch id {
	case smartCompletedID:
		return iconCheck
	case smartWontDoID:
		return iconWont
	case smartTrashID:
		return iconTrash
	default:
		return iconList
	}
}

func loadSmartTasksCmd(repo *ticktick.Repository, id, name string, force bool) tea.Cmd {
	return func() tea.Msg {
		if repo == nil {
			return tasksLoadedMsg{projectID: id, projectName: name, err: fmt.Errorf("TickTick data repository unavailable")}
		}
		tasks, cache, err := repo.SmartTasks(smartListKind(id), force)
		return tasksLoadedMsg{
			projectID: id, projectName: name, tasks: tasks,
			cache: cache, err: err,
		}
	}
}
