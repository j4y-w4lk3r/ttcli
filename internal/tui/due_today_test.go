package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestDueTodayKeySetsTheSelectedTask(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{{ID: "later", Title: "Ship", ProjectID: "p0"}}
	m.taskCursor = 0
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if cmd == nil || out.toast != "setting due to today…" {
		t.Fatalf("toast=%q cmd=%v", out.toast, cmd != nil)
	}
	msg, ok := cmd().(dueTodayMsg)
	if !ok || msg.err == nil || msg.err.Error() != "not connected" {
		t.Fatalf("msg=%#v", msg)
	}
}

func TestDueTodayKeyLeavesATaskThatIsAlreadyToday(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{{
		ID: "today", Title: "Ship", ProjectID: "p0",
		DueDate: dateOnly(time.Now()).Format("2006-01-02"), IsAllDay: true,
	}}
	m.taskCursor = 0
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if cmd != nil || out.toast != "already due today" {
		t.Fatalf("toast=%q cmd=%v", out.toast, cmd != nil)
	}
}

func TestDueTodayKeyIgnoresTheListsPane(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	out, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if cmd != nil || out.toast != "" {
		t.Fatalf("toast=%q cmd=%v", out.toast, cmd != nil)
	}
}
