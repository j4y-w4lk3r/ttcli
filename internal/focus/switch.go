package focus

import (
	"fmt"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// SwitchTask logs focused time on the current task and continues the same timer
// on a new task without resetting the planned duration or elapsed total.
func SwitchTask(c *ticktick.Client, taskID, taskTitle, projectID, projectName string) (*Session, error) {
	s, err := Load()
	if err != nil {
		return nil, err
	}
	if s == nil || !s.Active() || s.State == StateAwaitingDismiss {
		return nil, ErrNoSession
	}
	if taskID == "" {
		return nil, fmt.Errorf("task required")
	}
	if taskID == s.TaskID {
		return s, nil
	}
	if err := LogCurrentSegment(c, s); err != nil {
		return nil, err
	}
	prev := s.TaskTitle
	s.PriorSegmentsElapsed = s.Elapsed()
	s.TaskID = taskID
	s.TaskTitle = taskTitle
	s.ProjectID = projectID
	s.ProjectName = projectName
	s.SegmentStartedAt = time.Now().UTC()
	s.PlannedLogged = false
	if err := s.Save(); err != nil {
		return nil, err
	}
	sessionlog.Appendf("session_switch", "from=%q to=%q elapsed=%s", prev, taskTitle, s.PriorSegmentsElapsed)
	return s, nil
}

// LogCurrentSegment writes the active task slice to TickTick when long enough.
func LogCurrentSegment(c *ticktick.Client, s *Session) error {
	if c == nil || s == nil || s.TaskID == "" {
		return nil
	}
	elapsed := s.CurrentSegmentElapsed()
	if elapsed < MinLogDuration {
		return nil
	}
	_, err := c.LogPomodoro(ticktick.LogPomodoroInput{
		TaskID:      s.TaskID,
		TaskTitle:   s.TaskTitle,
		ProjectName: s.ProjectName,
		StartedAt:   s.segmentStartedAt(),
		Elapsed:     elapsed,
	})
	return err
}
