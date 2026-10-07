package listarchive

import (
	"crypto/rand"
	"encoding/hex"
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

// Record is a local snapshot taken before a list or folder is deleted.
type Record struct {
	ArchiveID    string            `json:"archiveId"`
	Kind         string            `json:"kind"` // list or folder
	Name         string            `json:"name"`
	Project      json.RawMessage   `json:"project,omitempty"`
	Tasks        []json.RawMessage `json:"tasks,omitempty"`
	ChildListIDs []string          `json:"childListIds,omitempty"`
	ArchivedAt   time.Time         `json:"archivedAt"`
	RestoredAt   time.Time         `json:"restoredAt,omitempty"`
	NewProjectID string            `json:"newProjectId,omitempty"`
}

func (r Record) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("list name required")
	}
	switch r.Kind {
	case "list":
		if !json.Valid(r.Project) {
			return fmt.Errorf("valid project snapshot required")
		}
	case "folder":
	default:
		return fmt.Errorf("kind must be list or folder")
	}
	return nil
}

func (r Record) Pending() bool {
	return r.RestoredAt.IsZero()
}

type diskStore struct {
	Version int      `json:"version"`
	Records []Record `json:"records"`
}

// Store persists deleted-list snapshots so they can be recreated.
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
		return filepath.Join(root, "ttcli", "list-archive.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "ttcli", "list-archive.json"), nil
}

func (s *Store) Put(record Record) (Record, error) {
	if record.ArchiveID == "" {
		record.ArchiveID = newArchiveID()
	}
	if record.ArchivedAt.IsZero() {
		record.ArchivedAt = time.Now()
	}
	if err := record.Validate(); err != nil {
		return Record{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return Record{}, err
	}
	records = append(records, record)
	if err := s.saveUnlocked(records); err != nil {
		return Record{}, err
	}
	return record, nil
}

func (s *Store) LatestPending() (Record, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return Record{}, false, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].ArchivedAt.After(records[j].ArchivedAt)
	})
	for _, record := range records {
		if record.Pending() {
			return record, true, nil
		}
	}
	return Record{}, false, nil
}

func (s *Store) NoteCreated(archiveID, newProjectID string) error {
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
		records[i].NewProjectID = newProjectID
		return s.saveUnlocked(records)
	}
	return fmt.Errorf("list archive %q not found", archiveID)
}

func (s *Store) MarkRestored(archiveID, newProjectID string) error {
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
		records[i].RestoredAt = time.Now()
		records[i].NewProjectID = newProjectID
		return s.saveUnlocked(records)
	}
	return fmt.Errorf("list archive %q not found", archiveID)
}

func (s *Store) Delete(archiveID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	records, err := s.loadUnlocked()
	if err != nil {
		return err
	}
	next := records[:0]
	for _, record := range records {
		if record.ArchiveID != archiveID {
			next = append(next, record)
		}
	}
	return s.saveUnlocked(next)
}

func (s *Store) loadUnlocked() ([]Record, error) {
	if s == nil || s.path == "" {
		return nil, fmt.Errorf("list archive path unavailable")
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
		return nil, fmt.Errorf("parse list archive: %w", err)
	}
	if disk.Version != storeVersion {
		return nil, fmt.Errorf("unsupported list archive version %d", disk.Version)
	}
	return disk.Records, nil
}

func (s *Store) saveUnlocked(records []Record) error {
	raw, err := json.MarshalIndent(diskStore{Version: storeVersion, Records: records}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".list-archive-*.tmp")
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
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.path)
}

func newArchiveID() string {
	buf := make([]byte, 8)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
