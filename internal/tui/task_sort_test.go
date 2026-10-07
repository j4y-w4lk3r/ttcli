package tui

import (
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestTaskSortPriority(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "1", Title: "low", Priority: 1},
		{ID: "2", Title: "high", Priority: 5},
		{ID: "3", Title: "none"},
	}
	sortTasksForProject(tasks, TaskSortPriority)
	if tasks[0].ID != "2" || tasks[1].ID != "1" || tasks[2].ID != "3" {
		t.Fatalf("order=%v %v %v", tasks[0].ID, tasks[1].ID, tasks[2].ID)
	}
}

func TestTaskSortTitle(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "1", Title: "Zebra"},
		{ID: "2", Title: "alpha"},
		{ID: "3", Title: "Beta"},
	}
	sortTasksForProject(tasks, TaskSortTitle)
	if tasks[0].Title != "alpha" || tasks[1].Title != "Beta" || tasks[2].Title != "Zebra" {
		t.Fatalf("order=%v %v %v", tasks[0].Title, tasks[1].Title, tasks[2].Title)
	}
}

func TestTaskSortModeNext(t *testing.T) {
	if TaskSortCustom.Next() != TaskSortDue {
		t.Fatal("custom should advance to due")
	}
	if TaskSortTitle.Next() != TaskSortList {
		t.Fatal("title should advance to list")
	}
	if TaskSortList.Next() != TaskSortCustom {
		t.Fatal("list should wrap to custom")
	}
}

func TestTaskSortListGroupsByListThenTitle(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "tech-done", Title: "alpha", ProjectID: "tech", Status: 2},
		{ID: "inbox-b", Title: "bravo", ProjectID: "inbox"},
		{ID: "tech-open", Title: "zeta", ProjectID: "tech"},
		{ID: "inbox-a", Title: "alpha", ProjectID: "inbox"},
		{ID: "child", Title: "aaa child", ProjectID: "tech", ParentID: "inbox-b"},
	}
	names := func(id string) string {
		switch id {
		case "inbox":
			return "Inbox"
		case "tech":
			return "tech"
		default:
			return ""
		}
	}
	rows := buildVisibleTaskRowsForScope(tasks, TaskSortList, TaskScopeAll, "", names)
	var got []string
	for _, row := range rows {
		got = append(got, row.Task.ID)
	}
	want := []string{"inbox-a", "inbox-b", "child", "tech-open", "tech-done"}
	if len(got) != len(want) {
		t.Fatalf("rows=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("rows=%v", got)
		}
	}
	if rows[2].Depth != 1 {
		t.Fatalf("child depth=%d", rows[2].Depth)
	}
}

func TestUISettingsSortForProject(t *testing.T) {
	s := uiSettings{
		TaskSortByProject: map[string]TaskSortMode{
			"p1": TaskSortTitle,
		},
	}
	if got := s.sortForProject("p1", "inbox"); got != TaskSortTitle {
		t.Fatalf("got %q", got)
	}
	if got := s.sortForProject("inbox", "inbox"); got != TaskSortDue {
		t.Fatalf("inbox default got %q", got)
	}
	if got := s.sortForProject("p2", "inbox"); got != TaskSortCustom {
		t.Fatalf("list default got %q", got)
	}
}
