package focus_test

import (
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestOvertimeTitle(t *testing.T) {
	got := focus.OvertimeTitle("Write docs")
	want := "Unclaimed · Write docs"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestUnclaimedBaseTitle(t *testing.T) {
	got := focus.UnclaimedBaseTitle(focus.OvertimeTitle("Write docs"))
	if got != "Write docs" {
		t.Fatalf("got %q want Write docs", got)
	}
	if focus.UnclaimedBaseTitle("Normal task") != "Normal task" {
		t.Fatal("expected plain title unchanged")
	}
}

func TestEnterAwaitingDismissAndOvertime(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	s, err := focus.Start(2*time.Minute, "t1", "Task", "p1", "Proj")
	if err != nil {
		t.Fatal(err)
	}
	s.StartedAt = time.Now().UTC().Add(-2 * time.Minute)
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := focus.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := loaded.EnterAwaitingDismiss(); err != nil {
		t.Fatal(err)
	}
	if loaded.State != focus.StateAwaitingDismiss {
		t.Fatalf("state=%q", loaded.State)
	}
	if loaded.PlannedElapsed() != 2*time.Minute {
		t.Fatalf("planned=%s", loaded.PlannedElapsed())
	}
	if !loaded.InOvertimeGrace() {
		t.Fatal("expected grace period immediately after completion")
	}
	if ot := loaded.OvertimeElapsed(); ot != 0 {
		t.Fatalf("expected no overtime during grace, got %s", ot)
	}

	loaded.CompletedAt = time.Now().UTC().Add(-focus.OvertimeGracePeriod - time.Second)
	if err := loaded.Save(); err != nil {
		t.Fatal(err)
	}
	loaded, err = focus.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.InOvertimeGrace() {
		t.Fatal("grace should have expired")
	}
	ot := loaded.OvertimeElapsed()
	if ot <= 0 {
		t.Fatalf("expected overtime after grace, got %s", ot)
	}
}

func TestPlannedSegmentElapsedExcludesOvertime(t *testing.T) {
	s := &focus.Session{
		State:                focus.StateAwaitingDismiss,
		Duration:             25 * time.Minute,
		PriorSegmentsElapsed: 0,
		StartedAt:            time.Now().UTC().Add(-90 * time.Minute),
		CompletedAt:          time.Now().UTC().Add(-60 * time.Minute),
		SegmentStartedAt:     time.Now().UTC().Add(-90 * time.Minute),
	}
	if got := s.PlannedSegmentElapsed(); got != 25*time.Minute {
		t.Fatalf("planned segment=%s want 25m (not %s current segment)", got, s.CurrentSegmentElapsed())
	}
}

func TestFinalizeDismissClearsSessionWithNilClient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	s, err := focus.Start(time.Minute, "t1", "Task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	s.State = focus.StateAwaitingDismiss
	s.CompletedAt = time.Now().UTC()
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := focus.FinalizeDismiss(nil); err != nil {
		t.Fatal(err)
	}
	loaded, err := focus.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active() {
		t.Fatal("expected idle session after FinalizeDismiss")
	}
}

func TestFinishAfterDismissClearsSession(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	s, err := focus.Start(time.Minute, "t1", "Task", "", "")
	if err != nil {
		t.Fatal(err)
	}
	s.State = focus.StateAwaitingDismiss
	s.CompletedAt = time.Now().UTC()
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishAfterDismiss(); err != nil {
		t.Fatal(err)
	}
	loaded, err := focus.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Active() {
		t.Fatal("expected idle session after dismiss")
	}
}
