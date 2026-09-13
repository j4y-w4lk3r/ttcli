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
	if TaskSortTitle.Next() != TaskSortCustom {
		t.Fatal("title should wrap to custom")
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
