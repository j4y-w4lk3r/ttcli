package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestWontDoDetailIsNotLabeledCompleted(t *testing.T) {
	lines := taskDetailPanel(ticktick.Task{ID: "skip", Title: "ESP32", Status: -1}, func(ticktick.Task) (ticktick.TaskFocusSummary, bool) {
		return ticktick.TaskFocusSummary{}, false
	}, 40, 8)
	text := stripANSI(strings.Join(lines, "\n"))
	if strings.Contains(text, "completed") || !strings.Contains(text, "won't do") {
		t.Fatalf("detail=%q", text)
	}
}

func TestBuildVisibleTaskRowsDedupesByID(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "a1", Title: "add the Disney + to the fm", DueDate: "2026-08-05"},
		{ID: "a1", Title: "add the Disney + to the fm", DueDate: "2026-08-05"},
		{ID: "b1", Title: "Culligan Water 💧", DueDate: "2026-08-05T05:15:00"},
	}
	rows := buildVisibleTaskRows(tasks, TaskSortDue, false, false, "")
	if len(rows) != 2 {
		t.Fatalf("want 2 rows after dedupe, got %d", len(rows))
	}
}

func TestInboxScrollWithLongNotesStableLayout(t *testing.T) {
	morningNote := strings.Repeat("Up to Date Homebrew npm gcloud monring fix.sh ffmpeg typingclub finance mail medium ", 8)
	tasks := make([]ticktick.Task, 0, 96)
	for i := 0; i < 96; i++ {
		tasks = append(tasks, ticktick.Task{
			ID:      fmt.Sprintf("t%d", i),
			Title:   fmt.Sprintf("task %d", i),
			DueDate: "2026-07-01",
		})
	}
	tasks[0] = ticktick.Task{
		ID:      "morning",
		Title:   "Morning",
		DueDate: "2026-02-22T08:00:00",
		Content: morningNote,
	}
	tasks[25] = ticktick.Task{ID: "d1", Title: "add the Disney + to the fm", DueDate: "2026-08-05"}
	tasks[26] = ticktick.Task{ID: "d1", Title: "add the Disney + to the fm", DueDate: "2026-08-05"}

	m := fixtureModel(200, 50)
	m.projectName = "Inbox"
	m.inboxID = "inbox"
	m.projectID = "inbox"
	m.tasks = dedupeTasks(tasks)
	m.paneFocus = paneTasks
	m.taskCursor = 0

	var prev string
	for i := 0; i < 50; i++ {
		view := m.View()
		r := analyzeView(view, m.width, m.height, "tasks")
		if !r.OK() {
			t.Fatalf("scroll %d frame invalid:\n%s", i, r)
		}
		if strings.Count(view, "Morning") > strings.Count(stripANSI(prev), "Morning")+1 && i > 0 {
			// allow one Morning in list + mentions in notes, not stacked duplicates in task rows
			morningRows := 0
			for _, line := range r.Lines {
				plain := stripANSI(line)
				if strings.Contains(plain, "Morning") && strings.Contains(plain, "22/02/2026") {
					morningRows++
				}
			}
			if morningRows > 1 {
				t.Fatalf("scroll %d: duplicate Morning task rows (%d)", i, morningRows)
			}
		}
		prev = view
		m = pressKey(m, "j")
	}
}

func TestPadFooterLinesFixedHeight(t *testing.T) {
	footer := []string{"a", "b"}
	padded := padFooterLines(footer, 5)
	if len(padded) != 5 {
		t.Fatalf("len=%d want 5", len(padded))
	}
	if padded[0] != "a" || padded[1] != "b" || padded[2] != "" {
		t.Fatalf("unexpected pad: %q", padded)
	}
}

func TestInboxDetailPanelWideFrame(t *testing.T) {
	tasks := make([]ticktick.Task, 0, 96)
	for i := 0; i < 96; i++ {
		tasks = append(tasks, ticktick.Task{
			ID:      fmt.Sprintf("t%d", i),
			Title:   fmt.Sprintf("task %d", i),
			DueDate: "2026-07-01",
		})
	}
	tasks[25] = ticktick.Task{ID: "dup", Title: "add the Disney + to the fm", DueDate: "2026-08-05"}
	tasks[26] = ticktick.Task{ID: "dup", Title: "add the Disney + to the fm", DueDate: "2026-08-05"}
	tasks[27] = ticktick.Task{ID: "hair", Title: "Haircut️", DueDate: "2026-08-01T07:30:00"}
	tasks[0] = ticktick.Task{
		ID:      "morning",
		Title:   "Morning",
		DueDate: "2026-02-22T08:00:00",
		Content: "1. Up to Date (Homebrew, npm, gcloud) in the monring 2. fix.sh to manually type",
	}

	m := fixtureModel(200, 50)
	m.projectName = "Inbox"
	m.inboxID = "inbox"
	m.projectID = "inbox"
	m.tasks = dedupeTasks(tasks)
	m.paneFocus = paneTasks
	m.taskCursor = 0

	for i := 0; i < 40; i++ {
		r := analyzeView(m.View(), m.width, m.height, "tasks")
		if !r.OK() {
			t.Fatalf("scroll %d:\n%s", i, r)
		}
		m = pressKey(m, "j")
	}
}

func TestVisibleTaskRowsIncludesSubtasks(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "p1", Title: "Purchase", ChildIDs: []string{"c1", "c2"}},
		{ID: "c1", Title: "Antena", ParentID: "p1"},
		{ID: "c2", Title: "Phone", ParentID: "p1"},
	}
	rows := buildVisibleTaskRows(tasks, TaskSortCustom, true, false, "")
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0].Task.ID != "p1" || rows[0].Depth != 0 {
		t.Fatalf("row0=%+v", rows[0])
	}
	if rows[1].Depth != 1 || rows[2].Depth != 1 {
		t.Fatalf("expected subtask depth 1: %+v", rows[1:])
	}
}

func TestOpenScopeKeepsAWontDoParentAboveItsChild(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "root", Title: "Zebra", ProjectID: "tech"},
		{ID: "parent", Title: "Parent", Status: -1, ProjectID: "inbox"},
		{ID: "child", Title: "Child item", ParentID: "parent", ProjectID: "tech"},
	}
	rows := buildVisibleTaskRowsForScope(tasks, TaskSortTitle, TaskScopeOpen, "", nil)
	if len(rows) != 3 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].Task.ID != "parent" || rows[0].Depth != 0 || rows[1].Task.ID != "child" || rows[1].Depth != 1 {
		t.Fatalf("tree=%+v %+v %+v", rows[0], rows[1], rows[2])
	}
	if rows[2].Task.ID != "root" || rows[2].Depth != 0 {
		t.Fatalf("root=%+v", rows[2])
	}
}

func TestParentChildIDsNestTasksWithAnEmptyParentField(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "parent", Title: "alpha", ProjectID: "prio", ChildIDs: []string{"uber", "code", "cull"}},
		{ID: "uber", Title: "child a", ProjectID: "tech"},
		{ID: "code", Title: "child b", ProjectID: "tech"},
		{ID: "cull", Title: "done child", ProjectID: "tech", Status: 2},
		{ID: "other", Title: "zeta", ProjectID: "purchase"},
	}
	rows := buildVisibleTaskRowsForScope(tasks, TaskSortTitle, TaskScopeAll, "", nil)
	if len(rows) != 5 {
		t.Fatalf("rows=%d %+v", len(rows), rows)
	}
	want := []struct {
		id    string
		depth int
	}{
		{"parent", 0},
		{"uber", 1},
		{"code", 1},
		{"cull", 1},
		{"other", 0},
	}
	for i, row := range want {
		if rows[i].Task.ID != row.id || rows[i].Depth != row.depth {
			t.Fatalf("row %d = %s depth %d", i, rows[i].Task.ID, rows[i].Depth)
		}
	}
	open := buildVisibleTaskRowsForScope(tasks, TaskSortTitle, TaskScopeOpen, "", nil)
	if len(open) != 4 || open[0].Task.ID != "parent" || open[3].Task.ID != "other" {
		t.Fatalf("open=%+v", open)
	}
	for _, row := range open {
		if row.Task.ID == "cull" {
			t.Fatal("completed child should stay hidden in Open")
		}
	}
}

func TestChildIDsDoNotStealATaskThatNamesAnotherParent(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "fm0", Title: "fm0", ChildIDs: []string{"amazon", "domain", "gone"}},
		{ID: "finance", Title: "alpha", ChildIDs: []string{"amazon", "domain"}},
		{ID: "amazon", Title: "Amazon", ParentID: "finance", ProjectID: "fm"},
		{ID: "domain", Title: "domain", ParentID: "finance", ProjectID: "tech"},
	}
	rows := buildVisibleTaskRowsForScope(tasks, TaskSortTitle, TaskScopeAll, "", nil)
	var got []string
	for _, row := range rows {
		got = append(got, fmt.Sprintf("%s:%d", row.Task.ID, row.Depth))
	}
	want := []string{"finance:0", "amazon:1", "domain:1", "fm0:0"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("rows=%v", got)
	}
}

func TestStaleChildListingIsTheParentThatDoesNotMatch(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "fm0", ChildIDs: []string{"amazon", "note"}},
		{ID: "finance", ChildIDs: []string{"amazon"}},
		{ID: "amazon", ParentID: "finance"},
		{ID: "note"},
	}
	stale := staleChildListings(tasks)
	if len(stale["fm0"]) != 1 || stale["fm0"][0] != "amazon" || len(stale["finance"]) != 0 {
		t.Fatalf("stale=%v", stale)
	}
	keep := keepLoadedChildren(tasks[:1], []ticktick.Task{
		{ID: "amazon", ParentID: "finance"},
		{ID: "note"},
	})
	if len(keep) != 1 || keep[0].ID != "note" {
		t.Fatalf("keep=%v", keep)
	}
}

func TestOrderedSubtasksUsesChildIDs(t *testing.T) {
	parent := ticktick.Task{ID: "p1", ChildIDs: []string{"c2", "c1"}}
	tasks := []ticktick.Task{
		parent,
		{ID: "c1", Title: "A", ParentID: "p1"},
		{ID: "c2", Title: "B", ParentID: "p1"},
	}
	got := orderedSubtasks(parent, tasks)
	if len(got) != 2 || got[0].ID != "c2" || got[1].ID != "c1" {
		t.Fatalf("order=%v", []string{got[0].ID, got[1].ID})
	}
}

func TestStripTaskHTML(t *testing.T) {
	got := stripTaskHTML("hello<br/>world")
	if got == "" {
		t.Fatal("expected content")
	}
}

func TestTaskNotesHTMLRoundTripPreservesNewlines(t *testing.T) {
	plain := "first line\nsecond line"
	encoded := taskNotesToHTML(plain)
	if encoded != "first line<br/>second line" {
		t.Fatalf("encoded=%q", encoded)
	}
	if decoded := stripTaskHTML(encoded); decoded != plain {
		t.Fatalf("decoded=%q want %q", decoded, plain)
	}
}
