package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestSelectedListUsesAFilledIconAndSelectedFolderOpens(t *testing.T) {
	m := fixtureModel(80, 30)
	var listRow, folderRow listRow
	for _, row := range m.listRows {
		if listRow.node.ID == "" && row.node.Kind == "list" {
			listRow = row
		}
		if folderRow.node.ID == "" && row.node.Kind == "folder" {
			folderRow = row
		}
	}
	if listRow.node.ID == "" || folderRow.node.ID == "" {
		t.Fatalf("rows list=%+v folder=%+v", listRow.node, folderRow.node)
	}
	selectedList := stripANSI(m.renderListRowSelected(listRow, 40, true))
	idleList := stripANSI(m.renderListRowSelected(listRow, 40, false))
	if !strings.Contains(selectedList, iconListSelected) || strings.Contains(selectedList, iconTaskOpen) {
		t.Fatalf("selected list=%q", selectedList)
	}
	if !strings.Contains(idleList, iconTaskOpen) || strings.Contains(idleList, iconListSelected) {
		t.Fatalf("idle list=%q", idleList)
	}
	selectedFolder := stripANSI(m.renderListRowSelected(folderRow, 40, true))
	idleFolder := stripANSI(m.renderListRowSelected(folderRow, 40, false))
	if !strings.Contains(selectedFolder, iconFolderOpen) || strings.Contains(selectedFolder, iconFolder) {
		t.Fatalf("selected folder=%q", selectedFolder)
	}
	if !strings.Contains(idleFolder, iconFolder) || strings.Contains(idleFolder, iconFolderOpen) {
		t.Fatalf("idle folder=%q", idleFolder)
	}
}

func TestSmartListsStayPinnedUnderProjects(t *testing.T) {
	m := fixtureModel(80, 30)
	m.view = viewTasks
	view := stripANSI(m.renderLists(m.layout()))
	completed := strings.Index(view, "Completed")
	wont := strings.Index(view, "Won't Do")
	trash := strings.Index(view, "Trash")
	if completed < 0 || wont < completed || trash < wont {
		t.Fatalf("smart lists missing:\n%s", view)
	}
	if strings.Index(view, "List0") > completed {
		t.Fatalf("projects rendered after smart lists:\n%s", view)
	}
}

func TestCalendarFooterNamesThePeriod(t *testing.T) {
	m := fixtureModel(80, 24)
	m.view = viewCalendar
	m.calMode = calModeWeek
	m.calDate = time.Date(2026, 9, 25, 10, 0, 0, 0, time.Local)
	footer := m.footerStatus()
	if strings.Contains(footer, "items") || !strings.Contains(footer, "week") || !strings.Contains(footer, "25 Sep") {
		t.Fatalf("footer=%q", footer)
	}
}

func TestListSearchOpensTrashAndFocusesTasks(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	m.projectID = "p0"
	m.projectName = "List0"

	out, _ := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	m = out
	if m.mode != modeListSearch {
		t.Fatalf("mode=%v", m.mode)
	}
	for _, r := range []rune("trash") {
		out, _ = m.updateListSearch(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = out
	}
	view := stripANSI(m.renderLists(m.layout()))
	if !strings.Contains(view, "Trash") || strings.Contains(view, "List0") || strings.Contains(view, "Completed") {
		t.Fatalf("search view:\n%s", view)
	}
	out, _ = m.updateListSearch(tea.KeyMsg{Type: tea.KeyEnter})
	m = out
	if m.mode != modeNormal || m.paneFocus != paneTasks || m.projectID != smartTrashID || m.projectName != "Trash" {
		t.Fatalf("mode=%v pane=%v id=%q name=%q", m.mode, m.paneFocus, m.projectID, m.projectName)
	}
	if m.listSearchInput.Value() != "" {
		t.Fatalf("search left open: %q", m.listSearchInput.Value())
	}
}

func TestListSearchEscLeavesTheCurrentList(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	m.mode = modeListSearch
	m.listSearchInput.SetValue("trash")
	out, _ := m.updateListSearch(tea.KeyMsg{Type: tea.KeyEsc})
	m = out
	if m.mode != modeNormal || m.projectID != "p0" || m.paneFocus != paneLists {
		t.Fatalf("mode=%v pane=%v id=%q", m.mode, m.paneFocus, m.projectID)
	}
}

func TestSmartListShowsLoadedTasksRegardlessOfScope(t *testing.T) {
	m := fixtureModel(80, 24)
	m.projectID = smartCompletedID
	m.projectName = "Completed"
	m.taskScope = TaskScopeOpen
	m.tasks = []ticktick.Task{{ID: "done", Title: "finished", Status: 2, ProjectID: "p0"}}
	if got := m.visibleTasks(); len(got) != 1 || got[0].ID != "done" {
		t.Fatalf("visible=%+v", got)
	}
}

func TestSmartTrashRestoresWithEnterWhileScopeIsAll(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.projectID = smartTrashID
	m.projectName = "Trash"
	m.taskScope = TaskScopeAll
	m.tasks = []ticktick.Task{{
		ID: "coffee", ProjectID: "p0", Title: "Old milk", Deleted: 1,
	}}
	m.taskCursor = 0

	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("d")},
		{Type: tea.KeyEnter},
	} {
		out, cmd := m.updateTasksKey(key)
		if cmd == nil || out.errMsg != "" || out.mode != modeNormal {
			t.Fatalf("key=%s cmd=%v err=%q mode=%v", key.String(), cmd != nil, out.errMsg, out.mode)
		}
	}

	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd != nil || out.mode != modeConfirmDelete || len(out.pendingPermanentDelete) != 1 {
		t.Fatalf("backspace mode=%v pending=%d cmd=%v", out.mode, len(out.pendingPermanentDelete), cmd != nil)
	}
}
