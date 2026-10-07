package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestFolderAndAllSearchAcrossLists(t *testing.T) {
	m := fixtureModel(120, 36)
	m.paneFocus = paneTasks
	m.taskScope = TaskScopeOpen

	var folder int
	var all int
	for i, row := range m.listRows {
		if row.node.Kind == "folder" && row.node.Name == "X" {
			folder = i
		}
		if row.node.ID == allTasksID {
			all = i
		}
	}
	if !m.listRows[folder].selectable || m.listRows[folder].node.Kind != "folder" {
		t.Fatal("folder X should be selectable")
	}
	if !m.listRows[all].selectable {
		t.Fatal("All should be selectable")
	}

	tasks := []ticktick.Task{
		{ID: "wake", ProjectID: "p1", Title: "wake up"},
		{ID: "pxc", ProjectID: "p2", Title: "pxc notes"},
		{ID: "co", ProjectID: "p3", Title: "company mail"},
	}
	got := tasksInProjects(tasks, m.folderChildListIDs(folder))
	if len(got) != 2 {
		t.Fatalf("folder tasks=%d %+v", len(got), got)
	}

	m.listCursor = folder
	m.projectID = m.listRows[folder].node.ID
	m.projectName = "X"
	m.tasks = got
	m.taskCursor = 0
	opened := pressKey(m, "/")
	for _, r := range "wake" {
		next, _ := opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		opened = next.(model)
	}
	view := stripANSI(opened.renderTasks(opened.layout()))
	if !strings.Contains(view, "wake up") || !strings.Contains(view, "Tech") || strings.Contains(view, "company mail") || strings.Contains(view, "pxc notes") {
		t.Fatalf("folder search:\n%s", view)
	}

	m.projectID = allTasksID
	m.projectName = "All"
	m.tasks = tasks
	m.taskCursor = 0
	m.filterInput.SetValue("")
	m.mode = modeNormal
	opened = pressKey(m, "/")
	next, _ := opened.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("mail")})
	allView := stripANSI(next.(model).renderTasks(next.(model).layout()))
	if !strings.Contains(allView, "Company") || strings.Contains(allView, "wake up") {
		t.Fatalf("all search:\n%s", allView)
	}
}

func TestMoveFromAllUsesTheTasksOwnList(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.projectID = allTasksID
	m.projectName = "All"
	m.tasks = []ticktick.Task{{ID: "wake", ProjectID: "p1", Title: "wake up"}}
	m.taskCursor = 0
	m.openListPicker(pickerMoveTask)
	if m.listPickerFromProject != "p1" || m.mode != modeListPicker {
		t.Fatalf("from=%q mode=%v", m.listPickerFromProject, m.mode)
	}

	m.projectID = "g1"
	m.projectName = "X"
	m.tasks = []ticktick.Task{
		{ID: "wake", ProjectID: "p1", Title: "wake up"},
		{ID: "mail", ProjectID: "p3", Title: "company mail"},
	}
	m.taskMarked = map[string]struct{}{"wake": {}, "mail": {}}
	m.openListPicker(pickerMoveTask)
	if m.listPickerFromProject != "" {
		t.Fatalf("mixed folder move from=%q", m.listPickerFromProject)
	}
	pending := tasksLeavingProject(m.listPickerMoveTasks, "p1", m.listPickerFromProject)
	if len(pending) != 1 || pending[0].ID != "mail" || pending[0].ProjectID != "p3" {
		t.Fatalf("pending=%+v", pending)
	}
}

func TestBackspaceOnAllTrashesTheTasksOwnList(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	for i, row := range m.listRows {
		if row.node.ID == allTasksID {
			m.listCursor = i
			break
		}
	}
	m.projectID = allTasksID
	m.projectName = "All"
	m.tasks = []ticktick.Task{{ID: "wake", ProjectID: "p1", Title: "wake up"}}
	m.taskCursor = 0

	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd == nil || out.errMsg != "" {
		t.Fatalf("err=%q cmd=%v", out.errMsg, cmd != nil)
	}
	msg, ok := cmd().(taskDeletedMsg)
	if !ok || msg.err == nil || strings.Contains(msg.err.Error(), "all:tasks") {
		t.Fatalf("msg=%+v ok=%v", msg, ok)
	}
	grouped, err := writableTaskProjects(out.tasks, allTasksID)
	if err != nil || len(grouped["p1"]) != 1 {
		t.Fatalf("grouped=%v err=%v", grouped, err)
	}
	if _, bad := grouped[allTasksID]; bad {
		t.Fatalf("grouped=%v", grouped)
	}

	reloaded, ok := m.reloadCurrentTasks(false)().(tasksLoadedMsg)
	if !ok || reloaded.projectID != allTasksID || reloaded.err == nil {
		t.Fatalf("reload=%+v ok=%v", reloaded, ok)
	}
	if !strings.Contains(reloaded.err.Error(), "repository unavailable") {
		t.Fatalf("reload err=%v", reloaded.err)
	}
}
