package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestDuplicateCreateInputDropsChildIDs(t *testing.T) {
	in, err := duplicateCreateInput(ticktick.Task{
		ID:        "purchase",
		Title:     "purchase",
		Content:   "this is purhcase note...",
		ProjectID: "purchase-list",
		Priority:  5,
		ChildIDs:  []string{"gone", "pixel"},
		ParentID:  "old-parent",
		Status:    -1,
		FocusSummaries: []ticktick.FocusSummary{{
			EstimatedDuration: 25 * 60,
			EstimatedPomo:     1,
			PomoCount:         1,
		}},
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if in.Title != "purchase-duplicate" || in.Content != "this is purhcase note..." || in.ProjectID != "purchase-list" || in.Priority != 5 {
		t.Fatalf("input=%+v", in)
	}
	if in.FocusPlan == nil || in.FocusPlan.Minutes != 25 || in.FocusPlan.Pomos != 1 {
		t.Fatalf("focus=%+v", in.FocusPlan)
	}
}

func TestDuplicateKeyStartsACopy(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{{
		ID: "purchase", Title: "purchase", ProjectID: "p0", ChildIDs: []string{"gone"},
	}}
	m.taskCursor = 0
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if cmd == nil || out.toast != "duplicating…" || out.errMsg != "" {
		t.Fatalf("cmd=%v toast=%q err=%q", cmd != nil, out.toast, out.errMsg)
	}
}

func TestDuplicateKeyIgnoresTheListsPane(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	if cmd != nil || out.errMsg != "" {
		t.Fatalf("cmd=%v err=%q", cmd != nil, out.errMsg)
	}
}
