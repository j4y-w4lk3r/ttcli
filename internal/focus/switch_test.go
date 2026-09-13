package focus

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCurrentSegmentElapsedAfterSwitch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cache", "ttcli", "focus-session.json")
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	t.Cleanup(func() { t.Setenv("HOME", oldHome) })

	start := time.Now().UTC().Add(-10 * time.Minute)
	s := &Session{
		State:                StateRunning,
		StartedAt:            start,
		SegmentStartedAt:     start.Add(7 * time.Minute),
		Duration:             25 * time.Minute,
		TaskID:               "b",
		TaskTitle:            "food",
		PriorSegmentsElapsed: 7 * time.Minute,
	}
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	seg := loaded.CurrentSegmentElapsed()
	if seg < 2*time.Minute || seg > 4*time.Minute {
		t.Fatalf("segment elapsed=%s want ~3m", seg)
	}
	total := loaded.Elapsed()
	if total < 9*time.Minute || total > 11*time.Minute {
		t.Fatalf("total elapsed=%s want ~10m", total)
	}
}

func TestSwitchTaskUpdatesSessionFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	_, err := Start(25*time.Minute, "a", "desk setup", "p1", "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)

	s, err := SwitchTask(nil, "b", "food", "p1", "Inbox")
	if err != nil {
		t.Fatal(err)
	}
	if s.TaskID != "b" || s.TaskTitle != "food" {
		t.Fatalf("task=%q id=%q", s.TaskTitle, s.TaskID)
	}
	if s.PriorSegmentsElapsed <= 0 {
		t.Fatal("expected prior segment elapsed to be recorded")
	}
	if s.SegmentStartedAt.IsZero() {
		t.Fatal("segment start should be set")
	}
	if s.State != StateRunning {
		t.Fatalf("state=%q want running", s.State)
	}
}
