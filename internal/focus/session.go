// Package focus tracks a local Pomodoro timer and logs completed sessions to TickTick.
package focus

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
)

const (
	StateIdle    = "idle"
	StateRunning = "running"
	StatePaused  = "paused"
)

// Session is persisted while a Pomodoro is active on this machine.
type Session struct {
	State        string        `json:"state"`
	StartedAt    time.Time     `json:"startedAt"`
	CompletedAt  time.Time     `json:"completedAt,omitempty"`
	PausedAt        *time.Time        `json:"pausedAt,omitempty"`
	PausedTotal     time.Duration     `json:"pausedTotalNs"`
	PauseIntervals  []PauseInterval   `json:"pauseIntervals,omitempty"`
	Duration     time.Duration `json:"durationNs"`
	TaskID       string        `json:"taskId,omitempty"`
	TaskTitle    string        `json:"taskTitle,omitempty"`
	ProjectID    string        `json:"projectId,omitempty"`
	ProjectName  string        `json:"projectName,omitempty"`
	PlannedLogged bool         `json:"plannedLogged,omitempty"`
	// SegmentStartedAt is when the current task slice began (reset on SwitchTask).
	SegmentStartedAt time.Time `json:"segmentStartedAt,omitempty"`
	// PriorSegmentsElapsed is focused time already logged on earlier tasks this block.
	PriorSegmentsElapsed time.Duration `json:"priorSegmentsElapsedNs,omitempty"`
}

// ErrNoSession is returned when no timer is active.
var ErrNoSession = errors.New("no focus session")

// SessionPath returns ~/.cache/ttcli/focus-session.json.
func SessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "focus-session.json"), nil
}

func idleSession() *Session {
	return &Session{State: StateIdle}
}

// Load reads the session file, or an idle session if missing.
// On error the returned session is still non-nil (idle) so callers can safely inspect it.
func Load() (*Session, error) {
	path, err := SessionPath()
	if err != nil {
		return idleSession(), err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return idleSession(), nil
		}
		return idleSession(), err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return idleSession(), err
	}
	if s.State == "" {
		s.State = StateIdle
	}
	s.normalizeSegmentFields()
	return &s, nil
}

func (s *Session) normalizeSegmentFields() {
	if s == nil {
		return
	}
	if s.SegmentStartedAt.IsZero() && !s.StartedAt.IsZero() {
		s.SegmentStartedAt = s.StartedAt
	}
}

// Save writes the session file.
func (s *Session) Save() error {
	path, err := SessionPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// Clear removes the session file.
func Clear() error {
	path, err := SessionPath()
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// Start begins a new Pomodoro. Any existing session is replaced.
func Start(duration time.Duration, taskID, taskTitle, projectID, projectName string) (*Session, error) {
	if duration <= 0 {
		return nil, fmt.Errorf("duration must be positive")
	}
	s := &Session{
		State:            StateRunning,
		StartedAt:        time.Now().UTC(),
		SegmentStartedAt: time.Now().UTC(),
		Duration:         duration,
		TaskID:           taskID,
		TaskTitle:        taskTitle,
		ProjectID:        projectID,
		ProjectName:      projectName,
	}
	if err := s.Save(); err != nil {
		return nil, err
	}
	sessionlog.Appendf("session_start", "task=%q duration=%s state=%s", taskTitle, duration, s.State)
	return s, nil
}

// Pause pauses a running session.
func (s *Session) Pause() error {
	if s.State != StateRunning {
		return fmt.Errorf("session is not running")
	}
	now := time.Now().UTC()
	s.State = StatePaused
	s.PausedAt = &now
	sessionlog.Append("session_pause", s.TaskTitle)
	return s.Save()
}

// Resume resumes a paused session.
func (s *Session) Resume() error {
	if s.State != StatePaused || s.PausedAt == nil {
		return fmt.Errorf("session is not paused")
	}
	now := time.Now().UTC()
	s.PauseIntervals = append(s.PauseIntervals, PauseInterval{
		Start: *s.PausedAt,
		End:   now,
	})
	s.PausedTotal += now.Sub(*s.PausedAt)
	s.PausedAt = nil
	s.State = StateRunning
	sessionlog.Append("session_resume", s.TaskTitle)
	return s.Save()
}

// Elapsed returns focused time excluding pauses.
func (s *Session) Elapsed() time.Duration {
	if s == nil || s.State == StateIdle {
		return 0
	}
	if s.State == StateAwaitingDismiss {
		return s.Duration + s.OvertimeElapsed()
	}
	end := time.Now().UTC()
	if s.State == StatePaused && s.PausedAt != nil {
		end = *s.PausedAt
	}
	elapsed := end.Sub(s.StartedAt) - s.PausedTotal
	if elapsed < 0 {
		return 0
	}
	return elapsed
}

// CurrentSegmentElapsed is focused time on the current task since the last switch.
func (s *Session) CurrentSegmentElapsed() time.Duration {
	if s == nil || s.State == StateIdle {
		return 0
	}
	seg := s.Elapsed() - s.PriorSegmentsElapsed
	if seg < 0 {
		return 0
	}
	return seg
}

func (s *Session) segmentStartedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	if s.SegmentStartedAt.IsZero() {
		return s.StartedAt
	}
	return s.SegmentStartedAt
}

// SegmentLogStart returns the TickTick start time for the current task slice.
func (s *Session) SegmentLogStart() time.Time {
	return s.segmentStartedAt()
}

// PauseTotal returns accumulated pause time, including the current pause spell.
func (s *Session) PauseTotal() time.Duration {
	if s == nil {
		return 0
	}
	total := s.PausedTotal
	if s.State == StatePaused && s.PausedAt != nil {
		total += time.Since(*s.PausedAt)
	}
	if total < 0 {
		return 0
	}
	return total
}

// Remaining returns time left in the target duration.
func (s *Session) Remaining() time.Duration {
	if s == nil || s.State == StateIdle || s.State == StateAwaitingDismiss {
		return 0
	}
	left := s.Duration - s.Elapsed()
	if left < 0 {
		return 0
	}
	return left
}

// Active reports whether a timer is running, paused, or awaiting dismiss after completion.
func (s *Session) Active() bool {
	if s == nil {
		return false
	}
	return s.State == StateRunning || s.State == StatePaused || s.State == StateAwaitingDismiss
}

// Finished reports whether the planned duration has been reached.
func (s *Session) Finished() bool {
	if s == nil {
		return false
	}
	if s.State == StateAwaitingDismiss {
		return true
	}
	return (s.State == StateRunning || s.State == StatePaused) && s.Elapsed() >= s.Duration
}

// Finish stops the session and returns the final elapsed time.
func (s *Session) Finish() (time.Duration, error) {
	if s == nil {
		return 0, ErrNoSession
	}
	if s.State == StateAwaitingDismiss {
		planned := s.Duration
		overtime := s.OvertimeElapsed()
		s.State = StateIdle
		s.CompletedAt = time.Time{}
		if err := Clear(); err != nil {
			return planned + overtime, err
		}
		return planned + overtime, nil
	}
	if !s.Active() {
		return 0, ErrNoSession
	}
	if s.State == StatePaused && s.PausedAt != nil {
		s.PausedTotal += time.Since(*s.PausedAt)
		s.PausedAt = nil
	}
	elapsed := s.Elapsed()
	s.State = StateIdle
	sessionlog.Appendf("session_stop", "task=%q elapsed=%s", s.TaskTitle, elapsed)
	if err := Clear(); err != nil {
		return elapsed, err
	}
	return elapsed, nil
}
