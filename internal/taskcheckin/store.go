package taskcheckin

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const storeVersion = 1

// Record is one task's completion/check-in history on a local calendar day.
// Native records mirror a TickTick recurring occurrence; local records are
// authoritative only inside ttcli.
type Record struct {
	TaskID      string    `json:"taskId"`
	SeriesID    string    `json:"seriesId"`
	ProjectID   string    `json:"projectId"`
	Title       string    `json:"title"`
	Date        string    `json:"date"`
	CompletedAt time.Time `json:"completedAt"`
	Native      bool      `json:"native"`
}

func (r Record) Validate() error {
	if r.TaskID == "" {
		return fmt.Errorf("task id required")
	}
	if r.SeriesID == "" {
		r.SeriesID = r.TaskID
	}
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return fmt.Errorf("invalid check-in date %q", r.Date)
	}
	return nil
}

func (r Record) key() string {
	seriesID := r.SeriesID
	if seriesID == "" {
		seriesID = r.TaskID
	}
	return seriesID + "\x00" + r.Date
}

type diskStore struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

// Store persists durable user-authored check-in history.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore uses XDG_STATE_HOME, falling back to ~/.local/state.
func NewStore() *Store {
	path, _ := DefaultPath()
	return &Store{path: path}
}

// NewStoreAt creates a store at an explicit path, primarily for tests.
func NewStoreAt(path string) *Store {
	return &Store{path: path}
}

// DefaultPath returns the durable task check-in journal path.
func DefaultPath() (string, error) {
	if root := os.Getenv("XDG_STATE_HOME"); root != "" {
		return filepath.Join(root, "ttcli", "task-checkins.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "ttcli", "task-checkins.json"), nil
}

func (s *Store) loadUnlocked() ([]Record, error) {
	if s == nil || s.path == "" {
		return nil, nil
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
		return nil, fmt.Errorf("parse task check-ins: %w", err)
	}
	if disk.Version != storeVersion {
		return nil, fmt.Errorf("unsupported task check-in version %d", disk.Version)
	}
	return disk.Records, nil
}

func (s *Store) saveUnlocked(records []Record) error {
	if s == nil || s.path == "" {
		return fmt.Errorf("task check-in path unavailable")
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].Date != records[j].Date {
			return records[i].Date < records[j].Date
		}
		return records[i].key() < records[j].key()
	})
	raw, err := json.MarshalIndent(diskStore{Version: storeVersion, Records: records}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".task-checkins-*.tmp")
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

// Records returns all persisted records.
func (s *Store) Records() ([]Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	return append([]Record(nil), records...), err
}

// Toggle inserts a record or removes the existing record for its series/date.
// The returned bool reports the new checked-in state.
func (s *Store) Toggle(record Record) (bool, error) {
	if record.SeriesID == "" {
		record.SeriesID = record.TaskID
	}
	if record.CompletedAt.IsZero() {
		record.CompletedAt = time.Now()
	}
	if err := record.Validate(); err != nil {
		return false, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return false, err
	}
	key := record.key()
	for i, existing := range records {
		if existing.key() == key {
			records = append(records[:i], records[i+1:]...)
			return false, s.saveUnlocked(records)
		}
	}
	records = append(records, record)
	return true, s.saveUnlocked(records)
}

// Put inserts or replaces a record without toggling it off.
func (s *Store) Put(record Record) error {
	if record.SeriesID == "" {
		record.SeriesID = record.TaskID
	}
	if record.CompletedAt.IsZero() {
		record.CompletedAt = time.Now()
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
	key := record.key()
	for i, existing := range records {
		if existing.key() == key {
			records[i] = record
			return s.saveUnlocked(records)
		}
	}
	records = append(records, record)
	return s.saveUnlocked(records)
}

// Remove deletes one series/date record.
func (s *Store) Remove(seriesID, date string) error {
	if seriesID == "" {
		return fmt.Errorf("series id required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	key := seriesID + "\x00" + date
	for i, existing := range records {
		if existing.key() == key {
			records = append(records[:i], records[i+1:]...)
			return s.saveUnlocked(records)
		}
	}
	return nil
}

// On reports a record for a series/date.
func (s *Store) On(seriesID, date string) (Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return Record{}, false, err
	}
	key := seriesID + "\x00" + date
	for _, record := range records {
		if record.key() == key {
			return record, true, nil
		}
	}
	return Record{}, false, nil
}
