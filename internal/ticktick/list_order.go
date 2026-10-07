package ticktick

import "sort"

const listSortGap int64 = 1024

// ProjectPlacement is a sidebar reorder. Orders lists the sort keys to write.
// SetGroup updates the moved list's folder.
type ProjectPlacement struct {
	MovedID  string
	GroupID  string
	SetGroup bool
	Orders   map[string]int64
}

// Active reports whether this placement changes a list.
func (p ProjectPlacement) Active() bool {
	return p.MovedID != "" && len(p.Orders) > 0
}

// GroupPlacement is a folder reorder.
type GroupPlacement struct {
	MovedID string
	Orders  map[string]int64
}

// Active reports whether this placement changes a folder.
func (p GroupPlacement) Active() bool {
	return p.MovedID != "" && len(p.Orders) > 0
}

func folderKey(groupID string) string {
	if isUngrouped(groupID) {
		return "NONE"
	}
	return groupID
}

func listsInFolder(projects []Project, folderID string) []Project {
	key := folderKey(folderID)
	var out []Project
	for _, project := range projects {
		if project.ID == "" || folderKey(project.GroupID) != key {
			continue
		}
		out = append(out, project)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func sortOrderBetween(prev, next *int64) (int64, bool) {
	switch {
	case prev == nil && next == nil:
		return 0, true
	case prev == nil:
		return *next - listSortGap, true
	case next == nil:
		return *prev + listSortGap, true
	case *next > *prev+1:
		return *prev + (*next-*prev)/2, true
	default:
		return 0, false
	}
}

func ordersForSequence(seq []string, byID map[string]int64, movedID string) map[string]int64 {
	idx := -1
	for i, id := range seq {
		if id == movedID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return nil
	}
	var prev, next *int64
	if idx > 0 {
		if order, ok := byID[seq[idx-1]]; ok {
			prev = &order
		}
	}
	if idx+1 < len(seq) {
		if order, ok := byID[seq[idx+1]]; ok {
			next = &order
		}
	}
	if order, ok := sortOrderBetween(prev, next); ok {
		return map[string]int64{movedID: order}
	}
	out := make(map[string]int64, len(seq))
	for i, id := range seq {
		out[id] = int64(i) * listSortGap
	}
	return out
}

// MoveListStep moves a list one place up (dir -1) or down (dir 1) among the
// lists in its folder.
func MoveListStep(projects []Project, listID string, dir int) ProjectPlacement {
	if listID == "" || (dir != -1 && dir != 1) {
		return ProjectPlacement{}
	}
	var self Project
	found := false
	for _, project := range projects {
		if project.ID == listID {
			self = project
			found = true
			break
		}
	}
	if !found {
		return ProjectPlacement{}
	}
	sibs := listsInFolder(projects, self.GroupID)
	index := -1
	for i, project := range sibs {
		if project.ID == listID {
			index = i
			break
		}
	}
	next := index + dir
	if index < 0 || next < 0 || next >= len(sibs) {
		return ProjectPlacement{}
	}
	seq := make([]string, len(sibs))
	byID := make(map[string]int64, len(sibs))
	for i, project := range sibs {
		seq[i] = project.ID
		if project.ID != listID {
			byID[project.ID] = project.SortOrder
		}
	}
	seq[index], seq[next] = seq[next], seq[index]
	return ProjectPlacement{MovedID: listID, Orders: ordersForSequence(seq, byID, listID)}
}

// MoveListToSlot places a list in a folder. beforeID is the list it should
// sit in front of. An empty beforeID puts it first. atEnd puts it last.
func MoveListToSlot(projects []Project, listID, folderID, beforeID string, atEnd bool) ProjectPlacement {
	if listID == "" {
		return ProjectPlacement{}
	}
	sibs := listsInFolder(projects, folderID)
	rest := make([]string, 0, len(sibs))
	byID := make(map[string]int64, len(sibs))
	for _, project := range sibs {
		if project.ID == listID {
			continue
		}
		rest = append(rest, project.ID)
		byID[project.ID] = project.SortOrder
	}
	insert := len(rest)
	if !atEnd {
		if beforeID == "" {
			insert = 0
		} else {
			for i, id := range rest {
				if id == beforeID {
					insert = i
					break
				}
			}
		}
	}
	seq := make([]string, 0, len(rest)+1)
	seq = append(seq, rest[:insert]...)
	seq = append(seq, listID)
	seq = append(seq, rest[insert:]...)
	return ProjectPlacement{
		MovedID:  listID,
		GroupID:  folderKey(folderID),
		SetGroup: true,
		Orders:   ordersForSequence(seq, byID, listID),
	}
}

func sortedGroups(groups []ProjectGroup) []ProjectGroup {
	var out []ProjectGroup
	for _, group := range groups {
		if group.ID == "" || group.Deleted != 0 {
			continue
		}
		out = append(out, group)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out
}

type listContainer struct {
	folderID string
	ids      []string
}

// sidebarContainers is the lists pane order: each folder, then lists with no folder.
func sidebarContainers(groups []ProjectGroup, projects []Project) []listContainer {
	var out []listContainer
	for _, group := range sortedGroups(groups) {
		sibs := listsInFolder(projects, group.ID)
		ids := make([]string, len(sibs))
		for i, project := range sibs {
			ids[i] = project.ID
		}
		out = append(out, listContainer{folderID: group.ID, ids: ids})
	}
	ungrouped := listsInFolder(projects, "NONE")
	ids := make([]string, len(ungrouped))
	for i, project := range ungrouped {
		ids[i] = project.ID
	}
	out = append(out, listContainer{folderID: "NONE", ids: ids})
	return out
}

// MoveListVisual moves a list one row up or down in the sidebar. Passing a
// folder boundary carries the list into that folder.
func MoveListVisual(projects []Project, groups []ProjectGroup, listID string, dir int) ProjectPlacement {
	if listID == "" || (dir != -1 && dir != 1) {
		return ProjectPlacement{}
	}
	containers := sidebarContainers(groups, projects)
	container, index := -1, -1
	for i, group := range containers {
		for j, id := range group.ids {
			if id == listID {
				container, index = i, j
			}
		}
	}
	if container < 0 {
		return ProjectPlacement{}
	}
	if dir < 0 {
		if index > 0 {
			return MoveListStep(projects, listID, -1)
		}
		if container == 0 {
			return ProjectPlacement{}
		}
		return MoveListToSlot(projects, listID, containers[container-1].folderID, "", true)
	}
	if index < len(containers[container].ids)-1 {
		return MoveListStep(projects, listID, 1)
	}
	if container >= len(containers)-1 {
		return ProjectPlacement{}
	}
	return MoveListToSlot(projects, listID, containers[container+1].folderID, "", false)
}

// PlacementFromProjects is the sidebar write for a list after a local reorder.
func PlacementFromProjects(before, after []Project, movedID string) ProjectPlacement {
	if movedID == "" {
		return ProjectPlacement{}
	}
	prior := make(map[string]Project, len(before))
	for _, project := range before {
		if project.ID != "" {
			prior[project.ID] = project
		}
	}
	var current Project
	found := false
	orders := map[string]int64{}
	for _, project := range after {
		if project.ID == "" {
			continue
		}
		if project.ID == movedID {
			current = project
			found = true
		}
		old, ok := prior[project.ID]
		if !ok || old.SortOrder != project.SortOrder || folderKey(old.GroupID) != folderKey(project.GroupID) {
			orders[project.ID] = project.SortOrder
		}
	}
	if !found || len(orders) == 0 {
		return ProjectPlacement{}
	}
	return ProjectPlacement{
		MovedID:  movedID,
		GroupID:  folderKey(current.GroupID),
		SetGroup: true,
		Orders:   orders,
	}
}

// PlacementFromGroups is the sidebar write for a folder after a local reorder.
func PlacementFromGroups(before, after []ProjectGroup, movedID string) GroupPlacement {
	if movedID == "" {
		return GroupPlacement{}
	}
	prior := make(map[string]int64, len(before))
	for _, group := range before {
		if group.ID != "" {
			prior[group.ID] = group.SortOrder
		}
	}
	orders := map[string]int64{}
	found := false
	for _, group := range after {
		if group.ID == "" {
			continue
		}
		if group.ID == movedID {
			found = true
		}
		old, ok := prior[group.ID]
		if !ok || old != group.SortOrder {
			orders[group.ID] = group.SortOrder
		}
	}
	if !found || len(orders) == 0 {
		return GroupPlacement{}
	}
	return GroupPlacement{MovedID: movedID, Orders: orders}
}

// MoveFolderStep moves a folder one place up (dir -1) or down (dir 1).
func MoveFolderStep(groups []ProjectGroup, groupID string, dir int) GroupPlacement {
	if groupID == "" || (dir != -1 && dir != 1) {
		return GroupPlacement{}
	}
	sibs := sortedGroups(groups)
	index := -1
	for i, group := range sibs {
		if group.ID == groupID {
			index = i
			break
		}
	}
	next := index + dir
	if index < 0 || next < 0 || next >= len(sibs) {
		return GroupPlacement{}
	}
	seq := make([]string, len(sibs))
	byID := make(map[string]int64, len(sibs))
	for i, group := range sibs {
		seq[i] = group.ID
		if group.ID != groupID {
			byID[group.ID] = group.SortOrder
		}
	}
	seq[index], seq[next] = seq[next], seq[index]
	return GroupPlacement{MovedID: groupID, Orders: ordersForSequence(seq, byID, groupID)}
}
