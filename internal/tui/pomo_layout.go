package tui

import (
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

// pomoTimelineLayout aligns →/bar/duration suffix columns across pomo rows.
type pomoTimelineLayout struct {
	suffixStartCol int
}

func computePomoTimelineLayout(grid []dayGridRow, contentW int, taskColors map[string]lipgloss.Color, live *focus.Session, now time.Time) pomoTimelineLayout {
	maxBodyW := 0
	maxSuffixW := 0
	for _, row := range grid {
		if row.kind != "pomo" && row.kind != "live" && row.kind != "pause" {
			continue
		}
		body, suffix := pomoGridRowParts(row, false, taskColors, live, now)
		if body == "" || suffix == "" {
			continue
		}
		if w := lipgloss.Width(body); w > maxBodyW {
			maxBodyW = w
		}
		if w := lipgloss.Width(suffix); w > maxSuffixW {
			maxSuffixW = w
		}
	}
	if maxBodyW == 0 {
		return pomoTimelineLayout{}
	}
	start := maxBodyW + pomoColGap
	maxStart := contentW - maxSuffixW - 1
	if maxStart < 8 {
		maxStart = 8
	}
	if start > maxStart {
		start = maxStart
	}
	return pomoTimelineLayout{suffixStartCol: start}
}

func pomoGridRowParts(row dayGridRow, selected bool, taskColors map[string]lipgloss.Color, live *focus.Session, now time.Time) (body, suffix string) {
	switch row.kind {
	case "pomo":
		title := row.rec.TaskTitle()
		if title == "" {
			title = "(untitled)"
		}
		col := taskColors[title]
		if col == "" {
			col = colorMuted
		}
		mins := int(focusRecordDuration(row.rec).Minutes() + 0.5)
		start := focusRecordClock(row.rec)
		end := focusRecordEndClock(row.rec)
		if end == start {
			end = ""
		}
		if row.inHourGrid {
			return hourPomoRowParts(title, col, start, end, mins, selected)
		}
		return dayPomoRowParts(title, col, start, end, mins, row.pomoSlotIndex, row.pomoSlotCount, selected)
	case "pause":
		return hourPauseRowParts(row.pause, selected, now)
	case "live":
		col := colorPeach
		if live != nil && live.TaskTitle != "" {
			if c := taskColors[live.TaskTitle]; c != "" {
				col = c
			}
		}
		if row.inHourGrid {
			return hourLiveRowParts(live, col)
		}
		return dayLiveRowParts(now.Format("15:04"), live, col)
	default:
		return "", ""
	}
}

func hourPauseRowParts(spell focus.PauseSpell, selected bool, now time.Time) (body, suffix string) {
	startClock := spell.Start.Local().Format("15:04")
	endClock := ""
	if !spell.End.IsZero() {
		endClock = spell.End.Local().Format("15:04")
	} else {
		endClock = now.Local().Format("15:04")
	}
	block := pauseBlockStyle.Render("▌")
	start := pomoTimeStyle.Render(startClock) + " "
	label := pauseLabelStyle.Render("paused")
	body = block + start + label + " · " + renderPauseTimelineTitle(spell.TaskTitle, selected)
	return body, formatPausePomoSuffixColumns(endClock, spell, now, selected)
}

func hourPomoRowParts(title string, taskColor lipgloss.Color, startClock, endClock string, mins int, selected bool) (body, suffix string) {
	if isUnclaimedFocusTitle(title) {
		block := unclaimedBlockStyle.Render("▌")
		start := ""
		if startClock != "" {
			start = pomoTimeStyle.Render(startClock) + " "
		}
		return block + start + renderUnclaimedTimelineTitle(title, selected),
			formatUnclaimedPomoSuffixColumns(endClock, mins, selected)
	}
	titleSt := listIdleStyle
	blockSt := lipgloss.NewStyle().Foreground(taskColor)
	if selected {
		titleSt = listSelStyle
		blockSt = lipgloss.NewStyle().Foreground(taskColor).Bold(true)
	}
	block := blockSt.Render("▌")
	start := ""
	if startClock != "" {
		start = pomoTimeStyle.Render(startClock) + " "
	}
	return block + start + titleSt.Render(title), formatPomoSuffixColumns(endClock, mins, selected, title, taskColor)
}

func dayPomoRowParts(title string, taskColor lipgloss.Color, clock, endClock string, mins, index, total int, selected bool) (body, suffix string) {
	conn := dayPomoConnector(index, total, selected)
	if isUnclaimedFocusTitle(title) {
		return conn + " " + renderUnclaimedTimelineTitle(title, selected),
			formatUnclaimedPomoSuffixColumns(endClock, mins, selected)
	}
	titleSt := listIdleStyle
	if selected {
		titleSt = listSelStyle
	}
	dot := lipgloss.NewStyle().Foreground(taskColor).Render("●")
	return conn + " " + dot + " " + titleSt.Render(title), formatPomoSuffixColumns(endClock, mins, selected, title, taskColor)
}

func hourLiveRowParts(sess *focus.Session, taskColor lipgloss.Color) (body, suffix string) {
	if sess == nil || !sess.Active() {
		return "", ""
	}
	title := sess.TaskTitle
	if title == "" {
		title = "(untitled)"
	}
	titleSt := taskSelStyle
	blockSt := lipgloss.NewStyle().Foreground(taskColor).Bold(true)
	status := pomoNowStyle.Render("live")
	if sess.State == focus.StatePaused {
		status = hintStyle.Render("paused")
	} else if sess.State == focus.StateAwaitingDismiss {
		if sess.InOvertimeGrace() {
			status = hintStyle.Render("grace")
			titleSt = taskSelStyle
		} else {
			status = renderUnclaimedLiveStatus()
			titleSt = unclaimedTaskStyle.Copy().Bold(true)
			blockSt = unclaimedBlockStyle
		}
	}
	block := blockSt.Render("▌")
	start := ""
	if !sess.SegmentLogStart().IsZero() {
		start = pomoTimeStyle.Render(sess.SegmentLogStart().Local().Format("15:04")) + " "
	}
	body = block + start + status + " · " + titleSt.Render(title)
	if sess.State == focus.StateAwaitingDismiss && sess.InOvertimeGrace() {
		suffix = hintStyle.Render(formatClock(sess.OvertimeGraceRemaining()) + " until unclaimed")
	} else if sess.State == focus.StateAwaitingDismiss {
		suffix = unclaimedLabelStyle.Render("+" + formatClock(sess.OvertimeElapsed()) + " extra")
	} else {
		suffix = liveSessionSuffix(sess)
	}
	return body, suffix
}

func dayLiveRowParts(clock string, sess *focus.Session, taskColor lipgloss.Color) (body, suffix string) {
	if sess == nil || !sess.Active() {
		return "", ""
	}
	title := sess.TaskTitle
	if title == "" {
		title = "(untitled)"
	}
	titleSt := taskSelStyle
	pulse := pomoNowStyle.Render("◉")
	status := pomoNowStyle.Render("live")
	if sess.State == focus.StatePaused {
		pulse = hintStyle.Render("◯")
		status = hintStyle.Render("paused")
	} else if sess.State == focus.StateAwaitingDismiss {
		if sess.InOvertimeGrace() {
			pulse = hintStyle.Render("◉")
			status = hintStyle.Render("grace")
			titleSt = taskSelStyle
		} else {
			status = renderUnclaimedLiveStatus()
			titleSt = unclaimedTaskStyle.Copy().Bold(true)
		}
	}
	conn := pomoBarSelStyle.Render("├─")
	if sess.State == focus.StateAwaitingDismiss && !sess.InOvertimeGrace() {
		body = conn + " " + status + " · " + titleSt.Render(title)
	} else if sess.State == focus.StateAwaitingDismiss {
		dot := lipgloss.NewStyle().Foreground(taskColor).Render("●")
		body = conn + " " + pulse + " " + status + " · " + dot + " " + titleSt.Render(title)
	} else {
		dot := lipgloss.NewStyle().Foreground(taskColor).Render("●")
		body = conn + " " + pulse + " " + status + " · " + dot + " " + titleSt.Render(title)
	}
	if sess.State == focus.StateAwaitingDismiss && sess.InOvertimeGrace() {
		suffix = hintStyle.Render(formatClock(sess.OvertimeGraceRemaining()) + " until unclaimed")
	} else if sess.State == focus.StateAwaitingDismiss {
		suffix = unclaimedLabelStyle.Render("+" + formatClock(sess.OvertimeElapsed()) + " extra")
	} else {
		suffix = liveSessionSuffix(sess)
	}
	_ = clock
	return body, suffix
}
