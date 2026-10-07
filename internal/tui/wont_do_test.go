package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestWontDoKeyAbandonsTheSelectedTask(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.taskCursor = 0
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if cmd == nil || out.errMsg != "" {
		t.Fatalf("cmd=%v err=%q", cmd != nil, out.errMsg)
	}
	msg, ok := cmd().(taskAbandonedMsg)
	if !ok || msg.err == nil {
		t.Fatalf("msg=%#v", msg)
	}
}

func TestWontDoKeyReopensAnAbandonedTask(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.projectID = smartWontDoID
	m.tasks = []ticktick.Task{{ID: "later", Title: "Later", Status: -1, ProjectID: "list"}}
	m.taskCursor = 0
	_, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if cmd == nil {
		t.Fatal("w should reopen a won't-do task")
	}
	if _, ok := cmd().(taskReopenedMsg); !ok {
		t.Fatalf("msg=%T", cmd())
	}
}

func TestEnterOnAWontDoTaskReopensIt(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.taskScope = TaskScopeAll
	m.tasks = []ticktick.Task{{ID: "later", Title: "Purchase", Status: -1, ProjectID: "inboxfixture"}}
	m.taskCursor = 0
	_, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should reopen a won't-do task")
	}
	msg := cmd()
	reopened, ok := msg.(taskReopenedMsg)
	if !ok || reopened.err == nil {
		t.Fatalf("msg=%#v", msg)
	}
}

func TestWontDoKeyIgnoresTheListsPane(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	if cmd != nil || out.errMsg != "" {
		t.Fatalf("cmd=%v err=%q", cmd != nil, out.errMsg)
	}
}

func TestTasksToAbandonSkipsCompleted(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{{ID: "done", Title: "Done", Status: 2}}
	m.taskCursor = 0
	if len(m.tasksToAbandon()) != 0 {
		t.Fatal("completed task should not move to Won't Do with w")
	}
}
