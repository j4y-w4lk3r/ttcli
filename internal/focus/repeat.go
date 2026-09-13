package focus

import (
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// MinLogDuration is the minimum focused time before a session is logged to TickTick.
const MinLogDuration = 10 * time.Second

// RepeatAfterDismiss logs overtime (if any), clears the session, and starts the same task again.
func RepeatAfterDismiss(c *ticktick.Client) (*Session, error) {
	s, err := Load()
	if err != nil {
		return nil, err
	}
	if !s.Active() {
		return nil, ErrNoSession
	}
	taskID := s.TaskID
	taskTitle := s.TaskTitle
	projectID := s.ProjectID
	projectName := s.ProjectName
	duration := s.Duration
	segStart := s.segmentStartedAt()
	segElapsed := s.CurrentSegmentElapsed()

	var syncErr error
	if s.State == StateAwaitingDismiss {
		syncErr = FinalizeDismiss(c)
	} else {
		pauseTotal := s.SegmentPauseTotal(time.Now().UTC())
		_, err = s.Finish()
		if err != nil {
			return nil, err
		}
		if segElapsed >= MinLogDuration && taskID != "" {
			if _, err := c.LogPomodoro(ticktick.LogPomodoroInput{
				TaskID:        taskID,
				TaskTitle:     taskTitle,
				ProjectName:   projectName,
				StartedAt:     segStart,
				Elapsed:       segElapsed,
				PauseDuration: pauseTotal,
			}); err != nil {
				syncErr = err
			}
		}
	}
	newS, err := Start(duration, taskID, taskTitle, projectID, projectName)
	if err != nil {
		return nil, err
	}
	return newS, syncErr
}

// Repeat logs the current session (when long enough) and starts a new one with the
// same task and duration.
func Repeat(c *ticktick.Client) (*Session, error) {
	return RepeatAfterDismiss(c)
}
