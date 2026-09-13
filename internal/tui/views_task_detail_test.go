package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

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
