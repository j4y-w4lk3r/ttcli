package tui

import (
	"encoding/json"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/taskarchive"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type fakePermanentDeleteClient struct {
	raw         json.RawMessage
	deleteCalls int
}

func (f *fakePermanentDeleteClient) TaskSnapshot(_, _ string) (json.RawMessage, error) {
	return f.raw, nil
}

func (f *fakePermanentDeleteClient) DeleteTasks(_ string, _ []string) error {
	f.deleteCalls++
	return nil
}

func TestTaskScopesCycleAndSearchNotesAndTags(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{
		{ID: "open", Title: "Open", Content: "buy oat milk", Tags: []string{"home"}},
		{ID: "done", Title: "Done", Status: 2},
		{ID: "trash", Title: "Trash", Deleted: 1},
	}
	wantScopes := []TaskScope{TaskScopeDone, TaskScopeTrash, TaskScopeAll, TaskScopeArchive, TaskScopeOpen}
	for _, want := range wantScopes {
		m = pressKey(m, "c")
		if got := m.effectiveTaskScope(); got != want {
			t.Fatalf("scope=%s want %s", got, want)
		}
	}
	m.filterInput.SetValue("oat milk")
	if tasks := m.visibleTasks(); len(tasks) != 1 || tasks[0].ID != "open" {
		t.Fatalf("notes search=%+v", tasks)
	}
	m.filterInput.SetValue("home")
	if tasks := m.visibleTasks(); len(tasks) != 1 || tasks[0].ID != "open" {
		t.Fatalf("tag search=%+v", tasks)
	}
}

func TestArchiveScopeIsProjectScopedAndSearchable(t *testing.T) {
	m := fixtureModel(120, 40)
	m.taskScope = TaskScopeArchive
	m.archiveRecords = []taskarchive.Record{
		{ArchiveID: "p0:a", Task: ticktick.Task{ID: "a", ProjectID: "p0", Title: "Food", Desc: "weekday lunch"}},
		{ArchiveID: "p1:b", Task: ticktick.Task{ID: "b", ProjectID: "p1", Title: "Other"}},
	}
	m.filterInput.SetValue("lunch")
	tasks := m.visibleTasks()
	if len(tasks) != 1 || tasks[0].ID != "archive:p0:a" {
		t.Fatalf("archive rows=%+v", tasks)
	}
	total, matching, shown := m.taskScopeStats()
	if total != 1 || matching != 1 || shown != 1 {
		t.Fatalf("archive counts=%d/%d/%d", total, matching, shown)
	}
}

func TestTrashBackspaceRequiresExplicitConfirmation(t *testing.T) {
	m := fixtureModel(120, 40)
	m.paneFocus = paneTasks
	m.taskScope = TaskScopeTrash
	m.tasks = []ticktick.Task{{ID: "trash", ProjectID: "p0", Title: "Trash", Deleted: 1}}
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd != nil || out.mode != modeConfirmDelete || len(out.pendingPermanentDelete) != 1 {
		t.Fatalf("mode=%v pending=%d cmd=%v", out.mode, len(out.pendingPermanentDelete), cmd)
	}
	cancelled, _ := out.updatePermanentDeleteConfirmation(tea.KeyMsg{Type: tea.KeyEsc})
	if cancelled.mode != modeNormal || len(cancelled.pendingPermanentDelete) != 0 {
		t.Fatalf("confirmation did not cancel: %+v", cancelled)
	}
}

func TestPermanentDeleteAbortsWhenArchiveCannotBeSaved(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	const taskID = "abcdef0123456789abcdef01"
	client := &fakePermanentDeleteClient{
		raw: json.RawMessage(`{"id":"abcdef0123456789abcdef01","projectId":"0123456789abcdef01234567"}`),
	}
	// Turn the archive path into a directory so the atomic rename fails.
	badStore := taskarchive.NewStoreAt(t.TempDir())
	msg := permanentlyDeleteTasksCmd(
		client, badStore,
		[]ticktick.Task{{ID: taskID, ProjectID: projectID, Title: "Keep snapshot"}},
		projectID,
	)().(taskDeletedMsg)
	if msg.err == nil {
		t.Fatal("expected archive write failure")
	}
	if client.deleteCalls != 0 {
		t.Fatalf("remote delete calls=%d; deletion must abort", client.deleteCalls)
	}
}
