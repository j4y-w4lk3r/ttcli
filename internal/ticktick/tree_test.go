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

func TestProjectTree_dedupesDuplicateLists(t *testing.T) {
	gs := []ProjectGroup{
		{ID: "g1", Name: "x", SortOrder: 1},
		{ID: "g1", Name: "x", SortOrder: 1}, // duplicate folder row from API
		{ID: "g2", Name: "Y", SortOrder: 2},
	}
	ps := []Project{
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK", SortOrder: 1},
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK", SortOrder: 1}, // dup list
		{ID: "p2", Name: "PXC", GroupID: "g2", Kind: "TASK", SortOrder: 2},
		{ID: "p3", Name: "List0", GroupID: "NONE", Kind: "TASK", SortOrder: 3},
		{ID: "p3", Name: "List0", GroupID: "g1", Kind: "TASK", SortOrder: 3}, // grouped wins
	}
	nodes := ProjectTree(gs, ps)
	ids := ListIDsInTree(nodes)
	if len(ids) != len(uniqueStrings(ids)) {
		t.Fatalf("duplicate list ids in tree: %v", ids)
	}
	want := []string{"p1", "p3", "p2"}
	if len(ids) != len(want) {
		t.Fatalf("got %d lists %v, want %d", len(ids), ids, len(want))
	}
	for _, id := range want {
		if !containsString(ids, id) {
			t.Fatalf("missing %s in %v", id, ids)
		}
	}
}

func TestProjectTreeWithInbox(t *testing.T) {
	gs := []ProjectGroup{{ID: "g1", Name: "Work", SortOrder: 1}}
	ps := []Project{{ID: "p1", Name: "PXC", GroupID: "g1", Kind: "TASK"}}
	nodes := ProjectTreeWithInbox("inbox123", gs, ps)
	if len(nodes) < 3 {
		t.Fatalf("got %d nodes", len(nodes))
	}
	if nodes[0].Kind != "list" || nodes[0].Name != "Inbox" || nodes[0].ID != "inbox123" {
		t.Fatalf("first node should be Inbox, got %+v", nodes[0])
	}
	// If inbox already in tree, do not duplicate.
	psWithInbox := append([]Project{{ID: "inbox123", Name: "My Inbox", GroupID: "NONE"}}, ps...)
	nodes2 := ProjectTreeWithInbox("inbox123", gs, psWithInbox)
	if len(nodes2) != len(ProjectTree(gs, psWithInbox)) {
		t.Fatalf("should not duplicate inbox when already present")
	}
}

func TestProjectTree_orphanGroupOnce(t *testing.T) {
	gs := []ProjectGroup{{ID: "g1", Name: "x", SortOrder: 1}}
	ps := []Project{
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
		{ID: "p2", Name: "Lost", GroupID: "gone-group", Kind: "TASK"},
	}
	nodes := ProjectTree(gs, ps)
	ids := ListIDsInTree(nodes)
	if len(ids) != 2 || !containsString(ids, "p2") {
		t.Fatalf("orphan list missing: %v", ids)
	}
}

func uniqueStrings(ss []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range ss {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func containsString(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
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
