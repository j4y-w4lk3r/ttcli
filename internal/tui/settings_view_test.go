package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestParseAppView(t *testing.T) {
	cases := map[string]appView{
		"":         viewTasks,
		"tasks":    viewTasks,
		"calendar": viewCalendar,
		"pomo":     viewPomodoro,
		"habits":   viewHabits,
		"bogus":    viewTasks,
	}
	for in, want := range cases {
		if got := parseAppView(in); got != want {
			t.Fatalf("parseAppView(%q)=%v want %v", in, got, want)
		}
	}
}

func TestAppViewNameRoundTrip(t *testing.T) {
	for _, v := range []appView{viewTasks, viewCalendar, viewPomodoro, viewHabits} {
		if parseAppView(appViewName(v)) != v {
			t.Fatalf("round trip failed for %v", v)
		}
	}
}

func TestTaskDetailLayoutToggle(t *testing.T) {
	if TaskDetailBottom.Toggle() != TaskDetailSide {
		t.Fatal("bottom should toggle to side")
	}
	if TaskDetailSide.Toggle() != TaskDetailBottom {
		t.Fatal("side should toggle to bottom")
	}
}

func TestTaskDetailSplitWidths(t *testing.T) {
	listW, detailW := taskDetailSplitWidths(120)
	if listW+detailW+1 != 120 {
		t.Fatalf("split widths don't sum: %d+%d+1 != 120", listW, detailW)
	}
	l, d := taskDetailSplitWidths(60)
	if l != 0 || d != 0 {
		t.Fatalf("narrow right pane should not split: %d %d", l, d)
	}
}

func TestCalDrillDownMonthToDay(t *testing.T) {
	m := fixtureModel(120, 40)
	m.view = viewCalendar
	m.calMode = calModeMonth
	m.calDate = mustParseDate("2026-09-03")
	out := m.calDrillDown()
	if out.calMode != calModeDay {
		t.Fatalf("mode=%v want day", out.calMode)
	}
	if out.toast == "" {
		t.Fatal("expected toast when opening day view")
	}
}

func TestTaskDetailSideLayoutWideFrame(t *testing.T) {
	tasks := make([]ticktick.Task, 0, 20)
	for i := 0; i < 20; i++ {
		tasks = append(tasks, ticktick.Task{
			ID:      fmt.Sprintf("t%d", i),
			Title:   fmt.Sprintf("task %d", i),
			DueDate: "2026-07-01",
			Content: "Note line one. Note line two with more detail about the task.",
		})
	}
	tasks[0].Content = strings.Repeat("Long note content. ", 20)

	m := fixtureModel(200, 50)
	m.uiSettings.TaskDetailLayout = TaskDetailSide
	m.projectName = "Inbox"
	m.inboxID = "inbox"
	m.projectID = "inbox"
	m.tasks = dedupeTasks(tasks)
	m.paneFocus = paneTasks
	m.taskCursor = 0

	for i := 0; i < 15; i++ {
		r := analyzeView(m.View(), m.width, m.height, "tasks")
		if !r.OK() {
			t.Fatalf("scroll %d:\n%s", i, r)
		}
		m = pressKey(m, "j")
	}
}

func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}
