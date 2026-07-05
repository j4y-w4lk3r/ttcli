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
	return live, nil
}

// ProjectTreeNode is one row in a folder tree.
type ProjectTreeNode struct {
	Depth   int
	Kind    string // "folder" or "list"
	Name    string
	ID      string
	ListKind string // TASK/NOTE for lists
}

// ProjectTree builds a folder → lists hierarchy.
func ProjectTree(groups []ProjectGroup, projects []Project) []ProjectTreeNode {
	byGroup := map[string][]Project{}
	var ungrouped []Project
	for _, p := range projects {
		gid := strings.TrimSpace(p.GroupID)
		if gid == "" || strings.EqualFold(gid, "NONE") {
			ungrouped = append(ungrouped, p)
			continue
		}
		byGroup[gid] = append(byGroup[gid], p)
	}
	sortProjects := func(ps []Project) {
		sort.Slice(ps, func(i, j int) bool { return ps[i].SortOrder < ps[j].SortOrder })
	}

	var out []ProjectTreeNode
	for _, g := range groups {
		ps := byGroup[g.ID]
		sortProjects(ps)
		out = append(out, ProjectTreeNode{Depth: 0, Kind: "folder", Name: g.Name, ID: g.ID})
		if len(ps) == 0 {
			out = append(out, ProjectTreeNode{Depth: 1, Kind: "empty", Name: "(empty)"})
			continue
		}
		for _, p := range ps {
			kind := p.Kind
			if kind == "" {
				kind = "TASK"
			}
			out = append(out, ProjectTreeNode{Depth: 1, Kind: "list", Name: p.Name, ID: p.ID, ListKind: kind})
		}
	}
	if len(ungrouped) > 0 {
		sortProjects(ungrouped)
		if len(groups) > 0 {
			out = append(out, ProjectTreeNode{Depth: 0, Kind: "folder", Name: "(no folder)"})
		}
		for _, p := range ungrouped {
			kind := p.Kind
			if kind == "" {
				kind = "TASK"
			}
			depth := 0
			if len(groups) > 0 {
				depth = 1
			}
			out = append(out, ProjectTreeNode{Depth: depth, Kind: "list", Name: p.Name, ID: p.ID, ListKind: kind})
		}
	}
	return out
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
