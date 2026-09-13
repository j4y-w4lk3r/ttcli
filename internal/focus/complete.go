package focus

import (
	"fmt"
	"strings"
	"time"
)

const (
	StateAwaitingDismiss = "awaiting_dismiss"
	OvertimeTitlePrefix  = "Unclaimed · "
	// OvertimeGracePeriod is how long after the planned block ends before
	// unclaimed overtime starts counting (matches the focus-alert grace window).
	OvertimeGracePeriod = 45 * time.Second
)

// OvertimeTitle names the post-session slice shown separately in the focus ring.
func OvertimeTitle(taskTitle string) string {
	taskTitle = strings.TrimSpace(taskTitle)
	if taskTitle == "" {
		return OvertimeTitlePrefix + "(untitled)"
	}
	return OvertimeTitlePrefix + taskTitle
}

// IsOvertimeTitle reports whether a logged focus title is unclaimed overtime.
func IsOvertimeTitle(title string) bool {
	return strings.HasPrefix(title, OvertimeTitlePrefix)
}

func (s *Session) IsOvertimeTitle(title string) bool {
	return IsOvertimeTitle(title)
}

// UnclaimedBaseTitle strips the unclaimed prefix from a logged focus title.
func UnclaimedBaseTitle(title string) string {
	if !IsOvertimeTitle(title) {
		return title
	}
	base := strings.TrimPrefix(title, OvertimeTitlePrefix)
	if strings.TrimSpace(base) == "" {
		return "(untitled)"
	}
	return base
}

// EnterAwaitingDismiss marks the planned pomodoro complete. Unclaimed overtime
// begins after OvertimeGracePeriod unless the user dismisses or stops sooner.
func (s *Session) EnterAwaitingDismiss() error {
	if s == nil {
		return ErrNoSession
	}
	if s.State == StateAwaitingDismiss {
		return nil
	}
	if s.State != StateRunning && s.State != StatePaused {
		return fmt.Errorf("session is not active")
	}
	if s.Elapsed() < s.Duration {
		return fmt.Errorf("session not finished yet")
	}
	now := time.Now().UTC()
	if s.State == StatePaused && s.PausedAt != nil {
		s.PausedTotal += time.Since(*s.PausedAt)
		s.PausedAt = nil
	}
	s.State = StateAwaitingDismiss
	s.CompletedAt = now
	return s.Save()
}

// PlannedElapsed returns the scheduled pomodoro length once complete.
func (s *Session) PlannedElapsed() time.Duration {
	if s == nil {
		return 0
	}
	return s.Duration
}

// PlannedSegmentElapsed returns focused time on the current task slice that
// counts as the planned pomodoro. Unclaimed overtime is excluded.
func (s *Session) PlannedSegmentElapsed() time.Duration {
	if s == nil {
		return 0
	}
	planned := s.Duration - s.PriorSegmentsElapsed
	if planned < 0 {
		planned = 0
	}
	if s.State == StateAwaitingDismiss {
		return planned
	}
	seg := s.CurrentSegmentElapsed()
	if planned > 0 && seg > planned {
		return planned
	}
	return seg
}

// OvertimeGraceRemaining is time left before unclaimed overtime begins.
func (s *Session) OvertimeGraceRemaining() time.Duration {
	if s == nil || s.State != StateAwaitingDismiss || s.CompletedAt.IsZero() {
		return 0
	}
	remaining := OvertimeGracePeriod - time.Now().UTC().Sub(s.CompletedAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// InOvertimeGrace reports whether unclaimed time has not started counting yet.
func (s *Session) InOvertimeGrace() bool {
	return s.OvertimeGraceRemaining() > 0
}

// OvertimeElapsed is unclaimed focused time after the grace window until dismiss/stop.
func (s *Session) OvertimeElapsed() time.Duration {
	if s == nil || s.State != StateAwaitingDismiss || s.CompletedAt.IsZero() {
		return 0
	}
	elapsed := time.Now().UTC().Sub(s.CompletedAt) - OvertimeGracePeriod
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// FinishAfterDismiss clears the session after the user dismisses the alert.
func (s *Session) FinishAfterDismiss() error {
	if s == nil {
		return ErrNoSession
	}
	if s.State != StateAwaitingDismiss && s.State != StateRunning && s.State != StatePaused {
		if s.State == StateIdle {
			return ErrNoSession
		}
	}
	s.State = StateIdle
	s.CompletedAt = time.Time{}
	return Clear()
}
