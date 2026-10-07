package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestCompleteOffersAnUndo(t *testing.T) {
	m := fixtureModel(80, 24)
	m.paneFocus = paneTasks
	task := ticktick.Task{ID: "t1", Title: "Milk", ProjectID: "inboxfixture"}
	next, _ := m.Update(taskDoneMsg{count: 1, nextUndo: undoFor(undoReopen, []ticktick.Task{task})})
	got := next.(model)
	if got.taskUndo == nil || got.taskUndo.op != undoReopen || !strings.Contains(got.toast, "u undo") {
		t.Fatalf("toast=%q undo=%+v", got.toast, got.taskUndo)
	}
	out, cmd := got.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil || out.taskUndo != nil {
		t.Fatal("u should start the undo and clear the slot")
	}
	msg, ok := cmd().(taskReopenedMsg)
	if !ok || msg.err == nil {
		t.Fatalf("msg=%#v ok=%v", msg, ok)
	}
}

func TestUndoKeyClearsMarksBeforeTheTaskAction(t *testing.T) {
	m := fixtureModel(80, 24)
	m.paneFocus = paneTasks
	m.taskMarked = map[string]struct{}{"t0": {}}
	m.taskUndo = &taskUndo{op: undoReopen, tasks: []ticktick.Task{{ID: "t1", ProjectID: "p0"}}}
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd != nil || out.markedTaskCount() != 0 || out.taskUndo == nil {
		t.Fatal("marked tasks should clear before the last action is undone")
	}
}

func TestPermanentDeleteClearsTaskUndo(t *testing.T) {
	m := fixtureModel(80, 24)
	m.taskUndo = &taskUndo{op: undoRestore, tasks: []ticktick.Task{{ID: "t1", ProjectID: "p0"}}}
	next, _ := m.Update(taskDeletedMsg{count: 1, op: "permanent"})
	got := next.(model)
	if got.taskUndo != nil {
		t.Fatal("permanent delete should drop the undo")
	}
	if !strings.Contains(got.toast, "permanently deleted") || strings.Contains(got.toast, "u undo") {
		t.Fatalf("toast=%q", got.toast)
	}
}

func TestWontDoUndoReopensTheAbandonedTask(t *testing.T) {
	m := fixtureModel(80, 24)
	m.paneFocus = paneTasks
	task := ticktick.Task{ID: "t1", Title: "Later", ProjectID: "inboxfixture", Status: -1}
	m.taskUndo = undoFor(undoReopenAbandoned, []ticktick.Task{task})
	_, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	if cmd == nil {
		t.Fatal("expected undo")
	}
	if _, ok := cmd().(taskReopenedMsg); !ok {
		t.Fatalf("msg=%T", cmd())
	}
}
