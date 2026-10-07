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

type taskChildren struct {
	byID map[string]ticktick.Task
	kids map[string][]ticktick.Task
}

func indexTaskChildren(tasks []ticktick.Task) taskChildren {
	idx := taskChildren{
		byID: make(map[string]ticktick.Task, len(tasks)),
		kids: make(map[string][]ticktick.Task),
	}
	for _, task := range tasks {
		if task.ID != "" {
			idx.byID[task.ID] = task
		}
		if task.ParentID != "" {
			idx.kids[task.ParentID] = append(idx.kids[task.ParentID], task)
		}
	}
	return idx
}

func orderedSubtasks(parent ticktick.Task, tasks []ticktick.Task) []ticktick.Task {
	return indexTaskChildren(tasks).ordered(parent)
}

func (idx taskChildren) ordered(parent ticktick.Task) []ticktick.Task {
	kids := idx.kids[parent.ID]
	if len(kids) == 0 && len(parent.ChildIDs) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(parent.ChildIDs))
	var out []ticktick.Task
	for _, id := range parent.ChildIDs {
		task, ok := idx.byID[id]
		if !ok || task.ParentID != parent.ID {
			continue
		}
		out = append(out, task)
		seen[id] = true
	}
	var rest []ticktick.Task
	for _, task := range kids {
		if !seen[task.ID] {
			rest = append(rest, task)
		}
	}
	sort.Slice(rest, func(i, j int) bool {
		if rest[i].SortOrder != rest[j].SortOrder {
			return rest[i].SortOrder < rest[j].SortOrder
		}
		return rest[i].Title < rest[j].Title
	})
	return append(out, rest...)
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
	return taskMatchesFilter(t, filter)
}

func taskMatchesFilter(t ticktick.Task, filter string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if filter == "" {
		return true
	}
	haystack := strings.ToLower(strings.Join(
		[]string{t.Title, t.Content, t.Desc, strings.Join(t.Tags, " ")},
		"\n",
	))
	return strings.Contains(haystack, filter)
}

func taskMatchesScope(t ticktick.Task, scope TaskScope) bool {
	switch normalizeTaskScope(scope) {
	case TaskScopeDone:
		return t.Done() && !t.Trashed() && !t.WontDo()
	case TaskScopeTrash:
		return t.Trashed()
	case TaskScopeAll:
		return true
	default:
		return !t.Done() && !t.Trashed()
	}
}

func subtreeHasVisibleRow(t ticktick.Task, idx taskChildren, showCompleted, showDeleted bool, filter string) bool {
	if taskRowVisible(t, showCompleted, showDeleted, filter) {
		return true
	}
	for _, child := range idx.ordered(t) {
		if subtreeHasVisibleRow(child, idx, showCompleted, showDeleted, filter) {
			return true
		}
	}
	return false
}

func appendVisibleTaskTree(
	parent ticktick.Task,
	depth int,
	idx taskChildren,
	showCompleted bool,
	showDeleted bool,
	filter string,
	out *[]taskListRow,
	placed map[string]bool,
) {
	if depth == 0 {
		if !subtreeHasVisibleRow(parent, idx, showCompleted, showDeleted, filter) {
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
	for _, child := range idx.ordered(parent) {
		appendVisibleTaskTree(child, depth+1, idx, showCompleted, showDeleted, filter, out, placed)
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

	idx := indexTaskChildren(tasks)
	var out []taskListRow
	placed := make(map[string]bool, len(roots))
	for _, root := range roots {
		appendVisibleTaskTree(root, 0, idx, showCompleted, showDeleted, filter, &out, placed)
	}
	return out
}

func appendTaskTreeForScope(
	parent ticktick.Task,
	depth int,
	idx taskChildren,
	scope TaskScope,
	filter string,
	out *[]taskListRow,
	placed map[string]bool,
) {
	if parent.ID != "" && placed[parent.ID] {
		return
	}
	self := taskMatchesScope(parent, scope) && taskMatchesFilter(parent, filter)
	kids := idx.ordered(parent)
	childVisible := false
	if !self {
		for _, child := range kids {
			if taskOrDescendantVisible(child, idx, scope, filter, map[string]bool{}) {
				childVisible = true
				break
			}
		}
	}
	if !self && !childVisible {
		return
	}
	if parent.ID != "" {
		placed[parent.ID] = true
	}
	*out = append(*out, taskListRow{Task: parent, Depth: depth})
	for _, child := range kids {
		appendTaskTreeForScope(child, depth+1, idx, scope, filter, out, placed)
	}
}

func taskOrDescendantVisible(task ticktick.Task, idx taskChildren, scope TaskScope, filter string, seen map[string]bool) bool {
	if task.ID != "" {
		if seen[task.ID] {
			return false
		}
		seen[task.ID] = true
	}
	if taskMatchesScope(task, scope) && taskMatchesFilter(task, filter) {
		return true
	}
	for _, child := range idx.ordered(task) {
		if taskOrDescendantVisible(child, idx, scope, filter, seen) {
			return true
		}
	}
	return false
}

func buildVisibleTaskRowsForScope(tasks []ticktick.Task, sortMode TaskSortMode, scope TaskScope, filter string) []taskListRow {
	seen := make(map[string]bool, len(tasks))
	var roots []ticktick.Task
	for _, task := range tasks {
		if task.IsSubtask() || task.ID == "" || seen[task.ID] {
			continue
		}
		seen[task.ID] = true
		roots = append(roots, task)
	}
	idx := indexTaskChildren(tasks)
	sortVisibleRoots(roots, idx, sortMode)
	var out []taskListRow
	placed := make(map[string]bool, len(tasks))
	for _, root := range roots {
		appendTaskTreeForScope(root, 0, idx, scope, filter, &out, placed)
	}
	for _, task := range tasks {
		if !placed[task.ID] && taskMatchesScope(task, scope) && taskMatchesFilter(task, filter) {
			out = append(out, taskListRow{Task: task})
		}
	}
	return out
}

func sortVisibleRoots(roots []ticktick.Task, idx taskChildren, mode TaskSortMode) {
	sort.SliceStable(roots, func(i, j int) bool {
		return compareTasks(rootForSort(roots[i], idx), rootForSort(roots[j], idx), mode)
	})
}

// rootForSort keeps a won't-do parent next to its open children in title order.
func rootForSort(task ticktick.Task, idx taskChildren) ticktick.Task {
	if !task.Done() || task.Trashed() {
		return task
	}
	for _, child := range idx.ordered(task) {
		if !child.Done() && !child.Trashed() {
			task.Status = 0
			return task
		}
	}
	return task
}

func taskTreePrefix(depth int) string {
	if depth < 1 {
		return ""
	}
	return strings.Repeat("  ", depth-1) + "├─ "
}
