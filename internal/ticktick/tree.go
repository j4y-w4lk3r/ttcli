package ticktick

import (
	"fmt"
	"sort"
	"strings"
)

// ProjectGroup is a TickTick folder that contains lists.
type ProjectGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SortOrder int64  `json:"sortOrder"`
	Deleted   int    `json:"deleted"`
}

// ListProjectGroups returns project folders (TickTick "groups").
func (c *Client) ListProjectGroups() ([]ProjectGroup, error) {
	var gs []ProjectGroup
	if err := c.getJSON("/api/v2/projectGroups", &gs); err != nil {
		return nil, err
	}
	live := gs[:0]
	for _, g := range gs {
		if g.Deleted == 0 {
			live = append(live, g)
		}
	}
	sort.Slice(live, func(i, j int) bool { return live[i].SortOrder < live[j].SortOrder })
	return dedupeGroups(live), nil
}

// ProjectTreeNode is one row in a folder tree.
type ProjectTreeNode struct {
	Depth    int
	Kind     string // "folder", "list", or "empty"
	Name     string
	ID       string
	ListKind string // TASK/NOTE for lists
}

func isUngrouped(gid string) bool {
	gid = strings.TrimSpace(gid)
	return gid == "" || strings.EqualFold(gid, "NONE")
}

func dedupeGroups(groups []ProjectGroup) []ProjectGroup {
	seen := make(map[string]struct{}, len(groups))
	out := make([]ProjectGroup, 0, len(groups))
	for _, g := range groups {
		if g.ID == "" {
			continue
		}
		if _, ok := seen[g.ID]; ok {
			continue
		}
		seen[g.ID] = struct{}{}
		out = append(out, g)
	}
	return out
}

// dedupeProjects keeps one row per list id. When the API returns duplicates,
// prefer the copy that belongs to a folder over NONE/ungrouped.
func dedupeProjects(projects []Project) []Project {
	byID := make(map[string]Project, len(projects))
	order := make([]string, 0, len(projects))
	for _, p := range projects {
		if p.ID == "" {
			continue
		}
		if existing, ok := byID[p.ID]; ok {
			if isUngrouped(existing.GroupID) && !isUngrouped(p.GroupID) {
				byID[p.ID] = p
			}
			continue
		}
		byID[p.ID] = p
		order = append(order, p.ID)
	}
	out := make([]Project, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out
}

func uniqueProjectsInOrder(ps []Project) []Project {
	seen := make(map[string]struct{}, len(ps))
	out := make([]Project, 0, len(ps))
	for _, p := range ps {
		if p.ID == "" {
			continue
		}
		if _, ok := seen[p.ID]; ok {
			continue
		}
		seen[p.ID] = struct{}{}
		out = append(out, p)
	}
	return out
}

func listKind(p Project) string {
	kind := p.Kind
	if kind == "" {
		kind = "TASK"
	}
	return kind
}

// ProjectTree builds a folder → lists hierarchy. Each list appears at most once.
func ProjectTree(groups []ProjectGroup, projects []Project) []ProjectTreeNode {
	groups = dedupeGroups(groups)
	projects = dedupeProjects(projects)

	groupIDs := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		groupIDs[g.ID] = struct{}{}
	}

	byGroup := map[string][]Project{}
	var ungrouped []Project
	for _, p := range projects {
		gid := strings.TrimSpace(p.GroupID)
		if isUngrouped(gid) {
			ungrouped = append(ungrouped, p)
			continue
		}
		byGroup[gid] = append(byGroup[gid], p)
	}

	sortProjects := func(ps []Project) {
		sort.Slice(ps, func(i, j int) bool { return ps[i].SortOrder < ps[j].SortOrder })
	}

	placed := make(map[string]struct{}, len(projects))
	appendList := func(out *[]ProjectTreeNode, p Project, depth int) {
		if _, ok := placed[p.ID]; ok {
			return
		}
		*out = append(*out, ProjectTreeNode{
			Depth:    depth,
			Kind:     "list",
			Name:     p.Name,
			ID:       p.ID,
			ListKind: listKind(p),
		})
		placed[p.ID] = struct{}{}
	}

	var out []ProjectTreeNode
	for _, g := range groups {
		ps := uniqueProjectsInOrder(byGroup[g.ID])
		sortProjects(ps)
		out = append(out, ProjectTreeNode{Depth: 0, Kind: "folder", Name: g.Name, ID: g.ID})
		if len(ps) == 0 {
			out = append(out, ProjectTreeNode{Depth: 1, Kind: "empty", Name: "(empty)"})
			continue
		}
		for _, p := range ps {
			appendList(&out, p, 1)
		}
	}

	// Lists whose groupId is missing from projectGroups land here once.
	var orphans []Project
	for gid, ps := range byGroup {
		if _, ok := groupIDs[gid]; ok {
			continue
		}
		orphans = append(orphans, ps...)
	}
	if len(orphans) > 0 {
		orphans = uniqueProjectsInOrder(orphans)
		sortProjects(orphans)
		ungrouped = append(ungrouped, orphans...)
	}

	if len(ungrouped) > 0 {
		ungrouped = uniqueProjectsInOrder(ungrouped)
		sortProjects(ungrouped)
		if len(groups) > 0 {
			out = append(out, ProjectTreeNode{Depth: 0, Kind: "folder", Name: "(no folder)"})
		}
		listDepth := 0
		if len(groups) > 0 {
			listDepth = 1
		}
		for _, p := range ungrouped {
			appendList(&out, p, listDepth)
		}
	}
	return out
}

// ProjectTreeWithInbox builds ProjectTree and pins TickTick's Inbox at the top.
// The inbox id comes from sync and is not listed in /api/v2/projects.
func ProjectTreeWithInbox(inboxID string, groups []ProjectGroup, projects []Project) []ProjectTreeNode {
	tree := ProjectTree(groups, projects)
	if inboxID == "" {
		return tree
	}
	for _, n := range tree {
		if n.Kind == "list" && n.ID == inboxID {
			return tree
		}
	}
	inbox := ProjectTreeNode{
		Depth:    0,
		Kind:     "list",
		Name:     "Inbox",
		ID:       inboxID,
		ListKind: "TASK",
	}
	return append([]ProjectTreeNode{inbox}, tree...)
}

// FormatProjectTree renders the tree for terminal output.
func FormatProjectTree(nodes []ProjectTreeNode) string {
	var b strings.Builder
	for _, n := range nodes {
		indent := strings.Repeat("  ", n.Depth)
		switch n.Kind {
		case "folder":
			fmt.Fprintf(&b, "%s📁 %s\n", indent, n.Name)
		case "empty":
			fmt.Fprintf(&b, "%s  (empty)\n", indent)
		default:
			fmt.Fprintf(&b, "%s  • %s  (%s)\n", indent, n.Name, n.ID)
		}
	}
	return b.String()
}

// ListIDsInTree returns list project ids in display order (for tests).
func ListIDsInTree(nodes []ProjectTreeNode) []string {
	var ids []string
	for _, n := range nodes {
		if n.Kind == "list" && n.ID != "" {
			ids = append(ids, n.ID)
		}
	}
	return ids
}
