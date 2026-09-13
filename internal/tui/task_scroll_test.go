package tui

import (
	"fmt"
	"strings"
	"testing"

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
