package tui

import (
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

// pomoGapLinesPerSlot returns how many spacer lines to insert after each hour rail.
func pomoGapLinesPerSlot(maxRows int) int {
	if maxRows < 12 {
		return 1
	}
	gaps := maxRows / 10
	if gaps < 2 {
		gaps = 2
	}
	if gaps > 8 {
		gaps = 8
	}
	return gaps
}

func expandPomoGrid(grid []dayGridRow, gapAfterSlot int) []dayGridRow {
	if gapAfterSlot < 1 || len(grid) == 0 {
		return grid
	}
	out := make([]dayGridRow, 0, len(grid)+24*gapAfterSlot)
	for _, row := range grid {
		out = append(out, row)
		if row.kind != "slot" {
			continue
		}
		for g := 0; g < gapAfterSlot; g++ {
			out = append(out, dayGridRow{
				hour:     row.hour,
				kind:     "gap",
				slotBusy: row.slotBusy,
			})
		}
	}
	return out
}

func (m model) pomoTimelineMaxRows(innerLines int) int {
	maxRows := innerLines - 2 // header + footer hint
	if maxRows < 6 {
		maxRows = 6
	}
	return maxRows
}

func (m model) pomoTimelineDensity() PomoTimelineDensity {
	if m.uiSettings.PomoTimelineDensity == PomoDensityCompact {
		return PomoDensityCompact
	}
	return PomoDensityStretch
}

func (m model) pomoTimelineGapLines(innerLines int) int {
	if m.pomoTimelineDensity() == PomoDensityCompact {
		return 0
	}
	return pomoGapLinesPerSlot(m.pomoTimelineMaxRows(innerLines))
}

func (m model) pomoDayGridFor(innerLines int) []dayGridRow {
	anchor := m.pomoTimelineAnchor()
	var live *focus.Session
	if m.pomoViewIsToday() {
		live, _ = focus.Load()
	}
	pauses, _ := focus.PauseSpellsForTimeline(m.pomoViewDate, live, anchor)
	grid := buildDayGrid(m.focusStatsRecords(), pauses, anchor, live, m.pomoViewIsToday())
	return expandPomoGrid(grid, m.pomoTimelineGapLines(innerLines))
}

func fillPomoTimelineStretch(lines []string, maxLines, width int, busy bool) []string {
	if maxLines < 1 {
		return lines
	}
	out := append([]string(nil), lines...)
	for len(out) < maxLines {
		out = append(out, renderHourSlotGapRow(width, busy, false))
	}
	if len(out) > maxLines {
		out = out[:maxLines]
	}
	return out
}
