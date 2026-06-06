// Package ticktick is a thin client for TickTick's private web API
// (api.ticktick.com), authenticated with a captured browser session
// (cookies + headers) rather than the official OAuth Open API.
//
// This mirrors the behaviour of the original Python ticktick-automator:
// the session is loaded from a JSON file shaped like
//
//	{"cookies": {"k": "v", ...}, "headers": {"k": "v", ...}, "saved_at": "..."}
//
// The cookies are joined into a Cookie header, the _csrf_token cookie is
// echoed back as x-csrftoken, and any extra headers are merged over the
// browser-like defaults.
package ticktick

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the private API host (NOT the OAuth Open API).
	DefaultBaseURL = "https://api.ticktick.com"
	webURL         = "https://ticktick.com"
)

type authFile struct {
	Cookies map[string]string `json:"cookies"`
	Headers map[string]string `json:"headers"`
	SavedAt string            `json:"saved_at"`
}

// Client talks to the TickTick private API with a captured session.
type Client struct {
	BaseURL  string
	AuthPath string
	SavedAt  string
	http     *http.Client
	headers  map[string]string
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent":       "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36",
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "application/json, text/plain, */*",
		"Accept-Language":  "en-US,en;q=0.9",
		"Content-Type":     "application/json;charset=UTF-8",
		"Origin":           webURL,
		"Referer":          webURL + "/",
		"Sec-Fetch-Dest":   "empty",
		"Sec-Fetch-Mode":   "cors",
		"Sec-Fetch-Site":   "same-site",
	}
}

// ResolveAuthPath finds the session file, matching the Python client's
// search order: $TICKTICK_AUTH_FILE, then ~/.ticktick_auth.json, then
// ./ticktick_auth.json.
func ResolveAuthPath() string {
	if p := os.Getenv("TICKTICK_AUTH_FILE"); p != "" {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		h := filepath.Join(home, ".ticktick_auth.json")
		if _, err := os.Stat(h); err == nil {
			return h
		}
	}
	return "ticktick_auth.json"
}

// New loads the session at authPath (or the resolved default if empty)
// and returns a ready client.
func New(authPath string) (*Client, error) {
	if authPath == "" {
		authPath = ResolveAuthPath()
	}
	raw, err := os.ReadFile(authPath)
	if err != nil {
		return nil, fmt.Errorf("read auth file %s: %w\n  run the capture/login flow to create it (see README)", authPath, err)
	}
	var a authFile
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("parse auth file %s: %w", authPath, err)
	}
	h := defaultHeaders()
	if len(a.Cookies) > 0 {
		parts := make([]string, 0, len(a.Cookies))
		for k, v := range a.Cookies {
			parts = append(parts, k+"="+v)
		}
		sort.Strings(parts)
		h["Cookie"] = strings.Join(parts, "; ")
		if csrf := a.Cookies["_csrf_token"]; csrf != "" {
			h["x-csrftoken"] = csrf
		}
	}
	for k, v := range a.Headers {
		h[k] = v
	}
	return &Client{
		BaseURL:  DefaultBaseURL,
		AuthPath: authPath,
		SavedAt:  a.SavedAt,
		http:     &http.Client{Timeout: 30 * time.Second},
		headers:  h,
	}, nil
}

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("auth rejected (HTTP %d) — the captured session has likely expired; re-capture %s", resp.StatusCode, c.AuthPath)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("GET %s: HTTP %d: %s", path, resp.StatusCode, truncate(string(body), 300))
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

// GetRaw fetches an arbitrary API path and returns the raw JSON. Handy
// for `ttcli raw /api/v2/...` while we expand typed coverage.
func (c *Client) GetRaw(path string) ([]byte, error) {
	var raw json.RawMessage
	if err := c.get(path, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

// ListProjects returns all lists/projects.
func (c *Client) ListProjects() ([]Project, error) {
	var ps []Project
	if err := c.get("/api/v2/projects", &ps); err != nil {
		return nil, err
	}
	return ps, nil
}

// ProjectTasks returns the live (non-deleted) tasks in a project. The
// /api/v2/project/{id}/tasks endpoint may answer as a bare array or as
// an object with a "tasks" / "syncTaskBean.update" field, so we handle
// both shapes (matching the Python client's tolerance).
func (c *Client) ProjectTasks(projectID string) ([]Task, error) {
	raw, err := c.GetRaw("/api/v2/project/" + projectID + "/tasks")
	if err != nil {
		return nil, err
	}
	tasks := parseTasks(raw)
	live := tasks[:0]
	for _, t := range tasks {
		if t.Deleted == 0 {
			live = append(live, t)
		}
	}
	return live, nil
}

func parseTasks(raw []byte) []Task {
	// Shape 1: bare array.
	var arr []Task
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	// Shape 2: object with tasks / syncTaskBean.update.
	var obj struct {
		Tasks        []Task `json:"tasks"`
		SyncTaskBean struct {
			Update []Task `json:"update"`
		} `json:"syncTaskBean"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		if len(obj.Tasks) > 0 {
			return obj.Tasks
		}
		if len(obj.SyncTaskBean.Update) > 0 {
			return obj.SyncTaskBean.Update
		}
	}
	return nil
}
