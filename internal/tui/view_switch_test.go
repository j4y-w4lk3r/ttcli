package tui

import (
	"fmt"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// Terminal sizes to regression-test (wide tmux panes, laptop, narrow).
var testTermSizes = [][2]int{
	{120, 40},
	{160, 45},
	{91, 24},
	{80, 24},
	{200, 50},
}

func TestViewSwitchTasksCalendarTasks(t *testing.T) {
	for _, size := range testTermSizes {
		w, h := size[0], size[1]
		t.Run(formatSize(w, h), func(t *testing.T) {
			m := fixtureModel(w, h)
			assertViewOK(t, m, "initial tasks")

			m = switchToView(m, viewCalendar)
			assertViewOK(t, m, "calendar")

			m = switchToView(m, viewTasks)
			assertViewOK(t, m, "tasks after calendar")

			// second round-trip
			m = switchToView(m, viewCalendar)
			m = switchToView(m, viewTasks)
			assertViewOK(t, m, "tasks after 2nd round-trip")
		})
	}
}

func TestViewSwitchAllViews(t *testing.T) {
	w, h := 120, 40
	m := fixtureModel(w, h)
	views := []appView{viewTasks, viewCalendar, viewPomodoro, viewHabits, viewTasks}
	names := []string{"tasks", "calendar", "pomo", "habits", "tasks"}
	for i, v := range views {
		m = switchToView(m, v)
		assertViewOK(t, m, names[i])
	}
}

func TestViewSwitchViaKeybindings(t *testing.T) {
	w, h := 120, 40
	m := fixtureModel(w, h)
	assertViewOK(t, m, "start")

	m = pressKey(m, "2")
	m.loading = false
	m.calTasks = fixtureModel(w, h).calTasks
	assertViewOK(t, m, "key 2 calendar")

	m = pressKey(m, "1")
	assertViewOK(t, m, "key 1 tasks")
}

func TestListSwitchClearsAndReloadsTasks(t *testing.T) {
	w, h := 120, 40
	m := fixtureModel(w, h)
	m.paneFocus = paneLists
	assertViewOK(t, m, "before list switch")

	m = pressKey(m, "j")
	row, ok := m.currentListRow()
	if !ok {
		t.Fatal("no list row")
	}
	out, _ := m.Update(tasksLoadedMsg{
		projectID:   row.node.ID,
		projectName: row.node.Name,
		tasks: []ticktick.Task{
			{ID: "x1", Title: "reloaded task", DueDate: "2026-08-30"},
		},
	})
	m = out.(model)
	assertViewOK(t, m, "after list switch load")
	if m.projectName == "List0" && len(m.tasks) == 20 {
		t.Fatal("expected different list after j")
	}
}

func TestWindowResizeMidSession(t *testing.T) {
	m := fixtureModel(120, 40)
	m = switchToView(m, viewCalendar)
	m = applyWindowSize(m, 91, 24)
	assertViewOK(t, m, "calendar resized")
	m = switchToView(m, viewTasks)
	assertViewOK(t, m, "tasks after resize")
}

func TestViewFrameReportDetectsBadWidth(t *testing.T) {
	// Deliberately too-wide line should fail analysis.
	view := "hello\n" + stringsRepeat("x", 200)
	r := analyzeView(view, 80, 2, "tasks")
	if r.OK() {
		t.Fatal("expected width errors")
	}
	if len(r.WidthErrors) == 0 {
		t.Fatal("expected width error details")
	}
}

func TestViewSwitchRegressionDump(t *testing.T) {
	if testing.Short() {
		t.Skip("verbose dump")
	}
	w, h := 160, 45
	m := fixtureModel(w, h)
	dumpView(t, m, "tasks initial")
	m = switchToView(m, viewCalendar)
	dumpView(t, m, "calendar")
	m = switchToView(m, viewTasks)
	dumpView(t, m, "tasks return")
}

func formatSize(w, h int) string {
	return fmt.Sprintf("%dx%d", w, h)
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
