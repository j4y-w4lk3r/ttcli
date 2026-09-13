package tui

import (
	"sort"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type taskListRow struct {
	Task  ticktick.Task
	Depth int
}

func dedupeTasks(tasks []ticktick.Task) []ticktick.Task {
	if len(tasks) < 2 {
		return tasks
	}
	seen := make(map[string]bool, len(tasks))
	out := make([]ticktick.Task, 0, len(tasks))
	for _, t := range tasks {
		if t.ID == "" {
			out = append(out, t)
			continue
		}
		if seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		out = append(out, t)
	}
	return out
}

func orderedSubtasks(parent ticktick.Task, tasks []ticktick.Task) []ticktick.Task {
	byID := make(map[string]ticktick.Task, len(tasks))
	for _, t := range tasks {
		byID[t.ID] = t
	}
	seen := map[string]bool{}
	var out []ticktick.Task
	for _, id := range parent.ChildIDs {
		if t, ok := byID[id]; ok && t.ParentID == parent.ID {
			out = append(out, t)
			seen[id] = true
		}
	}
	var rest []ticktick.Task
	for _, t := range tasks {
		if t.ParentID == parent.ID && !seen[t.ID] {
			rest = append(rest, t)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].SortOrder != rest[j].SortOrder {
			return rest[i].SortOrder < rest[j].SortOrder
		}
		return rest[i].Title < rest[j].Title
	})
	out = append(out, rest...)
	return out
}

func taskRowVisible(t ticktick.Task, showCompleted, showDeleted bool, filter string) bool {
	if t.Trashed() && !showDeleted {
		return false
	}
	if t.Done() && !showCompleted {
		return false
	}
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	return strings.Contains(strings.ToLower(t.Title), filter)
}

func subtreeHasVisibleRow(t ticktick.Task, tasks []ticktick.Task, showCompleted, showDeleted bool, filter string) bool {
	if taskRowVisible(t, showCompleted, showDeleted, filter) {
		return true
	}
	for _, child := range orderedSubtasks(t, tasks) {
		if subtreeHasVisibleRow(child, tasks, showCompleted, showDeleted, filter) {
			return true
		}
	}
	return false
}

func appendVisibleTaskTree(
	parent ticktick.Task,
	depth int,
	tasks []ticktick.Task,
	showCompleted bool,
	showDeleted bool,
	filter string,
	out *[]taskListRow,
	placed map[string]bool,
) {
	if depth == 0 {
		if !subtreeHasVisibleRow(parent, tasks, showCompleted, showDeleted, filter) {
			return
		}
		if placed[parent.ID] {
			return
		}
		placed[parent.ID] = true
		*out = append(*out, taskListRow{Task: parent, Depth: 0})
	} else {
		if !taskRowVisible(parent, showCompleted, showDeleted, filter) {
			return
		}
		if placed[parent.ID] {
			return
		}
		placed[parent.ID] = true
		*out = append(*out, taskListRow{Task: parent, Depth: depth})
	}
	for _, child := range orderedSubtasks(parent, tasks) {
		appendVisibleTaskTree(child, depth+1, tasks, showCompleted, showDeleted, filter, out, placed)
	}
}

func buildVisibleTaskRows(tasks []ticktick.Task, sortMode TaskSortMode, showCompleted, showDeleted bool, filter string) []taskListRow {
	seen := make(map[string]bool, len(tasks))
	var roots []ticktick.Task
	for _, t := range tasks {
		if t.IsSubtask() || t.ID == "" || seen[t.ID] {
			continue
		}
		seen[t.ID] = true
		roots = append(roots, t)
	}
	sortTasksForProject(roots, sortMode)

	var out []taskListRow
	placed := make(map[string]bool, len(roots))
	for _, root := range roots {
		appendVisibleTaskTree(root, 0, tasks, showCompleted, showDeleted, filter, &out, placed)
	}
	return out
}

func taskTreePrefix(depth int) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat("  ", depth-1) + "├─ "
}
