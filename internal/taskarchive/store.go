package taskarchive

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const storeVersion = 1

// Record is a recoverable task snapshot written before permanent deletion.
type Record struct {
	ArchiveID   string          `json:"archiveId"`
	Task        ticktick.Task   `json:"task"`
	RawTask     json.RawMessage `json:"rawTask"`
	ArchivedAt  time.Time       `json:"archivedAt"`
	RecreatedAt time.Time       `json:"recreatedAt,omitempty"`
	NewTaskID   string          `json:"newTaskId,omitempty"`
}

func (r Record) Validate() error {
	if r.Task.ID == "" {
		return fmt.Errorf("task id required")
	}
	if r.Task.ProjectID == "" {
		return fmt.Errorf("task project id required")
	}
	if !json.Valid(r.RawTask) {
		return fmt.Errorf("valid raw task snapshot required")
	}
	return nil
}

type diskStore struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

// Store persists recoverable task snapshots in XDG state.
type Store struct {
	path string
	mu   sync.Mutex
}

func NewStore() *Store {
	path, _ := DefaultPath()
	return &Store{path: path}
}

func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

func DefaultPath() (string, error) {
	if root := os.Getenv("XDG_STATE_HOME"); root != "" {
		return filepath.Join(root, "ttcli", "task-archive.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "ttcli", "task-archive.json"), nil
}

func (s *Store) loadUnlocked() ([]Record, error) {
	if s == nil || s.path == "" {
		return nil, fmt.Errorf("task archive path unavailable")
	}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var disk diskStore
	if err := json.Unmarshal(raw, &disk); err != nil {
		return nil, fmt.Errorf("parse task archive: %w", err)
	}
	if disk.Version != storeVersion {
		return nil, fmt.Errorf("unsupported task archive version %d", disk.Version)
	}
	return disk.Records, nil
}

func (s *Store) saveUnlocked(records []Record) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("task archive path unavailable")
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ArchivedAt.After(records[j].ArchivedAt)
	})
	raw, err := json.MarshalIndent(diskStore{Version: storeVersion, Records: records}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".task-archive-*.tmp")
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
	return os.Rename(tmpName, s.path)
}

func (s *Store) Records() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	return append([]Record(nil), records...), err
}

func (s *Store) Put(record Record) error {
	if record.ArchiveID == "" {
		record.ArchiveID = record.Task.ProjectID + ":" + record.Task.ID
	}
	if record.ArchivedAt.IsZero() {
		record.ArchivedAt = time.Now()
	}
	if err := record.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	for i := range records {
		if records[i].ArchiveID == record.ArchiveID {
			records[i] = record
			return s.saveUnlocked(records)
		}
	}
	records = append(records, record)
	return s.saveUnlocked(records)
}

func (s *Store) MarkRecreated(archiveID, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	for i := range records {
		if records[i].ArchiveID != archiveID {
			continue
		}
		records[i].RecreatedAt = time.Now()
		records[i].NewTaskID = taskID
		return s.saveUnlocked(records)
	}
	return fmt.Errorf("archive record %s not found", archiveID)
}
