package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type pomoSessionKind int

const (
	pomoSessionFull pomoSessionKind = iota
	pomoSessionPartial
	pomoSessionUnclaimed
	pomoSessionPause
)

func classifyPomoSession(title string, mins int) pomoSessionKind {
	if isUnclaimedFocusTitle(title) {
		return pomoSessionUnclaimed
	}
	if mins >= ticktick.StandardPomoMinutes-1 {
		return pomoSessionFull
	}
	return pomoSessionPartial
}

func formatPomoEndTime(endClock string) string {
	if endClock == "" {
		return padCellLeft("", pomoEndColW)
	}
	label := pomoEndArrowStyle.Render("──▶ ") + pomoEndStyle.Bold(true).Render(endClock)
	return padCellLeft(label, pomoEndColW)
}

func pomoBarFilledSegments(mins int) int {
	const barW = 8
	if mins < 1 {
		mins = 1
	}
	filled := (mins*barW + ticktick.StandardPomoMinutes/2) / ticktick.StandardPomoMinutes
	if filled < 1 {
		filled = 1
	}
	if filled > barW {
		filled = barW
	}
	return filled
}

func pomoDurationBarStyled(mins int, kind pomoSessionKind, taskColor lipgloss.Color, selected bool) string {
	const barW = 8
	filled := pomoBarFilledSegments(mins)
	barSt := pomoBarStyle
	emptySt := dayEmptySlotStyle
	switch kind {
	case pomoSessionUnclaimed:
		barSt = lipgloss.NewStyle().Foreground(colorRed)
	case pomoSessionPause:
		barSt = lipgloss.NewStyle().Foreground(colorBlue)
	case pomoSessionPartial:
		barSt = lipgloss.NewStyle().Foreground(colorYellow)
	default:
		if taskColor != "" {
			barSt = lipgloss.NewStyle().Foreground(taskColor)
		}
	}
	if selected {
		barSt = barSt.Copy().Bold(true)
	}
	fillChar := "▮"
	switch kind {
	case pomoSessionUnclaimed:
		fillChar = "▪"
	case pomoSessionPause:
		fillChar = "▫"
	}
	return barSt.Render(strings.Repeat(fillChar, filled)) + emptySt.Render(strings.Repeat("▯", barW-filled))
}

func pomoDurationBadgeStyled(mins int, kind pomoSessionKind, taskColor lipgloss.Color, selected bool) string {
	if mins < 1 {
		mins = 1
	}
	switch kind {
	case pomoSessionUnclaimed:
		return unclaimedDurationBadge(mins, selected)
	case pomoSessionPause:
		return pauseDurationBadge(mins, selected)
	case pomoSessionFull:
		bg := taskColor
		if bg == "" || bg == colorMuted {
			bg = colorPeach
		}
		st := lipgloss.NewStyle().Foreground(colorBase).Background(bg).Bold(true).Padding(0, 1)
		return st.Render(fmt.Sprintf("%dm", mins))
	default:
		st := lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Padding(0, 1)
		if selected {
			st = lipgloss.NewStyle().Foreground(colorBase).Background(colorYellow).Bold(true).Padding(0, 1)
		}
		return st.Render(fmt.Sprintf("%dm", mins))
	}
}
