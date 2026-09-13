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
	return renderPauseMark() + " " + taskSt.Render(title)
}

func formatPausePomoEndTime(endClock string) string {
	if endClock == "" {
		return padCellLeft("", pomoEndColW)
	}
	label := pauseLabelStyle.Render("──▶ ") + pauseLabelStyle.Copy().Bold(true).Render(endClock)
	return padCellLeft(label, pomoEndColW)
}

const pomoPauseBreakdownColW = 30

func formatPausePomoSuffixColumns(endClock string, spell focus.PauseSpell, now time.Time, selected bool) string {
	endPart := formatPausePomoEndTime(endClock)
	pauseMins := int(spell.Duration(now).Minutes() + 0.5)
	barPart := padCellLeft(pauseDurationBarOnly(pauseMins, selected), pomoBarColW)
	breakdown := formatPauseWorkBreakdown(spell, now)
	if breakdown == "" {
		breakdown = fmt.Sprintf("%dm", pauseMins)
	}
	durPart := padCellRight(pauseBreakdownBadge(breakdown, selected), pomoPauseBreakdownColW)
	return endPart + barPart + durPart
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

func pauseBreakdownBadge(text string, selected bool) string {
	st := pauseLabelStyle
	if selected {
		st = pauseLabelStyle.Copy().Bold(true)
	}
	return st.Render(text)
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

func pauseDurationBarOnly(mins int, selected bool) string {
	return pomoDurationBarStyled(mins, pomoSessionPause, colorBlue, selected)
}
