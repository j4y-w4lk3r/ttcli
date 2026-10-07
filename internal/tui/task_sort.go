package tui

import (
	"sort"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func sortTasks(tasks []ticktick.Task, mode TaskSortMode) {
	sortTasksNamed(tasks, mode, nil)
}

func sortTasksNamed(tasks []ticktick.Task, mode TaskSortMode, listName func(string) string) {
	sort.Slice(tasks, func(i, j int) bool {
		return compareTasksNamed(tasks[i], tasks[j], mode, listName)
	})
}

func compareTasks(a, b ticktick.Task, mode TaskSortMode) bool {
	return compareTasksNamed(a, b, mode, nil)
}

func compareTasksNamed(a, b ticktick.Task, mode TaskSortMode, listName func(string) string) bool {
	if mode == TaskSortList {
		return compareTasksByList(a, b, listName)
	}
	da, db := a.Done(), b.Done()
	if da != db {
		return !da
	}
	switch mode {
	case TaskSortDue:
		return compareTasksByDue(a, b)
	case TaskSortPriority:
		return compareTasksByPriority(a, b)
	case TaskSortTitle:
		return compareTasksByTitle(a, b)
	default:
		if a.SortOrder != b.SortOrder {
			return a.SortOrder < b.SortOrder
		}
		return strings.ToLower(a.Title) < strings.ToLower(b.Title)
	}
}

func compareTasksByList(a, b ticktick.Task, listName func(string) string) bool {
	la, lb := listSortName(a, listName), listSortName(b, listName)
	if la != lb {
		return la < lb
	}
	if a.Done() != b.Done() {
		return !a.Done()
	}
	return compareTasksByTitle(a, b)
}

func listSortName(task ticktick.Task, listName func(string) string) string {
	name := ""
	if listName != nil {
		name = strings.TrimSpace(listName(task.ProjectID))
	}
	if name == "" {
		name = task.ProjectID
	}
	return strings.ToLower(name)
}

func compareTasksByPriority(a, b ticktick.Task) bool {
	pa, pb := a.Priority.Int(), b.Priority.Int()
	if pa != pb {
		return pa > pb
	}
	return compareTasksByDue(a, b)
}

func compareTasksByTitle(a, b ticktick.Task) bool {
	ta, tb := strings.ToLower(a.Title), strings.ToLower(b.Title)
	if ta != tb {
		return ta < tb
	}
	return a.ID < b.ID
}
