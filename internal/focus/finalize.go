package focus

import (
	"errors"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// LogPlannedIfNeeded records the planned pomodoro slice for the current task when the timer completes.
// Idempotent: claims plannedLogged on disk before calling TickTick so concurrent ticks cannot duplicate.
func LogPlannedIfNeeded(c *ticktick.Client, _ *Session) error {
	if c == nil {
		return nil
	}
	s, err := Load()
	if err != nil {
		return err
	}
	if s.TaskID == "" || s.PlannedLogged {
		return nil
	}
	elapsed := s.PlannedSegmentElapsed()
	if elapsed < MinLogDuration {
		return nil
	}

	// Claim before the network call so parallel complete/stop handlers cannot double-log.
	s.PlannedLogged = true
	if err := s.Save(); err != nil {
		return err
	}

	now := time.Now().UTC()
	_, err = c.LogPomodoro(ticktick.LogPomodoroInput{
		TaskID:        s.TaskID,
		TaskTitle:     s.TaskTitle,
		ProjectName:   s.ProjectName,
		StartedAt:     s.segmentStartedAt(),
		Elapsed:       elapsed,
		PauseDuration: s.SegmentPauseTotal(now),
	})
	if err != nil {
		s.PlannedLogged = false
		_ = s.Save()
		return err
	}
	return nil
}

// LogOvertimeIfNeeded records time after the planned block until dismiss.
func LogOvertimeIfNeeded(c *ticktick.Client, s *Session) error {
	if c == nil || s == nil || s.TaskID == "" {
		return nil
	}
	elapsed := s.OvertimeElapsed()
	if elapsed < MinLogDuration {
		return nil
	}
	start := s.CompletedAt.Add(OvertimeGracePeriod)
	if start.IsZero() {
		start = s.StartedAt.Add(s.Duration + OvertimeGracePeriod)
	}
	_, err := c.LogPomodoro(ticktick.LogPomodoroInput{
		TaskID:      s.TaskID,
		TaskTitle:   OvertimeTitle(s.TaskTitle),
		ProjectName: s.ProjectName,
		StartedAt:   start,
		Elapsed:     elapsed,
	})
	return err
}

// FinalizeDismiss logs planned/overtime to TickTick when possible, then always clears the session file.
// A non-nil return value means TickTick sync failed; the local session is still cleared.
func FinalizeDismiss(c *ticktick.Client) error {
	s, err := Load()
	if err != nil {
		return err
	}
	if !s.Active() {
		return ErrNoSession
	}
	var syncErr error
	if err := LogPlannedIfNeeded(c, s); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	if err := LogOvertimeIfNeeded(c, s); err != nil {
		syncErr = errors.Join(syncErr, err)
	}
	if err := s.FinishAfterDismiss(); err != nil {
		return err
	}
	return syncErr
}
