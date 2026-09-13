package focus

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNeverNil(t *testing.T) {
	s, err := Load()
	if s == nil {
		t.Fatal("Load returned nil session")
	}
	if err != nil {
		t.Fatalf("unexpected load error in normal env: %v", err)
	}
	if s.State != StateIdle && !s.Active() {
		t.Fatalf("unexpected state %q", s.State)
	}
}

func TestActiveNilSafe(t *testing.T) {
	var s *Session
	if s.Active() {
		t.Fatal("nil session should not be active")
	}
	if s.Finished() {
		t.Fatal("nil session should not be finished")
	}
	if s.Elapsed() != 0 || s.Remaining() != 0 {
		t.Fatal("nil session durations should be zero")
	}
}

func TestLoadCorruptFileReturnsIdle(t *testing.T) {
	path, err := SessionPath()
	if err != nil {
		t.Skip(err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if backup == nil {
			_ = os.Remove(path)
		} else {
			_ = os.WriteFile(path, backup, 0o600)
		}
	})

	s, loadErr := Load()
	if s == nil {
		t.Fatal("Load returned nil on corrupt file")
	}
	if loadErr == nil {
		t.Fatal("expected error for corrupt file")
	}
	if s.State != StateIdle {
		t.Fatalf("state=%q want idle", s.State)
	}
}
