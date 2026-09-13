package focus

import (
	"testing"
	"time"
)

func TestPauseTotalWhilePaused(t *testing.T) {
	now := time.Now().UTC()
	pausedAt := now.Add(-3 * time.Minute)
	s := &Session{
		State:       StatePaused,
		StartedAt:   now.Add(-20 * time.Minute),
		PausedAt:    &pausedAt,
		PausedTotal: 2 * time.Minute,
		Duration:    25 * time.Minute,
	}
	got := s.PauseTotal()
	if got < 4*time.Minute+50*time.Second || got > 5*time.Minute+10*time.Second {
		t.Fatalf("PauseTotal=%s want ~5m (2m prior + 3m current)", got)
	}
}

func TestPauseTotalWhileRunning(t *testing.T) {
	s := &Session{
		State:       StateRunning,
		StartedAt:   time.Now().UTC(),
		PausedTotal: 4 * time.Minute,
		Duration:    25 * time.Minute,
	}
	if got := s.PauseTotal(); got != 4*time.Minute {
		t.Fatalf("PauseTotal=%s want 4m", got)
	}
}
