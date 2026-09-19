package ticktick

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const snapshotVersion = 1

// CacheMeta describes where a repository result came from.
type CacheMeta struct {
	SavedAt   time.Time
	FromCache bool
	Stale     bool
}

type cacheEntry[T any] struct {
	Value   T         `json:"value"`
	SavedAt time.Time `json:"savedAt"`
	Dirty   bool      `json:"dirty,omitempty"`
}

func (e cacheEntry[T]) present() bool {
	return !e.SavedAt.IsZero()
}

func (e cacheEntry[T]) fresh(now time.Time, ttl time.Duration) bool {
	return e.present() && !e.Dirty && now.Sub(e.SavedAt) < ttl
}

type TreeSnapshot struct {
	Groups   []ProjectGroup `json:"groups"`
	Projects []Project      `json:"projects"`
	InboxID  string         `json:"inboxId"`
}

type HabitsSnapshot struct {
	Day      string                  `json:"day"`
	Habits   []Habit                 `json:"habits"`
	Checkins map[string]HabitCheckin `json:"checkins"`
}

type dataSnapshot struct {
	Version       int                               `json:"version"`
	Tree          cacheEntry[TreeSnapshot]          `json:"tree"`
	ProjectTasks  map[string]cacheEntry[[]Task]     `json:"projectTasks,omitempty"`
	OpenTasks     cacheEntry[[]Task]                `json:"openTasks"`
	CompletedDays map[string]cacheEntry[[]Task]     `json:"completedDays,omitempty"`
	FocusDays     map[string]cacheEntry[FocusStats] `json:"focusDays,omitempty"`
	FocusHistory  cacheEntry[[]FocusRecord]         `json:"focusHistory"`
	Habits        cacheEntry[HabitsSnapshot]        `json:"habits"`
}

func newDataSnapshot() dataSnapshot {
	return dataSnapshot{
		Version:       snapshotVersion,
		ProjectTasks:  map[string]cacheEntry[[]Task]{},
		CompletedDays: map[string]cacheEntry[[]Task]{},
		FocusDays:     map[string]cacheEntry[FocusStats]{},
	}
}

func defaultSnapshotPath() (string, error) {
	if root := os.Getenv("XDG_CACHE_HOME"); root != "" {
		return filepath.Join(root, "ttcli", "data-v1.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "data-v1.json"), nil
}

func loadDataSnapshot(path string) dataSnapshot {
	snap := newDataSnapshot()
	if path == "" {
		return snap
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return snap
	}
	if err := json.Unmarshal(raw, &snap); err != nil || snap.Version != snapshotVersion {
		return newDataSnapshot()
	}
	if snap.ProjectTasks == nil {
		snap.ProjectTasks = map[string]cacheEntry[[]Task]{}
	}
	if snap.CompletedDays == nil {
		snap.CompletedDays = map[string]cacheEntry[[]Task]{}
	}
	if snap.FocusDays == nil {
		snap.FocusDays = map[string]cacheEntry[FocusStats]{}
	}
	return snap
}

func writeDataSnapshot(path string, snap dataSnapshot) error {
	if path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".data-v1-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	dirHandle, err := os.Open(dir)
	if err == nil {
		err = dirHandle.Sync()
		_ = dirHandle.Close()
	}
	if err != nil && !errors.Is(err, os.ErrInvalid) {
		return err
	}
	return nil
}
