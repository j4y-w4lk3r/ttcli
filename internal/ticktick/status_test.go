package ticktick

import "testing"

func TestOpenProjects(t *testing.T) {
	closed := true
	ps := []Project{
		{ID: "1", Name: "Open", Kind: "TASK"},
		{ID: "2", Name: "Closed", Kind: "TASK", Closed: &closed},
		{ID: "3", Name: "Notes", Kind: "NOTE"},
	}
	open := OpenProjects(ps, false)
	if len(open) != 1 || open[0].Name != "Open" {
		t.Fatalf("got %+v", open)
	}
	all := OpenProjects(ps, true)
	if len(all) != 2 {
		t.Fatalf("expected 2 with notes, got %d", len(all))
	}
}
