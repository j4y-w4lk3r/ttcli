package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/listarchive"
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
		m.listPickerMoveTasks = append([]ticktick.Task(nil), toMove...)
		m.listPickerTaskIDs = make([]string, len(toMove))
		for i, t := range toMove {
			m.listPickerTaskIDs[i] = t.ID
		}
		m.listPickerFromProject = sharedProjectID(toMove)
		if m.listPickerFromProject == "" && !m.showsTaskListName() {
			m.listPickerFromProject = m.projectID
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
			if r.node.Kind != "list" {
				continue
			}
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
		if len(m.subtaskParentLinks(m.listPickerMoveTasks)) > 0 {
			lines = append(lines, helpInnerLine(hintStyle.Render("subtasks become normal tasks"), innerW))
		}
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
		if selected {
			icon = iconListSelected
		}
		if row.kind == "folder" {
			icon = iconFolder
			if selected {
				icon = iconFolderOpen
			}
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
			pending := tasksLeavingProject(m.listPickerMoveTasks, dest.id, m.listPickerFromProject)
			links := m.subtaskParentLinks(m.listPickerMoveTasks)
			if len(m.listPickerMoveTasks) == 0 {
				m.errMsg = "no tasks selected"
				return m, nil
			}
			if len(pending) == 0 && len(links) == 0 {
				m.mode = modeNormal
				m.toast = "already in that list"
				return m, nil
			}
			m.mode = modeNormal
			if len(pending) == 0 {
				m.toast = fmt.Sprintf("making %d tasks normal…", len(links))
			} else {
				m.toast = "moving…"
			}
			return m, moveTasksGroupedCmd(m.client, pending, links, dest.id, dest.name)
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

func (m model) parentChildLinks(parent ticktick.Task) []ticktick.TaskParentLink {
	if parent.ID == "" {
		return nil
	}
	seen := map[string]bool{}
	var out []ticktick.TaskParentLink
	add := func(id string) {
		if id == "" || id == parent.ID || seen[id] {
			return
		}
		seen[id] = true
		out = append(out, ticktick.TaskParentLink{TaskID: id, OldParentID: parent.ID})
	}
	for _, id := range parent.ChildIDs {
		add(id)
	}
	for _, task := range m.tasks {
		if task.ParentID == parent.ID {
			add(task.ID)
		}
	}
	return out
}

func (m model) linksToMakeNormal() []ticktick.TaskParentLink {
	if task, ok := m.selectedTask(); ok {
		if links := m.parentChildLinks(task); len(links) > 0 {
			return links
		}
	}
	return m.subtaskParentLinks(m.tasksToMove())
}

func makeTasksNormalCmd(c *ticktick.Client, links []ticktick.TaskParentLink) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return taskMovedMsg{err: fmt.Errorf("not connected")}
		}
		n, err := c.MakeTasksNormal(links)
		return taskMovedMsg{madeNormal: n, err: err}
	}
}

func (m model) subtaskParentLinks(tasks []ticktick.Task) []ticktick.TaskParentLink {
	var out []ticktick.TaskParentLink
	seen := map[string]bool{}
	for _, task := range tasks {
		if task.ID == "" || seen[task.ID] {
			continue
		}
		parentID, _ := m.listingParent(task)
		if parentID == "" || parentID == task.ID {
			continue
		}
		seen[task.ID] = true
		out = append(out, ticktick.TaskParentLink{TaskID: task.ID, OldParentID: parentID})
	}
	return out
}

func moveTasksGroupedCmd(c *ticktick.Client, tasks []ticktick.Task, links []ticktick.TaskParentLink, destProjectID, destName string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return taskMovedMsg{destID: destProjectID, destName: destName, err: fmt.Errorf("not connected")}
		}
		moved := 0
		var movedTasks []ticktick.Task
		var lastErr error
		for projectID, taskIDs := range groupTasksByProject(tasks, "") {
			if !realListID(projectID) {
				lastErr = fmt.Errorf("task has no list")
				continue
			}
			n, err := c.MoveTasks(taskIDs, projectID, destProjectID)
			if n > 0 {
				moved += n
				movedTasks = append(movedTasks, tasksWithIDs(tasks, taskIDs)...)
			}
			if err != nil {
				lastErr = err
			}
		}
		msg := taskMovedMsg{
			destID: destProjectID, destName: destName, count: moved, err: lastErr,
			nextUndo: &taskUndo{op: undoMoveBack, tasks: movedTasks, fromProject: destProjectID, destName: destName},
		}
		if len(movedTasks) == 0 {
			msg.nextUndo = nil
		}
		if lastErr != nil {
			sessionlog.Appendf("task_move_fail", "moved=%d to=%s err=%v", moved, destName, lastErr)
			return msg
		}
		if len(links) > 0 {
			n, err := c.MakeTasksNormal(links)
			msg.madeNormal = n
			if err != nil {
				msg.err = err
				sessionlog.Appendf("task_move_fail", "moved=%d normal=%d to=%s err=%v", moved, n, destName, err)
				return msg
			}
		}
		sessionlog.Appendf("task_move_ok", "count=%d normal=%d to=%s", moved, msg.madeNormal, destName)
		return msg
	}
}

func moveTasksBackCmd(c *ticktick.Client, tasks []ticktick.Task, fromProject, toastName, redoName string) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return taskMovedMsg{destName: toastName, err: fmt.Errorf("not connected")}
		}
		moved := 0
		var back []ticktick.Task
		var invalidate []string
		seen := map[string]bool{}
		var lastErr error
		for sourceID, taskIDs := range groupTasksByProject(tasks, "") {
			if !realListID(sourceID) || sourceID == fromProject {
				lastErr = fmt.Errorf("task has no list")
				continue
			}
			n, err := c.MoveTasks(taskIDs, fromProject, sourceID)
			if n > 0 {
				moved += n
				back = append(back, tasksWithIDs(tasks, taskIDs)...)
				if !seen[sourceID] {
					seen[sourceID] = true
					invalidate = append(invalidate, sourceID)
				}
			}
			if err != nil {
				lastErr = err
			}
		}
		if fromProject != "" {
			invalidate = append(invalidate, fromProject)
		}
		msg := taskMovedMsg{destName: toastName, count: moved, err: lastErr, invalidate: invalidate}
		if len(back) > 0 && lastErr == nil {
			msg.nextUndo = &taskUndo{op: undoMoveTo, tasks: back, toProject: fromProject, destName: redoName}
		}
		if moved == 0 && lastErr != nil {
			sessionlog.Appendf("task_move_fail", "undo to=%s err=%v", toastName, lastErr)
			return msg
		}
		if lastErr != nil {
			sessionlog.Appendf("task_move_fail", "undo moved=%d to=%s err=%v", moved, toastName, lastErr)
			return msg
		}
		sessionlog.Appendf("task_move_ok", "undo count=%d to=%s", moved, toastName)
		return msg
	}
}

func tasksLeavingProject(tasks []ticktick.Task, destProjectID, fallbackProjectID string) []ticktick.Task {
	if !realListID(fallbackProjectID) {
		fallbackProjectID = ""
	}
	out := make([]ticktick.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ProjectID == "" {
			task.ProjectID = fallbackProjectID
		}
		if task.ProjectID == destProjectID {
			continue
		}
		out = append(out, task)
	}
	return out
}

func sharedProjectID(tasks []ticktick.Task) string {
	if len(tasks) == 0 {
		return ""
	}
	id := tasks[0].ProjectID
	if !realListID(id) {
		return ""
	}
	for _, task := range tasks[1:] {
		if task.ProjectID != id {
			return ""
		}
	}
	return id
}

func realListID(id string) bool {
	return id != "" && id != allTasksID && !strings.HasPrefix(id, "smart:")
}

func moveListCmd(c *ticktick.Client, listRef, folder string) tea.Cmd {
	return func() tea.Msg {
		err := c.MoveProject(listRef, folder)
		return listMovedMsg{folder: folder, err: err}
	}
}

func (m model) beginReorder() (model, tea.Cmd) {
	row, ok := m.currentListRowForRename()
	if !ok || (row.node.Kind == "folder" && row.node.ID == "") {
		m.errMsg = "select a list or folder"
		return m, nil
	}
	if row.node.Kind == "list" && row.node.ID == m.inboxID {
		m.errMsg = "inbox stays at the top"
		return m, nil
	}
	if row.node.Kind == "list" && !listKnown(m.projects, row.node.ID) {
		m.errMsg = "refresh the lists and try again"
		return m, nil
	}
	if row.node.Kind == "folder" && !groupKnown(m.groups, row.node.ID) {
		m.errMsg = "refresh the lists and try again"
		return m, nil
	}
	m.reorderKind = row.node.Kind
	m.reorderID = row.node.ID
	m.reorderName = row.node.Name
	m.reorderProjects = append([]ticktick.Project(nil), m.projects...)
	m.reorderGroups = append([]ticktick.ProjectGroup(nil), m.groups...)
	m.mode = modeReorderList
	m.toast = "moving " + row.node.Name
	return m, nil
}

func (m model) updateReorderList(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "?":
		m.showHelp = true
		m.helpCursor = 0
		m.helpFilter = ""
		return m, nil
	case "esc":
		m.projects = m.reorderProjects
		m.groups = m.reorderGroups
		m.rebuildSidebar()
		m.selectListByID(m.reorderID)
		m.clearReorder()
		m.toast = "cancelled"
		return m, nil
	case "enter":
		return m.commitReorder()
	case "j", "down":
		m.stepReorder(1)
		return m, nil
	case "k", "up":
		m.stepReorder(-1)
		return m, nil
	}
	return m, nil
}

func (m *model) stepReorder(dir int) {
	edge := "already last"
	if dir < 0 {
		edge = "already first"
	}
	if m.reorderKind == "folder" {
		place := ticktick.MoveFolderStep(m.groups, m.reorderID, dir)
		if !place.Active() {
			m.toast = edge
			return
		}
		m.applyGroupPlacement(place)
		m.toast = "moving " + m.reorderName
		return
	}
	place := ticktick.MoveListVisual(m.projects, m.groups, m.reorderID, dir)
	if !place.Active() {
		m.toast = edge
		return
	}
	m.applyProjectPlacement(place)
	m.toast = "moving " + m.reorderName
}

func (m model) commitReorder() (model, tea.Cmd) {
	kind := m.reorderKind
	id := m.reorderID
	if kind == "folder" {
		place := ticktick.PlacementFromGroups(m.reorderGroups, m.groups, id)
		m.clearReorder()
		if !place.Active() {
			m.toast = "order unchanged"
			return m, nil
		}
		m.toast = "saving…"
		return m, applyGroupOrdersCmd(m.client, place)
	}
	place := ticktick.PlacementFromProjects(m.reorderProjects, m.projects, id)
	m.clearReorder()
	if !place.Active() {
		m.toast = "order unchanged"
		return m, nil
	}
	m.toast = "saving…"
	return m, applyProjectOrdersCmd(m.client, place)
}

func (m *model) clearReorder() {
	m.mode = modeNormal
	m.reorderKind = ""
	m.reorderID = ""
	m.reorderName = ""
	m.reorderProjects = nil
	m.reorderGroups = nil
}

func groupKnown(groups []ticktick.ProjectGroup, id string) bool {
	for _, group := range groups {
		if group.ID == id {
			return true
		}
	}
	return false
}

func listKnown(projects []ticktick.Project, id string) bool {
	for _, project := range projects {
		if project.ID == id {
			return true
		}
	}
	return false
}

func (m *model) rebuildSidebar() {
	m.tree = ticktick.ProjectTreeWithInbox(m.inboxID, m.groups, m.projects)
	m.listRows = buildListRows(m.tree)
}

func (m *model) applyProjectPlacement(place ticktick.ProjectPlacement) {
	if !place.Active() {
		return
	}
	for i := range m.projects {
		if order, ok := place.Orders[m.projects[i].ID]; ok {
			m.projects[i].SortOrder = order
		}
		if place.SetGroup && m.projects[i].ID == place.MovedID {
			m.projects[i].GroupID = place.GroupID
		}
	}
	m.rebuildSidebar()
	m.selectListByID(place.MovedID)
}

func (m *model) applyGroupPlacement(place ticktick.GroupPlacement) {
	if !place.Active() {
		return
	}
	for i := range m.groups {
		if order, ok := place.Orders[m.groups[i].ID]; ok {
			m.groups[i].SortOrder = order
		}
	}
	m.rebuildSidebar()
	m.selectListByID(place.MovedID)
}

func applyProjectOrdersCmd(c *ticktick.Client, place ticktick.ProjectPlacement) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return listMovedMsg{what: "list", err: fmt.Errorf("not connected")}
		}
		err := c.ApplyProjectOrders(place)
		return listMovedMsg{what: "list", err: err}
	}
}

func applyGroupOrdersCmd(c *ticktick.Client, place ticktick.GroupPlacement) tea.Cmd {
	return func() tea.Msg {
		if c == nil {
			return listMovedMsg{what: "folder", err: fmt.Errorf("not connected")}
		}
		err := c.ApplyGroupOrders(place)
		return listMovedMsg{what: "folder", err: err}
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

type listDeleteRequest struct {
	kind         string
	ref          string
	name         string
	id           string
	childListIDs []string
}

func deleteListOrFolderCmd(c *ticktick.Client, store *listarchive.Store, req listDeleteRequest) tea.Cmd {
	return func() tea.Msg {
		msg := listDeletedMsg{kind: req.kind, name: req.name, id: req.id}
		if c == nil {
			msg.err = fmt.Errorf("not connected")
			return msg
		}
		if req.kind != "folder" && isInboxRef(req.ref) {
			msg.err = fmt.Errorf("inbox cannot be deleted")
			return msg
		}
		var saved listarchive.Record
		if store != nil {
			record, err := snapshotDeletedList(c, req)
			if err != nil {
				msg.err = err
				return msg
			}
			saved, err = store.Put(record)
			if err != nil {
				msg.err = fmt.Errorf("could not save list snapshot: %w", err)
				return msg
			}
			msg.canUndo = true
		}
		var err error
		switch req.kind {
		case "folder":
			err = c.DeleteProjectGroup(req.ref)
		default:
			err = c.DeleteProject(req.ref)
		}
		if err != nil {
			if saved.ArchiveID != "" && store != nil {
				_ = store.Delete(saved.ArchiveID)
			}
			msg.canUndo = false
			msg.err = err
			return msg
		}
		return msg
	}
}

func snapshotDeletedList(c *ticktick.Client, req listDeleteRequest) (listarchive.Record, error) {
	name := strings.TrimSpace(req.name)
	if name == "" {
		name = req.ref
	}
	if req.kind == "folder" {
		return listarchive.Record{Kind: "folder", Name: name, ChildListIDs: req.childListIDs}, nil
	}
	snap, err := c.SnapshotProject(req.ref)
	if err != nil {
		return listarchive.Record{}, fmt.Errorf("could not snapshot list: %w", err)
	}
	raw, err := json.Marshal(snap.Project)
	if err != nil {
		return listarchive.Record{}, err
	}
	return listarchive.Record{Kind: "list", Name: name, Project: raw, Tasks: snap.Tasks}, nil
}

func undoDeletedListCmd(c *ticktick.Client, store *listarchive.Store) tea.Cmd {
	return func() tea.Msg {
		if c == nil || store == nil {
			return listRestoredMsg{err: fmt.Errorf("nothing to undo")}
		}
		rec, ok, err := store.LatestPending()
		if err != nil {
			return listRestoredMsg{err: err}
		}
		if !ok {
			return listRestoredMsg{err: fmt.Errorf("nothing to undo")}
		}
		if rec.Kind == "folder" {
			return restoreDeletedFolder(c, store, rec)
		}
		return restoreDeletedList(c, store, rec)
	}
}

func restoreDeletedList(c *ticktick.Client, store *listarchive.Store, rec listarchive.Record) tea.Msg {
	var project map[string]any
	if err := json.Unmarshal(rec.Project, &project); err != nil {
		return listRestoredMsg{name: rec.Name, err: err}
	}
	snap := ticktick.ProjectSnapshot{Project: project, Tasks: rec.Tasks}
	id := rec.NewProjectID
	var n int
	var err error
	if id != "" {
		n, err = c.RestoreProjectTasks(id, snap)
	} else {
		id, n, err = c.RestoreProjectSnapshot(snap)
		if id != "" {
			_ = store.NoteCreated(rec.ArchiveID, id)
		}
	}
	if err != nil {
		return listRestoredMsg{kind: "list", name: rec.Name, projectID: id, tasks: n, err: err}
	}
	if markErr := store.MarkRestored(rec.ArchiveID, id); markErr != nil {
		return listRestoredMsg{kind: "list", name: rec.Name, projectID: id, tasks: n, err: markErr}
	}
	return listRestoredMsg{kind: "list", name: rec.Name, projectID: id, tasks: n}
}

func restoreDeletedFolder(c *ticktick.Client, store *listarchive.Store, rec listarchive.Record) tea.Msg {
	id := rec.NewProjectID
	if id == "" {
		var err error
		id, err = c.CreateProjectGroup(rec.Name)
		if err != nil {
			return listRestoredMsg{kind: "folder", name: rec.Name, err: err}
		}
		_ = store.NoteCreated(rec.ArchiveID, id)
	}
	moved := 0
	var moveErr error
	for _, child := range rec.ChildListIDs {
		if err := c.MoveProject(child, id); err != nil {
			if moveErr == nil {
				moveErr = err
			}
			continue
		}
		moved++
	}
	if moveErr != nil {
		return listRestoredMsg{kind: "folder", name: rec.Name, projectID: id, tasks: moved, err: moveErr}
	}
	if err := store.MarkRestored(rec.ArchiveID, id); err != nil {
		return listRestoredMsg{kind: "folder", name: rec.Name, projectID: id, tasks: moved, err: err}
	}
	return listRestoredMsg{kind: "folder", name: rec.Name, projectID: id, tasks: moved}
}

func (m model) deleteFocusedList() (model, tea.Cmd) {
	row, ok := m.currentListRowForRename()
	if !ok {
		m.errMsg = "select a list or folder to delete"
		return m, nil
	}
	if row.node.Kind == "list" && isInboxRef(row.node.ID) {
		m.errMsg = "inbox cannot be deleted"
		return m, nil
	}
	ref := row.node.Name
	if row.node.ID != "" {
		ref = row.node.ID
	}
	return m, deleteListOrFolderCmd(m.client, m.listStore, listDeleteRequest{
		kind:         row.node.Kind,
		ref:          ref,
		name:         row.node.Name,
		id:           row.node.ID,
		childListIDs: m.folderChildListIDs(m.listCursor),
	})
}

func (m model) folderChildListIDs(cursor int) []string {
	if cursor < 0 || cursor >= len(m.listRows) || m.listRows[cursor].node.Kind != "folder" {
		return nil
	}
	depth := m.listRows[cursor].node.Depth
	var ids []string
	for i := cursor + 1; i < len(m.listRows); i++ {
		next := m.listRows[i]
		if next.node.Depth <= depth {
			break
		}
		if next.node.Kind == "list" && next.node.ID != "" {
			ids = append(ids, next.node.ID)
		}
	}
	return ids
}

func isInboxRef(id string) bool {
	id = strings.TrimSpace(id)
	return strings.EqualFold(id, "inbox") || strings.HasPrefix(strings.ToLower(id), "inbox")
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
