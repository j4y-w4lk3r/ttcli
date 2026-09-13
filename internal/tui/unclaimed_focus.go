package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

const unclaimedMark = "×"

var (
	unclaimedLabelStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)
	unclaimedTaskStyle = lipgloss.NewStyle().
				Foreground(colorRed)
	unclaimedBlockStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)
)

func isUnclaimedFocusTitle(title string) bool {
	return focus.IsOvertimeTitle(title)
}

func unclaimedBaseTitle(title string) string {
	return focus.UnclaimedBaseTitle(title)
}

func renderUnclaimedMark() string {
	return unclaimedLabelStyle.Render(unclaimedMark)
}

func renderUnclaimedTimelineTitle(title string, selected bool) string {
	base := unclaimedBaseTitle(title)
	taskSt := unclaimedTaskStyle
	if selected {
		taskSt = unclaimedTaskStyle.Copy().Bold(true)
	}
	return renderUnclaimedMark() + " " + taskSt.Render(base)
}

func renderUnclaimedLiveStatus() string {
	return renderUnclaimedMark()
}

func renderUnclaimedLiveHint() string {
	return hintStyle.Render("timer ended — unclaimed time is counting (S stop · D dismiss)")
}

func unclaimedDurationBadge(mins int, selected bool) string {
	if mins < 1 {
		mins = 1
	}
	label := "+" + minsLabel(mins)
	st := lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	if selected {
		st = lipgloss.NewStyle().Foreground(colorBase).Background(colorRed).Bold(true).Padding(0, 1)
	}
	return st.Render(label)
}

func formatUnclaimedPomoEndTime(endClock string) string {
	if endClock == "" {
		return padCellLeft("", pomoEndColW)
	}
	label := unclaimedLabelStyle.Render("──▶ ") + unclaimedLabelStyle.Copy().Bold(true).Render(endClock)
	return padCellLeft(label, pomoEndColW)
}

func formatUnclaimedPomoSuffixColumns(endClock string, mins int, selected bool) string {
	endPart := formatUnclaimedPomoEndTime(endClock)
	barPart := padCellLeft(pomoDurationBarStyled(mins, pomoSessionUnclaimed, colorRed, selected), pomoBarColW)
	durPart := padCellRight(unclaimedDurationBadge(mins, selected), pomoDurExtraColW)
	return endPart + strings.Repeat(" ", pomoColGap) + barPart + " " + durPart
}

func unclaimedDurationBarOnly(mins int, selected bool) string {
	return pomoDurationBarStyled(mins, pomoSessionUnclaimed, colorRed, selected)
}

func minsLabel(mins int) string {
	if mins < 1 {
		mins = 1
	}
	return formatFocusTotal(mins * 60)
}
