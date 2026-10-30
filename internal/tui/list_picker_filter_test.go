package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestMovePickerTypesAListName(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.projectID = "p0"
	m.tasks = []ticktick.Task{{ID: "ship", Title: "ship", ProjectID: "p0"}}
	m.taskCursor = 0
	m.openListPicker(pickerMoveTask)
	for _, r := range "tech" {
		var cmd tea.Cmd
		m, cmd = m.updateListPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		if cmd != nil {
			t.Fatal("typing should not move yet")
		}
	}
	if len(m.listPickerRows) != 1 || m.listPickerRows[0].name != "Tech" {
		t.Fatalf("rows=%v", m.listPickerRows)
	}
	out, cmd := m.updateListPicker(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || out.toast != "moving…" {
		t.Fatalf("toast=%q cmd=%v", out.toast, cmd != nil)
	}
}

func TestMovePickerKeepsJKAmongMatches(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.projectID = "p0"
	m.tasks = []ticktick.Task{{ID: "ship", Title: "ship", ProjectID: "p0"}}
	m.openListPicker(pickerMoveTask)
	m, _ = m.updateListPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if len(m.listPickerRows) < 2 {
		t.Fatalf("rows=%v", m.listPickerRows)
	}
	first := m.listPickerRows[0].name
	m, _ = m.updateListPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if m.listPickerCursor != 1 || m.listPickerRows[0].name != first {
		t.Fatalf("cursor=%d rows=%v", m.listPickerCursor, m.listPickerRows)
	}
}
