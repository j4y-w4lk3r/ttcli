package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const pomoTimelineHours = 24

type indexedFocus struct {
	idx int
	rec ticktick.FocusRecord
	t   time.Time
}

type indexedPause struct {
	idx   int
	spell focus.PauseSpell
	t     time.Time
}

type hourTimelineItem struct {
	start time.Time
	kind  string // "pomo" | "pause"
	pomo  indexedFocus
	pause indexedPause
}

// buildPomoHourGrid builds a 00:00–23:00 day grid with hour rails, focus sessions, and pauses.
func buildPomoHourGrid(records []ticktick.FocusRecord, pauses []focus.PauseSpell, now time.Time, live *focus.Session, showNow bool) []dayGridRow {
	byHour := map[int][]indexedFocus{}
	for i, r := range records {
		t, ok := focusTimeLocal(r.StartTime)
		if !ok {
			continue
		}
		hour := t.Hour()
		byHour[hour] = append(byHour[hour], indexedFocus{idx: i, rec: r, t: t})
	}
	for hour := range byHour {
		sort.Slice(byHour[hour], func(a, b int) bool {
			return byHour[hour][a].t.Before(byHour[hour][b].t)
		})
	}

	byHourPause := map[int][]indexedPause{}
	for i, spell := range pauses {
		if spell.Start.IsZero() {
			continue
		}
		t := spell.Start.Local()
		hour := t.Hour()
		byHourPause[hour] = append(byHourPause[hour], indexedPause{idx: i, spell: spell, t: t})
	}
	for hour := range byHourPause {
		sort.Slice(byHourPause[hour], func(a, b int) bool {
			return byHourPause[hour][a].t.Before(byHourPause[hour][b].t)
		})
	}

	liveActive := showNow && live != nil && live.Active()
	liveHour := -1
	if liveActive {
		if !live.StartedAt.IsZero() {
			liveHour = live.StartedAt.Local().Hour()
		} else {
			liveHour = now.Hour()
		}
	}

	var rows []dayGridRow
	nowPlaced := false
	for hour := 0; hour < pomoTimelineHours; hour++ {
		items := mergeHourTimelineItems(byHour[hour], byHourPause[hour])
		sum := slotSummary{sessions: len(byHour[hour]), mins: slotFocusMins(byHour[hour])}

		rows = append(rows, dayGridRow{
			hour:        hour,
			hourLabel:   fmtHourLabel(hour),
			kind:        "slot",
			slotBusy:    len(items) > 0 || (showNow && hour == now.Hour()),
			slotSummary: sum,
		})

		for j, item := range items {
			if showNow && hour == now.Hour() && !nowPlaced && item.start.After(now) {
				rows = append(rows, nowGridRow(hour, now))
				nowPlaced = true
			}
			switch item.kind {
			case "pomo":
				rows = append(rows, dayGridRow{
					hour:          hour,
					kind:          "pomo",
					inHourGrid:    true,
					recIdx:        item.pomo.idx,
					rec:           item.pomo.rec,
					pomoSlotIndex: j,
					pomoSlotCount: len(items),
				})
			case "pause":
				rows = append(rows, dayGridRow{
					hour:          hour,
					kind:          "pause",
					inHourGrid:    true,
					pause:         item.pause.spell,
					pauseIdx:      item.pause.idx,
					pomoSlotIndex: j,
					pomoSlotCount: len(items),
				})
			}
		}

		if showNow && hour == now.Hour() && !nowPlaced {
			rows = append(rows, nowGridRow(hour, now))
			nowPlaced = true
		}

		if liveActive && liveHour == hour {
			rows = append(rows, dayGridRow{hour: hour, kind: "live", inHourGrid: true})
		}
	}
	return rows
}

func mergeHourTimelineItems(pomos []indexedFocus, pauses []indexedPause) []hourTimelineItem {
	out := make([]hourTimelineItem, 0, len(pomos)+len(pauses))
	for _, p := range pomos {
		out = append(out, hourTimelineItem{start: p.t, kind: "pomo", pomo: p})
	}
	for _, p := range pauses {
		out = append(out, hourTimelineItem{start: p.t, kind: "pause", pause: p})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].start.Equal(out[j].start) {
			return out[i].kind < out[j].kind
		}
		return out[i].start.Before(out[j].start)
	})
	return out
}

func nowGridRow(hour int, now time.Time) dayGridRow {
	return dayGridRow{
		hour:      hour,
		hourLabel: now.Format("15:04"),
		kind:      "now",
	}
}

func fmtHourLabel(hour int) string {
	return fmt.Sprintf("%02d:00", hour)
}

func gridRowForHour(grid []dayGridRow, hour int) int {
	for i, row := range grid {
		if row.hour == hour && row.kind == "now" {
			return i
		}
	}
	for i, row := range grid {
		if row.hour == hour && row.kind == "slot" {
			return i
		}
	}
	return 0
}

func gridRowForNow(grid []dayGridRow) int {
	for i, row := range grid {
		if row.kind == "now" {
			return i
		}
	}
	return -1
}

func renderHourSlotGapRow(width int, busy bool, selected bool) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	if selected {
		timeCol = pomoTimelineSelectionPrefix(true) + hintStyle.Render(strings.Repeat(" ", dayTimeColW-2))
	}
	gap := dayTimelineGap()
	railW := dayRailWidth(width)
	railChar := dayEmptyRail
	if busy {
		railChar = daySlotRail
	}
	rail := sectionRuleStyle.Render(strings.Repeat(railChar, railW))
	line := timeCol + gap + rail
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}

func renderHourSlotRow(label string, busy bool, width int, selected bool) string {
	timeCol := hintStyle.Render(fmtTimeCol(label))
	if busy {
		timeCol = pomoTimeStyle.Render(fmtTimeCol(label))
	}
	if selected {
		timeCol = pomoTimelineSelectionPrefix(true) + timeCol
	}
	gap := dayTimelineGap()
	railW := dayRailWidth(width)
	rail := sectionRuleStyle.Render(strings.Repeat(daySlotRail, railW))
	line := timeCol + gap + rail
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}

func renderHourNowRow(timeLabel string, width int, selected bool) string {
	timeCol := pomoNowStyle.Render(fmtTimeCol(timeLabel))
	if selected {
		timeCol = pomoTimelineSelectionPrefix(true) + timeCol
	}
	gap := dayTimelineGap()
	railW := dayRailWidth(width)
	marker := pomoNowStyle.Render("──● now ")
	pad := railW - lipgloss.Width(marker)
	if pad < 0 {
		pad = 0
	}
	line := timeCol + gap + marker + pomoNowStyle.Render(strings.Repeat("─", pad))
	if selected {
		line = highlightPomoTimelineRow(line, true)
	}
	return truncateInner(line, width)
}

func renderHourPomoRow(index, total int, selected bool, title string, taskColor lipgloss.Color, startClock, endClock string, mins int, width int, layout pomoTimelineLayout) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	gap := dayTimelineGap()
	contentW := dayTimelineContentW(width)
	body, suffix := hourPomoRowParts(title, taskColor, startClock, endClock, mins, selected)
	line := alignPomoRowColumns(pomoTimelineSelectionPrefix(selected)+body, suffix, contentW, layout)
	return truncateInner(highlightPomoTimelineRow(timeCol+gap+line, selected), width)
}

func renderHourPauseRow(spell focus.PauseSpell, selected bool, width int, layout pomoTimelineLayout, now time.Time) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	gap := dayTimelineGap()
	contentW := dayTimelineContentW(width)
	body, suffix := hourPauseRowParts(spell, selected, now)
	line := alignPomoRowColumns(pomoTimelineSelectionPrefix(selected)+body, suffix, contentW, layout)
	return truncateInner(highlightPomoTimelineRow(timeCol+gap+line, selected), width)
}

func renderHourLiveRow(sess *focus.Session, taskColor lipgloss.Color, width int, layout pomoTimelineLayout, selected bool) string {
	if sess == nil || !sess.Active() {
		return ""
	}
	body, suffix := hourLiveRowParts(sess, taskColor)
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	gap := dayTimelineGap()
	contentW := dayTimelineContentW(width)
	line := alignPomoRowColumns(pomoTimelineSelectionPrefix(selected)+body, suffix, contentW, layout)
	return truncateInner(highlightPomoTimelineRow(timeCol+gap+line, selected), width)
}
