package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestPaneLineStripsControlChars(t *testing.T) {
	m := fixtureModel(120, 40)
	m.tasks = []ticktick.Task{{ID: "1", Title: "broken\ntitle\rhere"}}
	m.projectName = "Home"
	m.paneFocus = paneTasks
	view := m.View()
	if strings.Contains(view, "broken\ntitle") {
		t.Fatalf("embedded newline leaked into view")
	}
	r := analyzeView(view, m.width, m.height, "tasks")
	if !r.OK() {
		t.Fatalf("frame broken:\n%s", r)
	}
}

func TestTaskScrollSubtasksFrameIntegrity(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "p1", Title: "One Time Setup"},
		{ID: "c1", Title: "su frisco", ParentID: "p1"},
		{ID: "c2", Title: "su ISP", ParentID: "p1", DueDate: "2026-09-19"},
		{ID: "c3", Title: "su culligan", ParentID: "p1"},
		{ID: "c4", Title: "su hairdresser 💇", ParentID: "p1", DueDate: "2026-09-19"},
		{ID: "t2", Title: "Tasks"},
		{ID: "g1", Title: "garbage", ParentID: "t2"},
		{ID: "g2", Title: "cleaning", ParentID: "t2"},
		{ID: "g3", Title: "groceries", ParentID: "t2"},
	}
	for i := 0; i < 30; i++ {
		tasks = append(tasks, ticktick.Task{ID: fmt.Sprintf("x%d", i), Title: fmt.Sprintf("extra task %d with emoji 💧", i)})
	}
	m := fixtureModel(200, 50)
	m.paneFocus = paneTasks
	m.projectName = "Home"
	m.tasks = tasks
	m.taskFocusByTitle = map[string]ticktick.TaskFocusSummary{
		ticktick.NormalizeFocusTaskTitle("su ISP"):   {FullSessions: 1, LoggedSessions: 1, TotalSeconds: 1500},
		ticktick.NormalizeFocusTaskTitle("garbage"):  {FullSessions: 4, LoggedSessions: 4, TotalSeconds: 6000},
		ticktick.NormalizeFocusTaskTitle("cleaning"): {FullSessions: 9, LoggedSessions: 9, TotalSeconds: 13500},
	}
	for i := 0; i < 60; i++ {
		m = pressKey(m, "j")
		r := analyzeView(m.View(), m.width, m.height, "tasks")
		if !r.OK() {
			t.Fatalf("after scroll %d:\n%s", i, r)
		}
	}
}

func TestListSwitchWhileScrollingFrameIntegrity(t *testing.T) {
	for _, size := range testTermSizes {
		w, h := size[0], size[1]
		t.Run(formatSize(w, h), func(t *testing.T) {
			m := fixtureModel(w, h)
			for i := 0; i < 8; i++ {
				m = pressKey(m, "j")
				assertViewOK(t, m, fmt.Sprintf("list switch %d", i))
				out, _ := m.Update(tasksLoadedMsg{
					projectID:   m.projectID,
					projectName: m.projectName,
					tasks:       m.tasks,
				})
				m = out.(model)
				assertViewOK(t, m, fmt.Sprintf("list loaded %d", i))
			}
		})
	}
}

func TestLargeTrashListScrollReusesTheBuiltRows(t *testing.T) {
	const n = 3000
	tasks := make([]ticktick.Task, n)
	for i := range tasks {
		tasks[i] = ticktick.Task{ID: fmt.Sprintf("bin-%d", i), Title: "trashed item", Deleted: 1}
	}
	start := time.Now()
	rows := buildVisibleTaskRowsForScope(tasks, TaskSortCustom, TaskScopeAll, "")
	if elapsed := time.Since(start); elapsed > 150*time.Millisecond {
		t.Fatalf("building %d trash rows took %s", n, elapsed)
	}
	if len(rows) != n {
		t.Fatalf("rows=%d", len(rows))
	}

	m := fixtureModel(100, 30)
	m.projectID = smartTrashID
	m.projectName = "Trash"
	m.paneFocus = paneTasks
	m.tasks = tasks
	if got := m.visibleTaskCount(); got != n {
		t.Fatalf("visible=%d", got)
	}
	start = time.Now()
	for i := 0; i < 40; i++ {
		m = pressKey(m, "j")
		if m.visibleTaskCount() != n {
			t.Fatalf("scroll %d lost rows", i)
		}
		_ = m.View()
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("40 trash scrolls took %s", elapsed)
	}
	if m.taskCursor != 40 {
		t.Fatalf("cursor=%d", m.taskCursor)
	}
}
