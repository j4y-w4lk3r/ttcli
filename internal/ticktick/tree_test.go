package ticktick

import "testing"

func TestProjectTree_emptyFolder(t *testing.T) {
	gs := []ProjectGroup{{ID: "g1", Name: "personal", SortOrder: 1}}
	nodes := ProjectTree(gs, nil)
	if len(nodes) != 2 {
		t.Fatalf("want folder+empty marker, got %d nodes", len(nodes))
	}
	if nodes[0].Name != "personal" || nodes[1].Kind != "empty" {
		t.Fatalf("got %+v", nodes)
	}
}

func TestProjectTree(t *testing.T) {
	gs := []ProjectGroup{
		{ID: "g1", Name: "Work", SortOrder: 1},
		{ID: "g2", Name: "Home", SortOrder: 2},
	}
	ps := []Project{
		{ID: "p1", Name: "PXC", GroupID: "g1", Kind: "TASK"},
		{ID: "p2", Name: "Books", GroupID: "g2", Kind: "TASK"},
		{ID: "p3", Name: "Inbox misc", GroupID: "NONE", Kind: "TASK"},
	}
	nodes := ProjectTree(gs, ps)
	if len(nodes) < 4 {
		t.Fatalf("got %d nodes", len(nodes))
	}
	out := FormatProjectTree(nodes)
	if !containsAll(out, "Work", "PXC", "Home", "Books") {
		t.Fatalf("tree missing entries:\n%s", out)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub)))
}

func stringContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
