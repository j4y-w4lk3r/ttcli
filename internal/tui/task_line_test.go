package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestFormatTaskLineKeepsShortTitleWithFocusColumn(t *testing.T) {
	m := fixtureModel(120, 40)
	m.taskFocusByID = map[string]ticktick.TaskFocusSummary{
		"garbage": {FullSessions: 4, LoggedSessions: 4, TotalSeconds: 6000},
	}
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "One Time Setup"}},
		{Task: ticktick.Task{ID: "garbage", Title: "Garbage"}},
	}
	focusColW := maxTaskFocusInlineW(m, rows)
	layout := computeTaskRowLayout(rows, w, focusColW)
	line := m.formatTaskLine(rows[0].Task, 0, false, false, w, layout)
	plain := stripANSI(line)
	if strings.Contains(plain, "One Time S…") {
		t.Fatalf("short title over-truncated: %q", plain)
	}
	if !strings.Contains(plain, "One Time Setup") {
		t.Fatalf("expected full title, got %q", plain)
	}
	if lipgloss.Width(line) > w {
		t.Fatalf("line too wide: %d > %d", lipgloss.Width(line), w)
	}
}

func TestParentTaskUsesTheWontDoColor(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(100, 30)
	m.tasks = []ticktick.Task{
		{ID: "p", Title: "Parent"},
		{ID: "c", Title: "Child", ParentID: "p"},
	}
	layout := taskRowLayout{TitleColW: 24, SuffixCol: 28}
	parent := m.formatTaskLine(m.tasks[0], 0, false, false, 80, layout)
	child := m.formatTaskLine(m.tasks[1], 1, false, false, 80, layout)
	want := foregroundSeq(taskWontStyle.Render("Parent"))
	if foregroundSeq(parent) != want || strings.Contains(parent, "48;2;") {
		t.Fatalf("parent %q", parent)
	}
	if foregroundSeq(child) == want {
		t.Fatalf("child picked up the parent color: %q", child)
	}
}

func TestParentListingAnotherParentsChildIsNotYellow(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(100, 30)
	m.tasks = []ticktick.Task{
		{ID: "fm0", Title: "fm0", ChildIDs: []string{"amazon"}},
		{ID: "finance", Title: "Parent", ChildIDs: []string{"amazon"}},
		{ID: "amazon", Title: "Amazon", ParentID: "finance"},
	}
	layout := taskRowLayout{TitleColW: 24, SuffixCol: 28}
	fm0 := m.formatTaskLine(m.tasks[0], 0, false, false, 80, layout)
	finance := m.formatTaskLine(m.tasks[1], 0, false, false, 80, layout)
	want := foregroundSeq(taskWontStyle.Render("Parent"))
	if foregroundSeq(finance) != want {
		t.Fatalf("finance %q", finance)
	}
	if foregroundSeq(fm0) == want {
		t.Fatalf("fm0 stayed yellow: %q", fm0)
	}
}

func TestSelectedRowPaintsThePomoCountWithTheTitle(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(120, 40)
	task := ticktick.Task{ID: "t", Title: "wake up"}
	layout := taskRowLayout{TitleColW: 24, SuffixCol: 40, FocusColW: 22}
	line := m.formatTaskLine(task, 0, true, false, 100, layout)
	plain := stripANSI(line)
	if strings.Count(line, "48;2;") != 1 || !strings.Contains(plain, "wake up") || !strings.Contains(plain, "0/1") {
		t.Fatalf("line %q plain %q", line, plain)
	}
	if lipgloss.Width(plain) != 100 {
		t.Fatalf("width %d", lipgloss.Width(plain))
	}
}

func TestForeignParentRowNamesItsList(t *testing.T) {
	m := fixtureModel(120, 40)
	m.projectID = "p0"
	m.inboxID = "inboxfixture"
	task := ticktick.Task{ID: "parent", Title: "Parent", ProjectID: "inboxfixture", Status: -1}
	line := stripANSI(m.formatTaskLine(task, 0, false, false, 80, taskRowLayout{TitleColW: 24, SuffixCol: 28}))
	if !strings.Contains(line, "Parent") || !strings.Contains(line, "Inbox") {
		t.Fatalf("line %q", line)
	}
}

func TestDueInlineConsistentWidth(t *testing.T) {
	future := dueInlineTask(ticktick.Task{DueDate: "2026-09-02T04:30:00.000+0200"})
	past := dueInlineTask(ticktick.Task{DueDate: "2020-01-01T04:30:00.000+0200"})
	if lipgloss.Width(future) != dueColWidth || lipgloss.Width(past) != dueColWidth {
		t.Fatalf("widths differ future=%d past=%d want %d", lipgloss.Width(future), lipgloss.Width(past), dueColWidth)
	}
	if !strings.Contains(stripANSI(future), "●") {
		t.Fatalf("future due should include marker: %q", stripANSI(future))
	}
}

func TestFormatTaskLineAlignsOverdueAndFuture(t *testing.T) {
	m := fixtureModel(120, 40)
	w := 80
	rows := []taskListRow{
		{Task: ticktick.Task{Title: "Overdue", DueDate: "2020-01-01T08:00:00.000+0200"}},
		{Task: ticktick.Task{Title: "Future", DueDate: "2026-09-02T04:30:00.000+0200"}},
	}
	layout := computeTaskRowLayout(rows, w, 0)
	a := dueColStart(m.formatTaskLine(rows[0].Task, 0, false, false, w, layout))
	b := dueColStart(m.formatTaskLine(rows[1].Task, 0, false, false, w, layout))
	if a < 0 || b < 0 || a != b {
		t.Fatalf("due markers misaligned: %d vs %d", a, b)
	}
}

func TestMaxVisibleRowsIsForty(t *testing.T) {
	if maxVisibleRows != 40 {
		t.Fatalf("maxVisibleRows=%d want 40", maxVisibleRows)
	}
}
