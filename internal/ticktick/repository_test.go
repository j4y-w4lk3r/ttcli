package ticktick

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func repositoryTestClient(server *httptest.Server) *Client {
	return &Client{
		BaseURL: server.URL,
		http:    server.Client(),
		headers: map[string]string{"Content-Type": "application/json"},
	}
}

func TestSnapshotRoundTripAndCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache", "data-v1.json")
	snapshot := newDataSnapshot()
	snapshot.OpenTasks = cacheEntry[[]Task]{
		Value:   []Task{{ID: "task-1", Title: "cached"}},
		SavedAt: time.Now().Round(time.Second),
	}
	if err := writeDataSnapshot(path, snapshot); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode=%o want 600", got)
	}
	loaded := loadDataSnapshot(path)
	if len(loaded.OpenTasks.Value) != 1 || loaded.OpenTasks.Value[0].Title != "cached" {
		t.Fatalf("loaded=%+v", loaded.OpenTasks.Value)
	}

	if err := os.WriteFile(path, []byte("{not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	corrupt := loadDataSnapshot(path)
	if corrupt.Version != snapshotVersion || corrupt.OpenTasks.present() {
		t.Fatalf("corrupt snapshot was not reset: %+v", corrupt)
	}
}

func TestRepositoryCoalescesAndFallsBackToStaleOpenTasks(t *testing.T) {
	var requests atomic.Int32
	var fail atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/project/all/tasks" {
			http.NotFound(w, r)
			return
		}
		requests.Add(1)
		time.Sleep(20 * time.Millisecond)
		if fail.Load() {
			http.Error(w, "offline", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode([]Task{{
			ID: "task-1", ProjectID: "project-1", Title: "cached task",
		}})
	}))
	defer server.Close()

	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))
	const callers = 8
	var wg sync.WaitGroup
	wg.Add(callers)
	errs := make(chan error, callers)
	for range callers {
		go func() {
			defer wg.Done()
			tasks, _, err := repo.OpenTasks(true)
			if err == nil && (len(tasks) != 1 || tasks[0].ID != "task-1") {
				t.Errorf("tasks=%+v", tasks)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests=%d want 1", got)
	}

	fail.Store(true)
	tasks, meta, err := repo.OpenTasks(true)
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if len(tasks) != 1 || !meta.FromCache || !meta.Stale {
		t.Fatalf("fallback tasks=%+v meta=%+v", tasks, meta)
	}
}

func TestRepositoryInvalidationForcesRefresh(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_ = json.NewEncoder(w).Encode([]Task{{ID: "task-1"}})
	}))
	defer server.Close()
	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))

	if _, _, err := repo.OpenTasks(false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repo.OpenTasks(false); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("requests before invalidation=%d", got)
	}
	repo.InvalidateTasks("project-1")
	if _, _, err := repo.OpenTasks(false); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 2 {
		t.Fatalf("requests after invalidation=%d", got)
	}
}

func TestRepositoryFiltersCrossProjectTaskLeaks(t *testing.T) {
	const projectID = "0123456789abcdef01234567"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Task{
			{ID: "wanted", ProjectID: projectID, Title: "wanted"},
			{ID: "leaked", ProjectID: "another-project", Title: "leaked"},
			{ID: "legacy-empty-project", Title: "legacy"},
		})
	}))
	defer server.Close()
	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))
	tasks, _, err := repo.ProjectTasks(projectID, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != "wanted" || tasks[1].ID != "legacy-empty-project" {
		t.Fatalf("project tasks=%+v", tasks)
	}
}

func TestProjectTasksMergesCompletedHistoryPastTheEmbeddedSample(t *testing.T) {
	const projectID = "prio"
	completedAt := "2026-09-24T16:00:00.000+0000"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/completed/"):
			_ = json.NewEncoder(w).Encode([]Task{
				{ID: "embedded", ProjectID: projectID, Title: "already listed", Status: 2, CompletedT: completedAt},
				{ID: "just-1", ProjectID: projectID, Title: "just finished", Status: 2, CompletedT: completedAt},
				{ID: "just-2", ProjectID: projectID, Title: "just finished too", Status: 2, CompletedT: completedAt},
				{ID: "open", ProjectID: projectID, Title: "still open", Status: 2, CompletedT: completedAt},
				{ID: "other-list", ProjectID: "elsewhere", Title: "elsewhere", Status: 2, CompletedT: completedAt},
				{ID: "unmarked", ProjectID: projectID, Title: "closed without status", CompletedT: completedAt},
			})
		default:
			_ = json.NewEncoder(w).Encode([]Task{
				{ID: "open", ProjectID: projectID, Title: "still open", Status: 0},
				{ID: "embedded", ProjectID: projectID, Title: "already listed", Status: 2, CompletedT: completedAt},
			})
		}
	}))
	defer server.Close()

	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))
	tasks, _, err := repo.ProjectTasks(projectID, true)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Task{}
	for _, task := range tasks {
		byID[task.ID] = task
	}
	if len(tasks) != 5 {
		t.Fatalf("tasks=%+v", tasks)
	}
	if _, ok := byID["other-list"]; ok {
		t.Fatal("completed task from another list leaked in")
	}
	if byID["open"].Status.Int() != 0 {
		t.Fatalf("open task was overwritten by completed history: %+v", byID["open"])
	}
	for _, id := range []string{"just-1", "just-2", "unmarked"} {
		task, ok := byID[id]
		if !ok || !task.Done() {
			t.Fatalf("missing completed task %s in %+v", id, tasks)
		}
	}
}

func TestSmartCompletedOmitsWontDo(t *testing.T) {
	completedAt := time.Now().UTC().Format(ticktickTimeLayout)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/all/completed/"):
			_ = json.NewEncoder(w).Encode([]Task{
				{ID: "done", Title: "finished", Status: 2, CompletedT: completedAt},
				{ID: "skip", Title: "abandoned", Status: -1, CompletedT: completedAt},
			})
		default:
			if r.URL.Query().Get("status") != "Abandoned" {
				t.Errorf("status=%q", r.URL.Query().Get("status"))
			}
			_ = json.NewEncoder(w).Encode([]Task{
				{ID: "skip", Title: "abandoned", Status: -1, CompletedT: completedAt},
			})
		}
	}))
	defer server.Close()
	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))
	completed, _, err := repo.SmartTasks("completed", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 || completed[0].ID != "done" {
		t.Fatalf("completed=%+v", completed)
	}
	abandoned, _, err := repo.SmartTasks("abandoned", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(abandoned) != 1 || abandoned[0].ID != "skip" {
		t.Fatalf("abandoned=%+v", abandoned)
	}
}

func TestRepositoryFocusBetweenUsesOneRangeRequestAndCachesDays(t *testing.T) {
	var requests atomic.Int32
	loc := time.Local
	first := time.Date(2026, 9, 1, 0, 0, 0, 0, loc)
	start := first.Add(9 * time.Hour)
	end := start.Add(25 * time.Minute)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/pomodoros" {
			http.NotFound(w, r)
			return
		}
		requests.Add(1)
		_ = json.NewEncoder(w).Encode([]FocusRecord{{
			ID: "focus-1", StartTime: start.UTC().Format(ticktickTimeLayout),
			EndTime: end.UTC().Format(ticktickTimeLayout),
		}})
	}))
	defer server.Close()
	repo := newRepository(repositoryTestClient(server), filepath.Join(t.TempDir(), "data.json"))
	days, _, err := repo.FocusBetween(first, first.AddDate(0, 0, 1), true)
	if err != nil {
		t.Fatal(err)
	}
	if days[first.Format("2006-01-02")].FullPomoCount != 1 {
		t.Fatalf("days=%+v", days)
	}
	if _, _, err := repo.FocusBetween(first, first.AddDate(0, 0, 1), false); err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != 1 {
		t.Fatalf("range requests=%d want 1", got)
	}
}
