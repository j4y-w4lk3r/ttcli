package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestDueInlineShowsTime(t *testing.T) {
	plain := stripANSI(dueInlineTask(ticktick.Task{
		DueDate: "2026-03-08T14:30:00.000+0200",
	}))
	if !strings.Contains(plain, "08/03/2026") || !strings.Contains(plain, "14:30") {
		t.Fatalf("got %q", plain)
	}
	allDay := stripANSI(dueInlineTask(ticktick.Task{
		DueDate:  "2026-03-08",
		IsAllDay: true,
	}))
	if strings.Contains(allDay, "14:") {
		t.Fatalf("all-day should not show time: %q", allDay)
	}
}

func TestSortTasksForProjectInbox(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "1", Title: "no due", SortOrder: 1},
		{ID: "2", Title: "later", DueDate: "2026-12-01T09:00:00", SortOrder: 2},
		{ID: "3", Title: "soon", DueDate: "2026-03-08T10:00:00", SortOrder: 3},
		{ID: "4", Title: "done", DueDate: "2026-03-01", Status: 2, SortOrder: 4},
	}
	sortTasksForProject(tasks, TaskSortDue)
	if tasks[0].ID != "3" || tasks[1].ID != "2" || tasks[2].ID != "1" {
		t.Fatalf("open order got %v %v %v", tasks[0].ID, tasks[1].ID, tasks[2].ID)
	}
	if tasks[3].ID != "4" {
		t.Fatalf("done task last, got %s", tasks[3].ID)
	}

	other := []ticktick.Task{
		{ID: "a", SortOrder: 5},
		{ID: "b", SortOrder: 1},
	}
	sortTasksForProject(other, TaskSortCustom)
	if other[0].ID != "b" {
		t.Fatalf("non-inbox keeps sortOrder")
	}
}

func TestFormatDueDMY(t *testing.T) {
	if got := formatDueDMY("2026-03-08T10:00:00"); got != "08/03/2026" {
		t.Fatalf("got %q", got)
	}
	if formatDueDMY("") != "" {
		t.Fatal("empty due should be blank")
	}
}

func TestFormatDueDMYLocalMidnightUTC(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	// Nov 1 2026 00:00 in Warsaw is stored as Oct 31 23:00 UTC.
	raw := "2026-10-31T23:00:00.000+0000"
	if got := formatDueDMY(raw); got != "01/11/2026" {
		t.Fatalf("got %q want 01/11/2026", got)
	}
}

func TestDueColumnFixedWidth(t *testing.T) {
	cases := []string{
		"",
		"2026-03-08",
		"2020-01-15",
		"2026-12-31",
	}
	for _, raw := range cases {
		col := dueColumn(raw)
		if lipgloss.Width(col) != dueColWidth {
			t.Fatalf("raw=%q width=%d want %d %q", raw, lipgloss.Width(col), dueColWidth, col)
		}
	}
}

func TestDueColumnOverdueRedDMY(t *testing.T) {
	past := time.Now().AddDate(0, 0, -3).Format("2006-01-02")
	col := dueColumn(past)
	plain := stripANSI(col)
	if !strings.Contains(plain, "/") {
		t.Fatalf("expected dd/mm/yyyy, got %q", plain)
	}
	if strings.Contains(plain, "-") {
		t.Fatalf("expected slashes not ISO date, got %q", plain)
	}
}

func TestFormatTaskLineAlignsDates(t *testing.T) {
	m := fixtureModel(120, 40)
	w := 60
	tasks := []ticktick.Task{
		{Title: "Short", DueDate: "2026-03-08"},
		{Title: "A much longer task name that should truncate nicely", DueDate: "2020-06-15"},
		{Title: "No date"},
	}
	var dueCols []string
	for i, task := range tasks {
		layout := computeTaskRowLayout([]taskListRow{{Task: task}}, w, 0)
		line := m.formatTaskLine(task, 0, i == 0, false, w, layout)
		if lipgloss.Width(line) > w {
			t.Fatalf("line %d width=%d exceeds %d", i, lipgloss.Width(line), w)
		}
		dueCols = append(dueCols, dueInlineTask(task))
	}
	for i, c := range dueCols {
		if c == "" {
			continue
		}
		if lipgloss.Width(c) > dueColWidth+2 {
			t.Fatalf("due col %d too wide %d", i, lipgloss.Width(c))
		}
	}
}
