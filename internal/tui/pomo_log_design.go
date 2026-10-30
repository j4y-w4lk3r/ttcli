package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func (m model) pomoLogDesign() PomoLogDesign {
	return normalizePomoLogDesign(m.uiSettings.PomoLogDesign)
}

func (m model) pomoTimelineDesignLabel() string {
	name := m.pomoLogDesign().Label()
	if m.pomoLogDesign() == PomoLogRails {
		return name + " · " + m.pomoTimelineDensity().Label()
	}
	return name
}

// buildPomoLogList keeps the logged sessions and drops the empty hour rails.
// sessions is a plain list. log and cards keep an hour label above each group.
func buildPomoLogList(grid []dayGridRow, now time.Time, showNow bool, hours bool) []dayGridRow {
	var out []dayGridRow
	for i := 0; i < len(grid); {
		if grid[i].kind != "slot" {
			i++
			continue
		}
		hour := grid[i].hour
		slot := grid[i]
		i++
		var items []dayGridRow
		for i < len(grid) && grid[i].kind != "slot" {
			if grid[i].kind != "now" && grid[i].kind != "gap" {
				items = append(items, grid[i])
			}
			i++
		}
		if len(items) == 0 && !(showNow && now.Hour() == hour) {
			continue
		}
		if hours {
			out = append(out, slot)
		}
		placed := false
		for _, item := range items {
			if showNow && now.Hour() == hour && !placed && pomoRowStartsAfter(item, now) {
				out = append(out, pomoNowListRow(hour, now))
				placed = true
			}
			out = append(out, item)
		}
		if showNow && now.Hour() == hour && !placed {
			out = append(out, pomoNowListRow(hour, now))
		}
	}
	if len(out) == 0 {
		out = append(out, dayGridRow{kind: "empty"})
	}
	return out
}

func pomoNowListRow(hour int, now time.Time) dayGridRow {
	return dayGridRow{hour: hour, hourLabel: now.Format("15:04"), kind: "now"}
}

func pomoRowStartsAfter(row dayGridRow, now time.Time) bool {
	switch row.kind {
	case "pomo":
		start, ok := focusTimeLocal(row.rec.StartTime)
		return ok && start.After(now)
	case "pause":
		return !row.pause.Start.IsZero() && row.pause.Start.Local().After(now)
	default:
		return false
	}
}

func renderPomoLogRow(design PomoLogDesign, row dayGridRow, now time.Time, selected bool, width int, taskColors map[string]lipgloss.Color, live *focus.Session, layout pomoTimelineLayout) string {
	switch row.kind {
	case "empty":
		return truncateInner(hintStyle.Render("no sessions logged"), width)
	case "slot":
		return renderPomoLogHour(row.hourLabel, width, selected)
	case "now":
		return renderPomoLogNow(row.hourLabel, width, selected)
	case "pause":
		return renderPomoLogPause(design, row.pause, selected, width, layout, now)
	case "live":
		return renderHourLiveRow(live, colorPeach, width, layout, selected)
	default:
		return renderPomoLogSession(design, row, selected, width, taskColors, layout)
	}
}

func renderPomoLogHour(label string, width int, selected bool) string {
	style := hintStyle
	if selected {
		style = pomoNowStyle
	}
	line := style.Render(label) + "  " + sectionRuleStyle.Render(strings.Repeat("─", 12))
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}

func renderPomoLogNow(label string, width int, selected bool) string {
	if label == "" {
		label = "now"
	}
	line := pomoNowStyle.Render("● "+label) + "  " + pomoNowStyle.Render(strings.Repeat("─", 12))
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}

func renderPomoLogSession(design PomoLogDesign, row dayGridRow, selected bool, width int, taskColors map[string]lipgloss.Color, layout pomoTimelineLayout) string {
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
	startAt, _ := focusTimeLocal(row.rec.StartTime)
	endAt, _ := focusTimeLocal(row.rec.EndTime)
	if row.sessionClose {
		return renderHourPomoCloseRow(selected, title, col, end, mins, width, layout, row.sessionPauses, startAt, endAt)
	}
	if row.sessionOpen {
		return renderHourPomoOpenRow(selected, title, col, start, width)
	}
	switch design {
	case PomoLogCards:
		return renderPomoMarkedLine(true, title, col, start, end, mins, classifyPomoSession(title, mins), selected, width, layout)
	case PomoLogTape:
		return renderPomoMarkedLine(false, title, col, start, end, mins, classifyPomoSession(title, mins), selected, width, layout)
	default:
		return renderHourPomoRow(row.pomoSlotIndex, row.pomoSlotCount, selected, title, col, start, end, mins, width, layout, row.sessionPauses, startAt, endAt)
	}
}

func renderPomoLogPause(_ PomoLogDesign, spell focus.PauseSpell, selected bool, width int, layout pomoTimelineLayout, now time.Time) string {
	return renderHourPauseRow(spell, selected, width, layout, now)
}

func renderPomoMarkedLine(card bool, title string, taskColor lipgloss.Color, start, end string, mins int, kind pomoSessionKind, selected bool, width int, layout pomoTimelineLayout) string {
	titleSt := listIdleStyle
	if selected {
		titleSt = listSelStyle
	}
	clock := start
	if end != "" && end != start {
		clock += "–" + end
	}
	var lead string
	if card {
		edge := lipgloss.NewStyle().Foreground(taskColor)
		if selected {
			edge = edge.Bold(true)
		}
		lead = edge.Render("┃") + " "
	} else {
		lead = lipgloss.NewStyle().Foreground(taskColor).Render("·") + " "
	}
	body := lead + pomoTimeStyle.Render(padToWidth(clock, 11)) + " " + titleSt.Render(title)
	bar := padCellLeft(pomoDurationBarStyled(mins, kind, taskColor, selected), pomoBarColW)
	dur := padCellRight(pomoDurationBadgeStyled(mins, kind, taskColor, selected), pomoDurColW+2)
	line := alignPomoRowColumns(body, bar+" "+dur, max(width-2, 20), layout)
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}
