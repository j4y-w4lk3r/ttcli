package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type batchProjectGroupPayload struct {
	Add    []any    `json:"add"`
	Update []any    `json:"update"`
	Delete []string `json:"delete"`
}

type batchProjectPayload struct {
	Add    []any    `json:"add"`
	Update []any    `json:"update"`
	Delete []string `json:"delete"`
}

// ResolveProjectGroup finds a folder by exact name (case-insensitive) or id.
func (c *Client) ResolveProjectGroup(q string) (ProjectGroup, error) {
	gs, err := c.ListProjectGroups()
	if err != nil {
		return ProjectGroup{}, err
	}
	for _, g := range gs {
		if g.ID == q || strings.EqualFold(g.Name, q) {
			return g, nil
		}
	}
	return ProjectGroup{}, fmt.Errorf("no folder matching %q (try `ttcli ls --tree`)", q)
}

// CreateProjectGroup adds a new folder.
func (c *Client) CreateProjectGroup(name string) (string, error) {
	id := generateID()
	payload := batchProjectGroupPayload{
		Add: []any{map[string]any{
			"id":        id,
			"name":      name,
			"list_type": "group",
		}},
		Update: []any{},
		Delete: []string{},
	}
	b, _ := json.Marshal(payload)
	rb, err := c.do(http.MethodPost, "/api/v2/batch/projectGroup", b)
	if err != nil {
		return "", err
	}
	got, err := canonicalID(rb, id)
	if err != nil {
		return got, err
	}
	if got == "" {
		return id, nil
	}
	return got, nil
}

// RenameProjectGroup renames a folder.
func (c *Client) RenameProjectGroup(groupRef, newName string) error {
	g, err := c.ResolveProjectGroup(groupRef)
	if err != nil {
		return err
	}
	payload := batchProjectGroupPayload{
		Update: []any{map[string]any{
			"id":        g.ID,
			"name":      newName,
			"list_type": "group",
		}},
		Add:    []any{},
		Delete: []string{},
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/batch/projectGroup", b)
	return err
}

// DeleteProjectGroup removes a folder (lists inside become ungrouped).
func (c *Client) DeleteProjectGroup(groupRef string) error {
	g, err := c.ResolveProjectGroup(groupRef)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"delete": []string{g.ID}})
	_, err = c.do(http.MethodPost, "/api/v2/batch/projectGroup", b)
	return err
}

// GetProject returns the full project record for updates.
func (c *Client) GetProject(projectRef string) (map[string]any, error) {
	pid, err := c.ResolveProject(projectRef)
	if err != nil {
		return nil, err
	}
	ps, err := c.ListProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		if p.ID == pid {
			b, _ := json.Marshal(p)
			var m map[string]any
			if err := json.Unmarshal(b, &m); err != nil {
				return nil, err
			}
			// ListProjects is a subset; fetch raw list for full fields.
			raw, err := c.GetRaw("/api/v2/projects")
			if err != nil {
				return m, nil
			}
			var all []map[string]any
			if json.Unmarshal(raw, &all) == nil {
				for _, item := range all {
					if id, _ := item["id"].(string); id == pid {
						return item, nil
					}
				}
			}
			return m, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", projectRef)
}

// CreateProject adds a new list.
func (c *Client) CreateProject(name, folder, color, kind string) (string, error) {
	id := generateID()
	item := map[string]any{
		"id":        id,
		"name":      name,
		"view_mode": "list",
		"kind":      "TASK",
	}
	if kind != "" {
		item["kind"] = strings.ToUpper(kind)
	}
	if color != "" {
		item["color"] = color
	}
	if folder != "" {
		if strings.EqualFold(folder, "none") || folder == "-" {
			item["group_id"] = "NONE"
		} else {
			g, err := c.ResolveProjectGroup(folder)
			if err != nil {
				return "", err
			}
			item["group_id"] = g.ID
		}
	}
	payload := batchProjectPayload{Add: []any{item}, Update: []any{}, Delete: []string{}}
	b, _ := json.Marshal(payload)
	rb, err := c.do(http.MethodPost, "/api/v2/batch/project", b)
	if err != nil {
		return "", err
	}
	return canonicalID(rb, id)
}

// RenameProject renames a list.
func (c *Client) RenameProject(projectRef, newName string) error {
	proj, err := c.GetProject(projectRef)
	if err != nil {
		return err
	}
	proj["name"] = newName
	return c.updateProject(proj)
}

// MoveProject moves a list into a folder (or ungrouped with folder "none").
func (c *Client) MoveProject(projectRef, folder string) error {
	proj, err := c.GetProject(projectRef)
	if err != nil {
		return err
	}
	if strings.EqualFold(folder, "none") || folder == "-" || folder == "" {
		proj["groupId"] = "NONE"
	} else {
		g, err := c.ResolveProjectGroup(folder)
		if err != nil {
			return err
		}
		proj["groupId"] = g.ID
	}
	return c.updateProject(proj)
}

// DeleteProject removes a list.
func (c *Client) DeleteProject(projectRef string) error {
	pid, err := c.ResolveProject(projectRef)
	if err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"delete": []string{pid}})
	_, err = c.do(http.MethodPost, "/api/v2/batch/project", b)
	return err
}

func (c *Client) updateProject(proj map[string]any) error {
	id, _ := proj["id"].(string)
	if id == "" {
		return fmt.Errorf("project has no id")
	}
	payload := batchProjectPayload{Update: []any{proj}, Add: []any{}, Delete: []string{}}
	b, _ := json.Marshal(payload)
	_, err := c.do(http.MethodPost, "/api/v2/batch/project", b)
	return err
}

// TaskEdit holds optional task field updates.
type TaskEdit struct {
	Title    *string
	Content  *string
	Priority *int
	Project  string // move to list
}

// EditTask updates a task by id or title search.
func (c *Client) EditTask(query string, edit TaskEdit) error {
	var task map[string]any
	var err error
	if looksLikeTaskID(query) {
		task, err = c.FindTaskByID(query)
	} else {
		task, err = c.FindTask(query)
	}
	if err != nil {
		return err
	}
	if edit.Title != nil {
		task["title"] = *edit.Title
	}
	if edit.Content != nil {
		task["content"] = *edit.Content
	}
	if edit.Priority != nil {
		task["priority"] = *edit.Priority
	}
	if edit.Project != "" {
		pid, err := c.ResolveProject(edit.Project)
		if err != nil {
			return err
		}
		task["projectId"] = pid
	}
	task["modifiedTime"] = time.Now().UTC().Format(ticktickTimeLayout)

	payload := map[string]any{
		"add": []any{}, "update": []any{task}, "delete": []any{},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/batch/task", b)
	return err
}

func looksLikeTaskID(s string) bool {
	return len(s) >= 20 && !strings.Contains(s, " ")
}
