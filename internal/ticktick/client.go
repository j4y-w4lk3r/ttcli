// Package ticktick is a thin client for TickTick's private web API
// (api.ticktick.com), authenticated with a captured browser session
// (cookies + headers) rather than the official OAuth Open API.
//
// The session is persisted as JSON shaped like
//
//	{"cookies": {"k": "v", ...}, "headers": {"k": "v", ...}, "saved_at": "..."}
//
// Cookies are joined into a Cookie header, the _csrf_token cookie is echoed
// back as x-csrftoken, and any extra headers are merged over browser-like
// defaults. When TICKTICK_EMAIL / TICKTICK_PASSWORD are set the client can
// mint a fresh session itself (ttcli login) and silently re-authenticate
// when the API answers 401 — which is what makes it usable headless.
package ticktick

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
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

	// xDevice mirrors the web client's device descriptor; signon rejects
	// requests without a plausible X-Device header.
	xDevice = `{"platform":"web","os":"macOS 10.15.7","device":"Chrome 138.0.0.0","name":"","version":6360,"id":"68983d411ca57e044a5fa842","channel":"website","campaign":"","websocket":""}`

	ticktickTimeLayout = "2006-01-02T15:04:05.000-0700"
)

type authFile struct {
	Cookies map[string]string `json:"cookies"`
	Headers map[string]string `json:"headers"`
	SavedAt string            `json:"saved_at"`
}

// Credentials are the TickTick login email + password used to mint or
// refresh a session.
type Credentials struct {
	Email    string
	Password string
}

// Valid reports whether both fields are present.
func (c Credentials) Valid() bool { return c.Email != "" && c.Password != "" }

// CredentialsFromEnv reads TICKTICK_EMAIL / TICKTICK_PASSWORD.
func CredentialsFromEnv() Credentials {
	return Credentials{
		Email:    os.Getenv("TICKTICK_EMAIL"),
		Password: os.Getenv("TICKTICK_PASSWORD"),
	}
}

// Client talks to the TickTick private API with a captured session.
type Client struct {
	BaseURL  string
	AuthPath string
	SavedAt  string

	creds   Credentials
	http    *http.Client
	headers map[string]string
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

// DefaultAuthSavePath is where a freshly minted session is written when no
// explicit path is given: $TICKTICK_AUTH_FILE, else ~/.ticktick_auth.json.
func DefaultAuthSavePath() string {
	if p := os.Getenv("TICKTICK_AUTH_FILE"); p != "" {
		return p
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".ticktick_auth.json")
	}
	return "ticktick_auth.json"
}

// New loads the session at authPath (or the resolved default if empty).
// If no session file exists but env credentials are present, it logs in
// and writes one.
func New(authPath string) (*Client, error) {
	explicit := authPath != ""
	if authPath == "" {
		authPath = ResolveAuthPath()
	}
	c := &Client{
		BaseURL:  DefaultBaseURL,
		AuthPath: authPath,
		creds:    CredentialsFromEnv(),
		http:     &http.Client{Timeout: 30 * time.Second},
		headers:  defaultHeaders(),
	}
	if err := c.loadAuthFile(authPath); err != nil {
		if c.creds.Valid() {
			// No session on disk: mint one and persist it to the home
			// path (not the cwd fallback) unless the caller was explicit.
			if !explicit {
				c.AuthPath = DefaultAuthSavePath()
			}
			return c, c.refresh()
		}
		return nil, err
	}
	return c, nil
}

// Login mints a fresh session with creds, writes it to authPath (or the
// default save path if empty), and returns a ready client.
func Login(creds Credentials, authPath string) (*Client, error) {
	if !creds.Valid() {
		return nil, fmt.Errorf("missing credentials: set TICKTICK_EMAIL and TICKTICK_PASSWORD (or pass --email/--password)")
	}
	if authPath == "" {
		authPath = DefaultAuthSavePath()
	}
	c := &Client{
		BaseURL:  DefaultBaseURL,
		AuthPath: authPath,
		creds:    creds,
		http:     &http.Client{Timeout: 30 * time.Second},
		headers:  defaultHeaders(),
	}
	if err := c.refresh(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) loadAuthFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read auth file %s: %w\n  run `ttcli login` (with TICKTICK_EMAIL/PASSWORD set) to create it", path, err)
	}
	var a authFile
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("parse auth file %s: %w", path, err)
	}
	c.applyAuth(a)
	c.SavedAt = a.SavedAt
	return nil
}

func (c *Client) applyAuth(a authFile) {
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
	c.headers = h
}

// refresh performs a fresh signon and persists the resulting session.
func (c *Client) refresh() error {
	cookies, err := doSignon(c.creds)
	if err != nil {
		return err
	}
	a := authFile{
		Cookies: cookies,
		Headers: map[string]string{"x-csrftoken": cookies["_csrf_token"]},
		SavedAt: time.Now().UTC().Format(time.RFC3339),
	}
	c.applyAuth(a)
	c.SavedAt = a.SavedAt
	if err := saveAuth(c.AuthPath, a); err != nil {
		// Non-fatal: we can still operate this session in-memory.
		fmt.Fprintf(os.Stderr, "ttcli: warning: could not save session to %s: %v\n", c.AuthPath, err)
	}
	return nil
}

// doSignon warms a session against the web signin page, posts credentials
// to /api/v2/user/signon, and harvests the resulting cookies.
func doSignon(creds Credentials) (map[string]string, error) {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Timeout: 30 * time.Second, Jar: jar}

	// Warm-up GET to seed _csrf_token and friends.
	if req, err := http.NewRequest(http.MethodGet, webURL+"/signin", nil); err == nil {
		req.Header.Set("User-Agent", defaultHeaders()["User-Agent"])
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		if resp, err := hc.Do(req); err == nil {
			resp.Body.Close()
		}
	}

	body, _ := json.Marshal(map[string]any{
		"username": creds.Email,
		"password": creds.Password,
		"remember": true,
	})
	req, err := http.NewRequest(http.MethodPost, DefaultBaseURL+"/api/v2/user/signon?wc=true&remember=true", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultHeaders()["User-Agent"])
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-GB,en-US;q=0.9,en;q=0.8")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Referer", webURL+"/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("X-Device", xDevice)
	resp, err := hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("signon request: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed: HTTP %d: %s", resp.StatusCode, truncate(string(rb), 300))
	}

	cookies := map[string]string{}
	for _, u := range []string{webURL, DefaultBaseURL} {
		pu, _ := url.Parse(u)
		for _, ck := range jar.Cookies(pu) {
			cookies[ck.Name] = ck.Value
		}
	}
	if len(cookies) == 0 {
		return nil, fmt.Errorf("login returned 200 but no session cookies were set")
	}
	return cookies, nil
}

func saveAuth(path string, a authFile) error {
	if dir := filepath.Dir(path); dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// ---- request helpers (with one-shot 401 auto-refresh) ----

func (c *Client) do(method, path string, body []byte) ([]byte, error) {
	rb, status, err := c.do1(method, path, body)
	if err != nil {
		return nil, err
	}
	if (status == http.StatusUnauthorized || status == http.StatusForbidden) && c.creds.Valid() {
		if rerr := c.refresh(); rerr != nil {
			return nil, fmt.Errorf("session expired and re-login failed: %w", rerr)
		}
		rb, status, err = c.do1(method, path, body)
		if err != nil {
			return nil, err
		}
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		return nil, fmt.Errorf("auth rejected (HTTP %d) — the session has expired; run `ttcli login` (or set TICKTICK_EMAIL/PASSWORD for auto-refresh)", status)
	}
	if status >= 300 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %s", method, path, status, truncate(string(rb), 300))
	}
	return rb, nil
}

func (c *Client) do1(method, path string, body []byte) ([]byte, int, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.BaseURL+path, rdr)
	if err != nil {
		return nil, 0, err
	}
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("X-Device", xDevice)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if os.Getenv("TTCLI_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "[ttcli] %s %s -> %d: %s\n", method, path, resp.StatusCode, truncate(string(rb), 600))
	}
	return rb, resp.StatusCode, nil
}

func (c *Client) getJSON(path string, out any) error {
	rb, err := c.do(http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	if out != nil {
		if err := json.Unmarshal(rb, out); err != nil {
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

// GetRaw fetches an arbitrary API path and returns the raw JSON.
func (c *Client) GetRaw(path string) ([]byte, error) {
	return c.do(http.MethodGet, path, nil)
}

// ---- typed reads ----

// ListProjects returns all lists/projects.
func (c *Client) ListProjects() ([]Project, error) {
	var ps []Project
	if err := c.getJSON("/api/v2/projects", &ps); err != nil {
		return nil, err
	}
	return ps, nil
}

// InboxID resolves the user's Inbox project id from the sync endpoint.
func (c *Client) InboxID() (string, error) {
	var data struct {
		InboxID string `json:"inboxId"`
	}
	if err := c.getJSON("/api/v3/batch/check/0", &data); err != nil {
		return "", err
	}
	if data.InboxID == "" {
		return "", fmt.Errorf("could not resolve inbox id from sync")
	}
	return data.InboxID, nil
}

// ResolveProject turns a project id or name into an id. Empty input
// resolves to the Inbox.
func (c *Client) ResolveProject(q string) (string, error) {
	if q == "" || strings.EqualFold(q, "inbox") {
		return c.InboxID()
	}
	if strings.HasPrefix(q, "inbox") || looksLikeID(q) {
		return q, nil
	}
	ps, err := c.ListProjects()
	if err != nil {
		return "", err
	}
	for _, p := range ps {
		if strings.EqualFold(p.Name, q) {
			return p.ID, nil
		}
	}
	return "", fmt.Errorf("no project named %q (try `ttcli ls`)", q)
}

// ProjectTasks returns the live (non-deleted) tasks in a project.
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
	var arr []Task
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr
	}
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

// ---- typed writes ----

// AddTask creates a task in projectID (Inbox if empty) and returns its id.
func (c *Client) AddTask(title, projectID string, priority int, content string) (string, error) {
	pid, err := c.ResolveProject(projectID)
	if err != nil {
		return "", err
	}
	now := time.Now().UTC().Format(ticktickTimeLayout)
	id := generateID()
	item := map[string]any{
		"id":           id,
		"projectId":    pid,
		"title":        title,
		"content":      content,
		"priority":     priority,
		"status":       0,
		"progress":     0,
		"tags":         []string{},
		"items":        []any{},
		"exDate":       []any{},
		"sortOrder":    -time.Now().UnixMicro(),
		"isAllDay":     nil,
		"isFloating":   false,
		"timeZone":     localTZ(),
		"createdTime":  now,
		"modifiedTime": now,
	}
	payload := map[string]any{
		"add": []any{item}, "update": []any{}, "delete": []any{},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	rb, err := c.do(http.MethodPost, "/api/v2/batch/task", b)
	if err != nil {
		return "", err
	}
	return canonicalID(rb, id)
}

// canonicalID parses a batch response ({"id2etag":{...},"id2error":{...}})
// and returns the server's id for the (single) item, failing if the item
// reported an error. TickTick assigns its own ObjectId when the client id
// isn't a valid 24-hex ObjectId, so the response is the source of truth.
func canonicalID(rb []byte, fallback string) (string, error) {
	var resp struct {
		ID2Etag  map[string]string `json:"id2etag"`
		ID2Error map[string]any    `json:"id2error"`
	}
	if err := json.Unmarshal(rb, &resp); err != nil {
		return fallback, nil
	}
	if len(resp.ID2Error) > 0 {
		return "", fmt.Errorf("server rejected task: %v", resp.ID2Error)
	}
	for id := range resp.ID2Etag {
		return id, nil
	}
	return fallback, nil
}

// DeleteTask permanently removes a task.
func (c *Client) DeleteTask(projectID, taskID string) error {
	pid, err := c.ResolveProject(projectID)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"add": []any{}, "update": []any{},
		"delete":         []any{map[string]string{"taskId": taskID, "projectId": pid}},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/batch/task", b)
	return err
}

// CompleteTask marks a task done by patching its raw JSON status to 2 and
// sending it back in the update batch (preserving all other fields).
func (c *Client) CompleteTask(projectID, taskID string) error {
	pid, err := c.ResolveProject(projectID)
	if err != nil {
		return err
	}
	raw, err := c.GetRaw("/api/v2/project/" + pid + "/tasks")
	if err != nil {
		return err
	}
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err != nil {
		// object-shaped response
		var obj struct {
			Tasks []map[string]any `json:"tasks"`
		}
		if err2 := json.Unmarshal(raw, &obj); err2 != nil {
			return fmt.Errorf("decode tasks: %w", err)
		}
		arr = obj.Tasks
	}
	var target map[string]any
	for _, t := range arr {
		if id, _ := t["id"].(string); id == taskID {
			target = t
			break
		}
	}
	if target == nil {
		return fmt.Errorf("task %s not found in project %s", taskID, pid)
	}
	target["status"] = 2
	target["completedTime"] = time.Now().UTC().Format(ticktickTimeLayout)
	payload := map[string]any{
		"add": []any{}, "update": []any{target}, "delete": []any{},
		"addAttachments": []any{}, "updateAttachments": []any{}, "deleteAttachments": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/batch/task", b)
	return err
}

// FocusStats summarises pomodoro/focus records for a UTC day.
type FocusStats struct {
	Date         string
	PomoCount    int
	TotalSeconds int64
	Records      []FocusRecord
}

// FocusRecord is one pomodoro/focus session.
type FocusRecord struct {
	ID        string `json:"id"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Status    int    `json:"status"`
	Note      string `json:"note"`
	Tasks     []struct {
		TaskID      string `json:"taskId"`
		Title       string `json:"title"`
		ProjectName string `json:"projectName"`
		StartTime   string `json:"startTime"`
		EndTime     string `json:"endTime"`
	} `json:"tasks"`
}

// FocusForDay fetches pomodoro records for the given day (UTC). A zero day
// means today.
func (c *Client) FocusForDay(day time.Time) (*FocusStats, error) {
	if day.IsZero() {
		day = time.Now().UTC()
	}
	day = day.UTC()
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.UTC)
	end := time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 999000000, time.UTC)
	path := fmt.Sprintf("/api/v2/pomodoros?from=%d&to=%d", start.UnixMilli(), end.UnixMilli())
	var recs []FocusRecord
	if err := c.getJSON(path, &recs); err != nil {
		return nil, err
	}
	stats := &FocusStats{Date: start.Format("2006-01-02"), Records: recs}
	for _, r := range recs {
		stats.PomoCount++
		st, e1 := time.Parse(ticktickTimeLayout, r.StartTime)
		et, e2 := time.Parse(ticktickTimeLayout, r.EndTime)
		if e1 == nil && e2 == nil && et.After(st) {
			stats.TotalSeconds += int64(et.Sub(st).Seconds())
		}
	}
	return stats, nil
}

// ---- helpers ----

// generateID returns a MongoDB-style 24-hex ObjectId (4-byte timestamp +
// 8 random bytes), which is the id shape TickTick's batch API expects.
func generateID() string {
	b := make([]byte, 12)
	binary.BigEndian.PutUint32(b[:4], uint32(time.Now().Unix()))
	_, _ = rand.Read(b[4:])
	return hex.EncodeToString(b)
}

func localTZ() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	return "Europe/Warsaw"
}

func looksLikeID(s string) bool {
	if len(s) < 16 {
		return false
	}
	for _, r := range s {
		ok := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}
