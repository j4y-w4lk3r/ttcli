package tui

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/listarchive"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestDoneScopeHidesWontDo(t *testing.T) {
	open := ticktick.Task{ID: "open", Title: "Open", Status: 0}
	done := ticktick.Task{ID: "done", Title: "Done", Status: 2}
	wont := ticktick.Task{ID: "wont", Title: "Abandoned", Status: -1}
	trashed := ticktick.Task{ID: "trash", Title: "Trash", Status: 2, Deleted: 1}
	if !taskMatchesScope(done, TaskScopeDone) || taskMatchesScope(wont, TaskScopeDone) || taskMatchesScope(trashed, TaskScopeDone) {
		t.Fatalf("done=%v wont=%v trash=%v", taskMatchesScope(done, TaskScopeDone), taskMatchesScope(wont, TaskScopeDone), taskMatchesScope(trashed, TaskScopeDone))
	}
	if taskMatchesScope(wont, TaskScopeOpen) || !taskMatchesScope(open, TaskScopeOpen) {
		t.Fatal("open scope changed")
	}

	m := fixtureModel(80, 24)
	m.projectID = "p0"
	m.taskScope = TaskScopeDone
	m.tasks = []ticktick.Task{open, done, wont, trashed}
	got := m.visibleTasks()
	if len(got) != 1 || got[0].ID != "done" {
		t.Fatalf("visible=%+v", got)
	}
}

func TestFolderChildListIDsStopAtTheNextFolder(t *testing.T) {
	m := fixtureModel(80, 24)
	folder := -1
	for i, row := range m.listRows {
		if row.node.Kind == "folder" && row.node.Name == "X" {
			folder = i
			break
		}
	}
	if folder < 0 {
		t.Fatal("folder X missing")
	}
	ids := map[string]bool{}
	for _, id := range m.folderChildListIDs(folder) {
		ids[id] = true
	}
	if len(ids) != 2 || !ids["p1"] || !ids["p2"] {
		t.Fatalf("children=%v", ids)
	}
}

func TestDeletedListToastOffersUndo(t *testing.T) {
	m := fixtureModel(80, 24)
	m.projectID = "p1"
	m.projectName = "Tech"
	next, _ := m.Update(listDeletedMsg{kind: "list", name: "Tech", id: "p1", canUndo: true})
	got := next.(model)
	if !strings.Contains(got.toast, "Tech") || !strings.Contains(got.toast, "u undo") {
		t.Fatalf("toast=%q", got.toast)
	}
	if got.projectID != "" || got.projectName != "" {
		t.Fatalf("current list still selected: %s %s", got.projectID, got.projectName)
	}
}

func TestRestoredListIsSelectedAfterTheTreeReloads(t *testing.T) {
	m := fixtureModel(80, 24)
	next, cmd := m.Update(listRestoredMsg{kind: "list", name: "Zero", projectID: "p2", tasks: 3})
	got := next.(model)
	if cmd == nil || got.pendingSelectListID != "p2" || !strings.Contains(got.toast, "Zero") || !strings.Contains(got.toast, "3") {
		t.Fatalf("toast=%q pending=%s cmd=%v", got.toast, got.pendingSelectListID, cmd != nil)
	}
	loaded, _ := got.Update(treeLoadedMsg{
		projects: []ticktick.Project{{ID: "p2", Name: "Zero", GroupID: "NONE", Kind: "TASK"}},
	})
	selected := loaded.(model)
	row := selected.listRows[selected.listCursor]
	if row.node.ID != "p2" {
		t.Fatalf("cursor=%d row=%+v", selected.listCursor, row.node)
	}
}

func TestStartupSelectsRememberedList(t *testing.T) {
	m := fixtureModel(80, 24)
	m.projectID = ""
	m.projectName = ""
	m.tasks = nil
	m.uiSettings.LastListID = "p2"
	loaded, cmd := m.Update(treeLoadedMsg{
		groups: []ticktick.ProjectGroup{{ID: "g1", Name: "X", SortOrder: 1}},
		projects: []ticktick.Project{
			{ID: "p0", Name: "List0", GroupID: "NONE", Kind: "TASK"},
			{ID: "p2", Name: "PXC", GroupID: "g1", Kind: "TASK"},
		},
	})
	selected := loaded.(model)
	row := selected.listRows[selected.listCursor]
	if row.node.ID != "p2" || cmd == nil {
		t.Fatalf("cursor row=%s cmd=%v", row.node.ID, cmd != nil)
	}
}

func TestTreeReloadKeepsTheOpenList(t *testing.T) {
	m := fixtureModel(80, 24)
	m.projectID = "p0"
	m.uiSettings.LastListID = "p2"
	loaded, cmd := m.Update(treeLoadedMsg{
		projects: []ticktick.Project{
			{ID: "p0", Name: "List0", GroupID: "NONE", Kind: "TASK"},
			{ID: "p2", Name: "PXC", GroupID: "NONE", Kind: "TASK"},
		},
	})
	selected := loaded.(model)
	row := selected.listRows[selected.listCursor]
	if row.node.ID != "p0" || cmd != nil {
		t.Fatalf("cursor row=%s cmd=%v", row.node.ID, cmd != nil)
	}
}

func TestChangingListsRemembersTheSelectionWithoutWritingSettings(t *testing.T) {
	path, err := settingsPath()
	if err != nil {
		t.Fatal(err)
	}
	before, readErr := os.ReadFile(path)
	m := fixtureModel(80, 24)
	for i, row := range m.listRows {
		if row.node.ID == "p2" {
			m.listCursor = i
			break
		}
	}
	out, _ := m.beginListLoad()
	if out.uiSettings.LastListID != "p2" || out.projectID != "p2" {
		t.Fatalf("last=%s project=%s", out.uiSettings.LastListID, out.projectID)
	}
	if readErr == nil {
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(before, after) {
			t.Fatal("list change rewrote tui.json during tests")
		}
	}
}

func TestInboxCannotBeDeleted(t *testing.T) {
	m := fixtureModel(80, 24)
	m.paneFocus = paneLists
	for i, row := range m.listRows {
		if row.node.Kind != "list" {
			continue
		}
		m.listCursor = i
		m.listRows[i].node.ID = "inboxfixture"
		m.listRows[i].node.Name = "Inbox"
		break
	}
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil || !strings.Contains(out.errMsg, "inbox") {
		t.Fatalf("err=%q cmd=%v", out.errMsg, cmd != nil)
	}
}

func TestUndoOnListsPaneWithoutASnapshotReportsNothing(t *testing.T) {
	m := fixtureModel(80, 24)
	m.paneFocus = paneLists
	m.listStore = listarchive.NewStoreAt(filepath.Join(t.TempDir(), "list-archive.json"))
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd != nil || !strings.Contains(out.errMsg, "nothing to undo") {
		t.Fatalf("err=%q cmd=%v", out.errMsg, cmd != nil)
	}
}
