package tui

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestListRowsUniqueListIDs(t *testing.T) {
	gs := []ticktick.ProjectGroup{
		{ID: "g1", Name: "x", SortOrder: 1},
		{ID: "g1", Name: "x", SortOrder: 1},
		{ID: "g2", Name: "Y", SortOrder: 2},
	}
	ps := []ticktick.Project{
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
		{ID: "p2", Name: "PXC", GroupID: "g2", Kind: "TASK"},
		{ID: "p3", Name: "List0", GroupID: "NONE", Kind: "TASK"},
	}
	tree := ticktick.ProjectTree(gs, ps)
	rows := buildListRows(tree)

	seen := map[string]int{}
	for _, r := range rows {
		if r.node.Kind != "list" {
			continue
		}
		seen[r.node.ID]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Fatalf("list %s appears %d times in listRows", id, n)
		}
	}
}

func TestTreeReloadDoesNotDuplicateRows(t *testing.T) {
	m := fixtureModel(120, 40)

	out, _ := m.Update(treeLoadedMsg{
		groups: []ticktick.ProjectGroup{
			{ID: "g1", Name: "X", SortOrder: 1},
			{ID: "g2", Name: "Y", SortOrder: 2},
		},
		projects: []ticktick.Project{
			{ID: "p0", Name: "List0", GroupID: "NONE", Kind: "TASK"},
			{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
			{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
			{ID: "p2", Name: "PXC", GroupID: "g1", Kind: "TASK"},
		},
	})
	m = out.(model)

	seen := map[string]int{}
	for _, r := range m.listRows {
		if r.node.Kind == "list" {
			seen[r.node.ID]++
		}
	}
	for id, n := range seen {
		if n > 1 {
			t.Fatalf("after reload list %s appears %d times", id, n)
		}
	}
	if seen["p1"] != 1 {
		t.Fatalf("expected Tech once, counts: %v", seen)
	}
}
