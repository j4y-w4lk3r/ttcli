package focus

import (
	"strings"
	"time"
)

// PauseInterval is one pause spell within a focus session.
type PauseInterval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end,omitempty"`
}

// PauseTitlePrefix labels aggregated pause time in the pomo legend.
const PauseTitlePrefix = "Paused · "

// PauseLegendTitle builds a legend key for pause time on a task.
func PauseLegendTitle(taskTitle string) string {
	taskTitle = strings.TrimSpace(taskTitle)
	if taskTitle == "" {
		taskTitle = "(untitled)"
	}
	return PauseTitlePrefix + taskTitle
}

// IsPauseLegendTitle reports legend rows that represent paused time.
func IsPauseLegendTitle(title string) bool {
	return strings.HasPrefix(title, PauseTitlePrefix)
}

// PauseLegendBaseTitle strips the pause prefix from a legend title.
func PauseLegendBaseTitle(title string) string {
	if !IsPauseLegendTitle(title) {
		return title
	}
	base := strings.TrimPrefix(title, PauseTitlePrefix)
	if strings.TrimSpace(base) == "" {
		return "(untitled)"
	}
	return base
}

// PauseSpell is a pause interval shown on the pomo timeline.
type PauseSpell struct {
	TaskTitle  string
	Start      time.Time
	End        time.Time
	WorkBefore time.Duration // focused work ending at Start (since last start/resume)
	WorkAfter  time.Duration // focused work after End until next pause/stop/complete
}

// Duration returns the pause length; zero End means ongoing (use now).
func (p PauseSpell) Duration(now time.Time) time.Duration {
	if p.Start.IsZero() {
		return 0
	}
	end := p.End
	if end.IsZero() {
		end = now
	}
	d := end.Sub(p.Start)
	if d < 0 {
		return 0
	}
	return d
}

// ActivePauseSpells returns closed and in-progress pause intervals for the session.
func (s *Session) ActivePauseSpells(now time.Time) []PauseSpell {
	if s == nil {
		return nil
	}
	out := make([]PauseSpell, 0, len(s.PauseIntervals)+1)
	for _, iv := range s.PauseIntervals {
		if iv.Start.IsZero() {
			continue
		}
		out = append(out, PauseSpell{
			TaskTitle: s.TaskTitle,
			Start:     iv.Start,
			End:       iv.End,
		})
	}
	if s.State == StatePaused && s.PausedAt != nil {
		out = append(out, PauseSpell{
			TaskTitle: s.TaskTitle,
			Start:     *s.PausedAt,
		})
	}
	return out
}

// SegmentPauseTotal is pause time since the current task slice began.
func (s *Session) SegmentPauseTotal(now time.Time) time.Duration {
	if s == nil {
		return 0
	}
	segStart := s.segmentStartedAt()
	var total time.Duration
	for _, spell := range s.ActivePauseSpells(now) {
		if spell.End.IsZero() {
			if spell.Start.Before(segStart) {
				continue
			}
			total += now.UTC().Sub(spell.Start)
			continue
		}
		if !spell.End.After(segStart) {
			continue
		}
		start := spell.Start
		if start.Before(segStart) {
			start = segStart
		}
		total += spell.End.Sub(start)
	}
	return total
}
