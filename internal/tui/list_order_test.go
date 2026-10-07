package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func reorderFixture() model {
	m := fixtureModel(80, 24)
	m.paneFocus = paneLists
	m.groups = []ticktick.ProjectGroup{{ID: "g1", Name: "X", SortOrder: 1}}
	m.projects = []ticktick.Project{
		{ID: "p0", Name: "List0", GroupID: "NONE", Kind: "TASK", SortOrder: 5},
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK", SortOrder: 10},
		{ID: "p2", Name: "PXC", GroupID: "g1", Kind: "TASK", SortOrder: 30},
	}
	m.rebuildSidebar()
	m.selectListByID("p0")
	return m
}

func TestReorderListSlidesIntoAFolderUntilEnter(t *testing.T) {
	m := reorderFixture()
	picked, cmd := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if cmd != nil || picked.mode != modeReorderList || picked.toast != "moving List0" {
		t.Fatalf("mode=%v toast=%q cmd=%v", picked.mode, picked.toast, cmd)
	}
	moved, cmd := picked.updateReorderList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if cmd != nil {
		t.Fatal("k saved before enter")
	}
	var group string
	var order int64
	for _, project := range moved.projects {
		if project.ID == "p0" {
			group = project.GroupID
			order = project.SortOrder
		}
	}
	if group != "g1" || order <= 30 {
		t.Fatalf("after k group=%s order=%d", group, order)
	}
	if !strings.Contains(stripANSI(moved.renderLists(moved.layout())), "j/k · enter · esc") {
		t.Fatal("missing place hint")
	}
	saved, cmd := moved.updateReorderList(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || saved.mode != modeNormal || saved.toast != "saving…" {
		t.Fatalf("mode=%v toast=%q cmd=%v", saved.mode, saved.toast, cmd)
	}
}

func TestReorderListEscRestoresTheOldPlace(t *testing.T) {
	m := reorderFixture()
	picked, _ := m.updateKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	moved, _ := picked.updateReorderList(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	back, cmd := moved.updateReorderList(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil || back.mode != modeNormal || back.toast != "cancelled" {
		t.Fatalf("mode=%v toast=%q", back.mode, back.toast)
	}
	for _, project := range back.projects {
		if project.ID == "p0" && (project.GroupID != "NONE" || project.SortOrder != 5) {
			t.Fatalf("restored %+v", project)
		}
	}
}
