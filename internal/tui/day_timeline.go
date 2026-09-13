package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

const (
	dayEmptyRail = "╌"
	daySlotRail  = "─"

	pomoEndColW   = 10 // ──▶ 11:55
	pomoBarColW   = 8 // ▮▯▯▯▯▯▯▯
	pomoDurColW   = 4 // 2m … 60m
	pomoDurExtraColW = 11 // +1h08m … +60m
	pomoColGap    = 2 // space between aligned columns
)

func dayTimelineGap() string { return "  " }

func dayTimelineContentW(totalW int) int {
	gap := dayTimelineGap()
	w := totalW - dayTimeColW - len(gap)
	if w < 12 {
		return 12
	}
	return w
}

func dayRailWidth(totalW int) int {
	w := dayTimelineContentW(totalW)
	if w < 6 {
		return 6
	}
	return w
}

type slotSummary struct {
	sessions int
	mins     int
}

func daySlotMarker(busy bool, totalW int, sum slotSummary) string {
	railW := dayRailWidth(totalW)
	summary := slotSummaryLabel(sum)
	summaryW := 0
	if summary != "" {
		summaryW = lipgloss.Width(" "+hintStyle.Render(summary))
	}
	contentW := railW - summaryW
	if contentW < 4 {
		contentW = 4
	}
	if busy {
		if sum.sessions > 0 {
			fillRatio := float64(sum.mins) / float64(dayHourStep*60)
			if sum.mins <= 0 {
				fillRatio = float64(sum.sessions) / 4.0
			}
			if fillRatio > 1 {
				fillRatio = 1
			}
			if fillRatio > 0 {
				fillLen := int(float64(contentW-1)*fillRatio + 0.5)
				if fillLen < 1 {
					fillLen = 1
				}
				if fillLen > contentW-1 {
					fillLen = contentW - 1
				}
				fill := pomoBarStyle.Render(strings.Repeat("█", fillLen))
				empty := dayEmptySlotStyle.Render(strings.Repeat("░", contentW-1-fillLen))
				rail := sectionRuleStyle.Render("├") + fill + empty
				if summary != "" {
					rail += " " + hintStyle.Render(summary)
				}
				return rail
			}
		}
		rail := sectionRuleStyle.Render("├" + strings.Repeat(daySlotRail, contentW-1))
		if summary != "" {
			rail += " " + hintStyle.Render(summary)
		}
		return rail
	}
	return dayEmptySlotStyle.Render("╌" + strings.Repeat(dayEmptyRail, contentW-1))
}

func slotSummaryLabel(sum slotSummary) string {
	if sum.sessions <= 0 {
		return ""
	}
	if sum.mins > 0 {
		if sum.sessions == 1 {
			return fmt.Sprintf("%dm", sum.mins)
		}
		return fmt.Sprintf("%d · %dm", sum.sessions, sum.mins)
	}
	if sum.sessions == 1 {
		return "1 task"
	}
	return fmt.Sprintf("%d tasks", sum.sessions)
}

func dayPomoConnector(index, total int, selected bool) string {
	ch := "╰─"
	if total > 1 && index < total-1 {
		ch = "├─"
	}
	st := pomoRailStyle
	if selected {
		st = pomoBarSelStyle
	}
	return st.Render(ch)
}

func dayItemPrefix(selected bool) string {
	if selected {
		return pomoRailStyle.Render("▸ ")
	}
	return pomoRailStyle.Render("│ ")
}

func renderDaySlotRow(label string, busy bool, width int, sum slotSummary) string {
	timeCol := dayGridTimeCol(label)
	if !busy && label != "" {
		timeCol = hintStyle.Render(fmtTimeCol(label))
	} else if busy && sum.sessions > 0 {
		timeCol = pomoTimeStyle.Render(fmtTimeCol(label))
	}
	gap := dayTimelineGap()
	rail := daySlotMarker(busy, width, sum)
	return truncateInner(timeCol+gap+rail, width)
}

func fmtTimeCol(label string) string {
	return fmt.Sprintf("%-*s", dayTimeColW, label)
}

func renderDayNowRow(nowLabel string, width int) string {
	timeCol := pomoNowStyle.Render(fmtTimeCol(nowLabel))
	gap := dayTimelineGap()
	marker := pomoNowStyle.Render("● now")
	return truncateInner(timeCol+gap+marker, width)
}

// renderDayTimelineEntry renders a plain-title timeline item with clock in the left column.
func renderDayTimelineEntry(clock string, selected bool, title, suffix string, width int) string {
	titleSt := listIdleStyle
	if selected {
		titleSt = listSelStyle
	}
	prefix := dayItemPrefix(selected)
	return renderDayTimelineEntryBody(clock, prefix+titleSt.Render(title), suffix, width, pomoTimelineLayout{})
}

// renderDayPomoRow renders a focus session on the day timeline.
func renderDayPomoRow(clock string, index, total int, selected bool, title string, taskColor lipgloss.Color, endClock string, mins int, width int, layout pomoTimelineLayout) string {
	body, suffix := dayPomoRowParts(title, taskColor, clock, endClock, mins, index, total, selected)
	return renderDayPomoEntryBody(clock, body, suffix, width, layout)
}

func renderDayLiveRow(clock string, sess *focus.Session, taskColor lipgloss.Color, width int, layout pomoTimelineLayout) string {
	if sess == nil || !sess.Active() {
		return ""
	}
	body, suffix := dayLiveRowParts(clock, sess, taskColor)
	return renderDayTimelineEntryBody(clock, body, suffix, width, layout)
}

// renderDayTimelineEntryBody renders a timeline row; bodyMain is the full styled content after the gap.
func renderDayTimelineEntryBody(clock string, bodyMain, suffix string, width int, layout pomoTimelineLayout) string {
	gap := dayTimelineGap()
	contentW := dayTimelineContentW(width)
	body := bodyMain
	if suffix != "" {
		body = alignPomoRowColumns(bodyMain, suffix, contentW, layout)
	} else if lipgloss.Width(body) > contentW {
		body = truncateRenderedWidth(body, contentW)
	}
	timeCol := dayGridTimeCol(clock)
	return truncateInner(timeCol+gap+body, width)
}

// renderDayPomoEntryBody lays out title and suffix; layout aligns suffix columns across rows.
func renderDayPomoEntryBody(clock string, bodyMain, suffix string, width int, layout pomoTimelineLayout) string {
	gap := dayTimelineGap()
	contentW := dayTimelineContentW(width)
	body := bodyMain
	if suffix != "" {
		body = alignPomoRowColumns(bodyMain, suffix, contentW, layout)
	} else if lipgloss.Width(body) > contentW {
		body = truncateRenderedWidth(body, contentW)
	}
	timeCol := dayGridTimeCol(clock)
	return truncateInner(timeCol+gap+body, width)
}

func alignPomoRowColumns(title, suffix string, contentW int, layout pomoTimelineLayout) string {
	const minGap = pomoColGap
	suffixW := lipgloss.Width(suffix)
	maxTitle := contentW - suffixW - minGap
	if maxTitle < 8 {
		maxTitle = 8
	}
	titlePart := title
	titleW := lipgloss.Width(titlePart)
	if titleW > maxTitle {
		titlePart = truncateRenderedWidth(titlePart, maxTitle)
		titleW = lipgloss.Width(titlePart)
	}

	pad := minGap
	if col := layout.suffixStartCol; col > 0 {
		target := col
		if target+suffixW > contentW {
			target = contentW - suffixW - 1
			if target < 8 {
				target = 8
			}
		}
		if target > titleW {
			pad = target - titleW
		}
	}
	return truncateInner(titlePart+strings.Repeat(" ", pad)+suffix, contentW)
}

func formatPomoSuffixColumns(endClock string, mins int, selected bool, title string, taskColor lipgloss.Color) string {
	kind := classifyPomoSession(title, mins)
	endPart := formatPomoEndTime(endClock)
	barPart := padCellLeft(pomoDurationBarStyled(mins, kind, taskColor, selected), pomoBarColW)
	durPart := padCellRight(pomoDurationBadgeStyled(mins, kind, taskColor, selected), pomoDurColW+2)
	return endPart + strings.Repeat(" ", pomoColGap) + barPart + " " + durPart
}

func padCellLeft(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap > 0 {
		return s + strings.Repeat(" ", gap)
	}
	if gap < 0 {
		return truncateRenderedWidth(s, w)
	}
	return s
}

func padCellRight(s string, w int) string {
	gap := w - lipgloss.Width(s)
	if gap > 0 {
		return strings.Repeat(" ", gap) + s
	}
	if gap < 0 {
		return truncateRenderedWidth(s, w)
	}
	return s
}

func pomoDurationBarFixed(mins int, selected bool, title string, taskColor lipgloss.Color) string {
	return pomoDurationVisual(mins, selected, title, taskColor)
}

func pomoDurationBarOnly(mins int, selected bool, title string, taskColor lipgloss.Color) string {
	kind := classifyPomoSession(title, mins)
	return pomoDurationBarStyled(mins, kind, taskColor, selected)
}

func pomoDurationVisual(mins int, selected bool, title string, taskColor lipgloss.Color) string {
	kind := classifyPomoSession(title, mins)
	return pomoDurationBarStyled(mins, kind, taskColor, selected) + " " + pomoDurationBadgeStyled(mins, kind, taskColor, selected)
}
