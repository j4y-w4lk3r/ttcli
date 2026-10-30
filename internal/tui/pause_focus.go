package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

const pauseMark = "⏸"

var (
	pauseLabelStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)
	pauseTaskStyle = lipgloss.NewStyle().
			Foreground(colorBlue)
	pauseBlockStyle = lipgloss.NewStyle().
			Foreground(colorBlue)
)

func renderPauseMark() string {
	return pauseLabelStyle.Render(pauseMark)
}

func renderPauseTimelineTitle(taskTitle string, selected bool) string {
	title := strings.TrimSpace(taskTitle)
	if title == "" {
		title = "(untitled)"
	}
	taskSt := pauseTaskStyle
	if selected {
		taskSt = pauseTaskStyle.Copy().Bold(true)
	}
	return taskSt.Render(title)
}

// pauseTimelineClock shows seconds when the pause starts and ends in the same minute.
func pauseTimelineClock(spell focus.PauseSpell) string {
	if spell.Start.IsZero() {
		return ""
	}
	start := spell.Start.Local()
	if spell.End.IsZero() {
		return start.Format("15:04:05")
	}
	end := spell.End.Local()
	if start.Format("15:04") == end.Format("15:04") {
		return start.Format("15:04:05")
	}
	return start.Format("15:04") + "–" + end.Format("15:04")
}

func pauseTimelineLength(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return formatPauseSliceDuration(d)
}

func pauseTimelineContext(spell focus.PauseSpell) string {
	var parts []string
	if spell.WorkBefore >= 30*time.Second {
		parts = append(parts, formatPauseSliceDuration(spell.WorkBefore)+" before")
	}
	if spell.WorkAfter >= 30*time.Second {
		parts = append(parts, formatPauseSliceDuration(spell.WorkAfter)+" after")
	}
	return strings.Join(parts, " · ")
}

func formatPauseTimelineSuffix(spell focus.PauseSpell, now time.Time, selected bool) string {
	length := pauseTimelineLength(spell.Duration(now))
	label := length
	if spell.End.IsZero() {
		label = length + " · still paused"
	}
	st := pauseLabelStyle
	if selected {
		st = pauseLabelStyle.Copy().Bold(true)
	}
	suffix := st.Render(label)
	if ctx := pauseTimelineContext(spell); ctx != "" {
		ctxSt := pauseTaskStyle
		if selected {
			ctxSt = pauseTaskStyle.Copy().Bold(true)
		}
		suffix += "  " + ctxSt.Render(ctx)
	}
	return suffix
}

func formatPauseWorkBreakdown(spell focus.PauseSpell, now time.Time) string {
	if spell.WorkBefore <= 0 && spell.WorkAfter <= 0 {
		return ""
	}
	var parts []string
	if spell.WorkBefore >= 30*time.Second {
		parts = append(parts, formatPauseSliceDuration(spell.WorkBefore)+" work")
	}
	parts = append(parts, formatPauseSliceDuration(spell.Duration(now))+" paused")
	if spell.WorkAfter >= 30*time.Second {
		parts = append(parts, formatPauseSliceDuration(spell.WorkAfter)+" work")
	}
	return strings.Join(parts, " · ")
}

func formatPauseSliceDuration(d time.Duration) string {
	if d < time.Minute {
		return formatClock(d)
	}
	m := int(d.Minutes() + 0.5)
	if m >= 60 {
		return formatClock(d)
	}
	return fmt.Sprintf("%dm", m)
}

func pauseDurationBadge(mins int, selected bool) string {
	if mins < 1 {
		mins = 1
	}
	st := lipgloss.NewStyle().Foreground(colorBlue).Bold(true).Padding(0, 1)
	if selected {
		st = lipgloss.NewStyle().Foreground(colorBase).Background(colorBlue).Bold(true).Padding(0, 1)
	}
	return st.Render(fmt.Sprintf("%dm", mins))
}
