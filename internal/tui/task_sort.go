package tui

import (
	"sort"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func sortTasks(tasks []ticktick.Task, mode TaskSortMode) {
	sort.Slice(tasks, func(i, j int) bool {
		return compareTasks(tasks[i], tasks[j], mode)
	})
}

func compareTasks(a, b ticktick.Task, mode TaskSortMode) bool {
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
