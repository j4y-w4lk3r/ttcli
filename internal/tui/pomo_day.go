package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) pomoViewIsToday() bool {
	if m.pomoViewDate.IsZero() {
		return true
	}
	return dateKey(m.pomoViewDate) == dateKey(time.Now())
}

// pomoTimelineAnchor is the reference clock for the hour grid (live "now" only on today).
func (m model) pomoTimelineAnchor() time.Time {
	if m.pomoViewIsToday() {
		if !m.pomoNowTick.IsZero() {
			return m.pomoNowTick
		}
		return time.Now()
	}
	d := m.pomoViewDate
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.Local)
}

func pomoDaySectionTitle(viewDate time.Time) string {
	if dateKey(viewDate) == dateKey(time.Now()) {
		return "Today"
	}
	return viewDate.Format("Mon 2 Jan")
}

func pomoTimelineDayLabel(viewDate time.Time) string {
	label := viewDate.Format("Mon 2 Jan")
	if dateKey(viewDate) == dateKey(time.Now()) {
		return label + "  " + calTodayStyle.Render("today")
	}
	return label + "  " + hintStyle.Render(viewDate.Format("02/01/2006"))
}

func pomoDoneSectionTitle(viewDate time.Time) string {
	if dateKey(viewDate) == dateKey(time.Now()) {
		return "Done today"
	}
	return "Done · " + viewDate.Format("Mon 2 Jan")
}

func (m model) pomoNavDay(delta int) (model, tea.Cmd) {
	next := dateOnly(m.pomoViewDate.AddDate(0, 0, delta))
	today := dateOnly(time.Now())
	if next.After(today) {
		m.toast = "timeline stops at today"
		return m, nil
	}
	m.pomoViewDate = next
	m.loading = true
	m.pomoCursor = 0
	m.pomoGridCursor = 0
	m.pomoScrollToNow = m.pomoViewIsToday()
	m.pomoFollowNow = m.pomoViewIsToday()
	m.toast = fmt.Sprintf("timeline · %s", next.Format("Mon 2 Jan"))
	return m, loadPomoCmd(m.client, next)
}

func (m model) pomoJumpToday() (model, tea.Cmd) {
	today := dateOnly(time.Now())
	if dateKey(m.pomoViewDate) == dateKey(today) {
		m.centerPomoTimelineOnNow()
		m.toast = "timeline · centered on now"
		return m, nil
	}
	m.pomoViewDate = today
	m.loading = true
	m.pomoCursor = 0
	m.pomoGridCursor = 0
	m.pomoScrollToNow = true
	m.pomoFollowNow = true
	m.toast = "timeline · today"
	return m, loadPomoCmd(m.client, today)
}

func (m *model) centerPomoTimelineOnNow() {
	if !m.pomoViewIsToday() {
		return
	}
	grid := m.pomoDayGrid()
	if idx := gridRowForNow(grid); idx >= 0 {
		m.pomoGridCursor = idx
		if snap := nearestSelectablePomoGridRow(grid, idx, -1); snap >= 0 {
			m.pomoGridCursor = snap
		}
		m.syncPomoCursorFromGrid(grid)
		m.pomoFollowNow = true
	}
}

func (m model) pomoTimelineGridCursor(grid []dayGridRow) int {
	if m.pomoFollowNow && m.pomoViewIsToday() {
		if idx := gridRowForNow(grid); idx >= 0 {
			return idx
		}
	}
	cursor := m.pomoGridCursor
	if cursor >= len(grid) {
		cursor = gridRowForRecord(grid, m.pomoCursor)
	}
	return cursor
}
