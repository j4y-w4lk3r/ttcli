package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type listPickerPurpose int

const (
	pickerMoveTask listPickerPurpose = iota
	pickerMoveList
	pickerAddList
)

type listPickerRow struct {
	id   string
	name string
	kind string // list, folder, none
}

func (m *model) openListPicker(purpose listPickerPurpose) {
	m.listPickerPurpose = purpose
	m.listPickerCursor = 0
	m.listPickerRows = m.buildListPickerRows(purpose)
	if purpose == pickerMoveTask {
		toMove := m.tasksToMove()
		if len(toMove) == 0 {
			return
		}
		m.listPickerTaskIDs = make([]string, len(toMove))
		for i, t := range toMove {
			m.listPickerTaskIDs[i] = t.ID
		}
		m.listPickerFromProject = m.projectID
		if m.listPickerFromProject == "" {
			m.listPickerFromProject = toMove[0].ProjectID
		}
		for i, row := range m.listPickerRows {
			if row.id == m.listPickerFromProject {
				m.listPickerCursor = i
				break
			}
		}
	} else if purpose == pickerMoveList {
		if row, ok := m.currentListRowForRename(); ok && row.node.Kind == "list" {
			m.listPickerListRef = row.node.ID
			if m.listPickerListRef == "" {
				m.listPickerListRef = row.node.Name
			}
		}
	} else if purpose == pickerAddList {
		want := m.listFolderContextForCursor(m.listCursor)
		for i, row := range m.listPickerRows {
			if folderPickerRowMatches(want, row) {
				m.listPickerCursor = i
				break
			}
		}
	}
	if len(m.listPickerRows) == 0 {
		m.errMsg = "no destinations available"
		return
	}
	m.mode = modeListPicker
}

func (m model) buildListPickerRows(purpose listPickerPurpose) []listPickerRow {
	switch purpose {
	case pickerMoveList, pickerAddList:
		var rows []listPickerRow
		rows = append(rows, listPickerRow{id: "NONE", name: "(no folder)", kind: "none"})
		for _, r := range m.listRows {
			if r.node.Kind == "folder" && r.node.Name != "(no folder)" {
				rows = append(rows, listPickerRow{id: r.node.ID, name: r.node.Name, kind: "folder"})
			}
		}
		return rows
	default:
		var rows []listPickerRow
		for _, r := range m.selectableLists() {
			rows = append(rows, listPickerRow{id: r.node.ID, name: r.node.Name, kind: "list"})
		}
		return rows
	}
}

func (m model) renderListPickerOverlay() string {
	boxW, innerW, listH := focusPickerLayout(m.width, m.height)
	title := "Move task to list"
	switch m.listPickerPurpose {
	case pickerMoveList:
		title = "Move list to folder"
	case pickerAddList:
		title = "Create list in folder"
	default:
		if n := len(m.listPickerTaskIDs); n > 1 {
			title = fmt.Sprintf("Move %d tasks to list", n)
		}
	}
	var lines []string
	lines = append(lines, helpInnerLine(headerStyle.Render(iconList+"  "+title), innerW))
	if m.listPickerPurpose == pickerMoveTask {
		switch len(m.listPickerTaskIDs) {
		case 1:
			if t, ok := m.selectedTask(); ok {
				lines = append(lines, helpInnerLine(hintStyle.Render("Task: "+t.Title), innerW))
			}
		case 0:
		default:
			lines = append(lines, helpInnerLine(hintStyle.Render(fmt.Sprintf("%d tasks selected", len(m.listPickerTaskIDs))), innerW))
		}
	}
	lines = append(lines, helpInnerLine(focusPickerDivider(innerW), innerW))

	win := computeScrollWindow(m.listPickerCursor, len(m.listPickerRows), listH)
	for i := win.Start; i < win.End; i++ {
		row := m.listPickerRows[i]
		selected := i == m.listPickerCursor
		prefix := "  "
		st := listIdleStyle
		if selected {
			prefix = "▸ "
			st = listSelStyle
		}
		icon := iconTaskOpen
		if row.kind == "folder" {
			icon = iconFolder
		}
		if row.kind == "none" {
			icon = "○"
		}
		line := prefix + st.Render(icon+" "+row.name)
		lines = append(lines, helpInnerLine(line, innerW))
	}
	lines = append(lines, helpInnerLine(focusPickerDivider(innerW), innerW))
	action := "move"
	if m.listPickerPurpose == pickerAddList {
		action = "choose folder"
	}
	if h := m.keyHint("j/k select · enter " + action + " · esc cancel"); h != "" {
		lines = append(lines, helpInnerLine(h, innerW))
	}
	box := helpBoxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
	return centerBoxOnPlainScreen(box, m.width, m.height)
}

func (m model) updateListPicker(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		return m, nil
	case "j", "down":
		if len(m.listPickerRows) > 0 {
			m.listPickerCursor = min(m.listPickerCursor+1, len(m.listPickerRows)-1)
		}
		return m, nil
	case "k", "up":
		if len(m.listPickerRows) > 0 {
			m.listPickerCursor = max(m.listPickerCursor-1, 0)
		}
		return m, nil
	case "enter":
		if len(m.listPickerRows) == 0 {
			return m, nil
		}
		dest := m.listPickerRows[m.listPickerCursor]
		switch m.listPickerPurpose {
		case pickerMoveTask:
			if len(m.listPickerTaskIDs) == 0 {
				m.errMsg = "no tasks selected"
				return m, nil
			}
			if dest.id == m.listPickerFromProject {
				m.mode = modeNormal
				m.toast = "already in that list"
				return m, nil
			}
			return m, moveTasksCmd(m.client, m.listPickerTaskIDs, m.listPickerFromProject, dest.id, dest.name)
		case pickerMoveList:
			if m.listPickerListRef == "" {
				m.errMsg = "select a list to move"
				return m, nil
			}
			folder := dest.name
			if dest.kind == "none" {
				folder = "none"
			}
			return m, moveListCmd(m.client, m.listPickerListRef, folder)
		case pickerAddList:
			m.addListFolder = folderRefFromPickerRow(dest)
			m.mode = modeAddList
			m.addInput.Placeholder = "new list name…"
			m.addInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func moveTasksCmd(c *ticktick.Client, taskIDs []string, fromProjectID, destProjectID, destName string) tea.Cmd {
	return func() tea.Msg {
		n, err := c.MoveTasks(taskIDs, fromProjectID, destProjectID)
		if err != nil {
			sessionlog.Appendf("task_move_fail", "count=%d from=%s to=%s ids=%v err=%v", len(taskIDs), fromProjectID, destProjectID, taskIDs, err)
			return taskMovedMsg{destName: destName, count: n, err: err}
		}
		sessionlog.Appendf("task_move_ok", "count=%d to=%s", n, destName)
		return taskMovedMsg{destName: destName, count: n, err: nil}
	}
}

func moveListCmd(c *ticktick.Client, listRef, folder string) tea.Cmd {
	return func() tea.Msg {
		err := c.MoveProject(listRef, folder)
		return listMovedMsg{folder: folder, err: err}
	}
}

func createFolderCmd(c *ticktick.Client, name string) tea.Cmd {
	return func() tea.Msg {
		_, err := c.CreateProjectGroup(name)
		return folderAddedMsg{name: name, err: err}
	}
}

func createListCmd(c *ticktick.Client, name, folder string) tea.Cmd {
	return func() tea.Msg {
		_, err := c.CreateProject(name, folder, "", "TASK")
		return listAddedMsg{name: name, err: err}
	}
}

func deleteListOrFolderCmd(c *ticktick.Client, kind, ref string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch kind {
		case "folder":
			err = c.DeleteProjectGroup(ref)
		default:
			err = c.DeleteProject(ref)
		}
		return listDeletedMsg{kind: kind, name: ref, err: err}
	}
}

func (m model) listFolderContext() string {
	return m.listFolderContextForCursor(m.listCursor)
}

func (m model) listFolderContextForCursor(cursor int) string {
	if cursor < 0 || cursor >= len(m.listRows) {
		return "none"
	}
	row := m.listRows[cursor]
	if row.node.Kind == "folder" {
		if row.node.Name == "(no folder)" {
			return "none"
		}
		return row.node.Name
	}
	for i := cursor; i >= 0; i-- {
		if m.listRows[i].node.Kind == "folder" {
			if m.listRows[i].node.Name == "(no folder)" {
				return "none"
			}
			return m.listRows[i].node.Name
		}
	}
	return "none"
}

func folderPickerRowMatches(want string, row listPickerRow) bool {
	if want == "none" || want == "" {
		return row.kind == "none"
	}
	return row.kind == "folder" && strings.EqualFold(row.name, want)
}

func folderRefFromPickerRow(row listPickerRow) string {
	if row.kind == "none" {
		return "none"
	}
	return row.name
}

func folderDisplayLabel(folder string) string {
	if folder == "" || strings.EqualFold(folder, "none") {
		return "(no folder)"
	}
	return folder
}

func (m *model) openAddListFolderPicker() {
	m.openListPicker(pickerAddList)
}
