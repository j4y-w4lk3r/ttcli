package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type calWeekRow struct {
	hour     int
	kind     string // slot, task, now
	dayIndex int
	task     ticktick.Task
}

func taskDueHour(t ticktick.Task) (int, bool) {
	if t.IsAllDay || !hasDueTime(t.DueDate) {
		return 0, false
	}
	due, ok := parseDueTime(t.DueDate)
	if !ok {
		return 0, false
	}
	return due.Hour(), true
}

func buildCalWeekTimeline(idx calIndex, weekStart, now time.Time) []calWeekRow {
	type keyed struct {
		day  int
		task ticktick.Task
		hour int
	}
	var byHour = map[int][]keyed{}
	for d := 0; d < 7; d++ {
		day := weekStart.AddDate(0, 0, d)
		for _, t := range idx.onSorted(day) {
			hour, ok := taskDueHour(t)
			if !ok {
				hour = 8
			}
			byHour[hour] = append(byHour[hour], keyed{day: d, task: t, hour: hour})
		}
	}
	for hour := range byHour {
		sort.Slice(byHour[hour], func(i, j int) bool {
			a, b := byHour[hour][i], byHour[hour][j]
			if a.day != b.day {
				return a.day < b.day
			}
			return a.task.Title < b.task.Title
		})
	}

	isThisWeek := dateKey(now) >= dateKey(weekStart) && dateKey(now) <= dateKey(weekStart.AddDate(0, 0, 6))
	nowHour := now.Hour()
	var rows []calWeekRow
	nowPlaced := false
	for hour := 0; hour < 24; hour++ {
		rows = append(rows, calWeekRow{hour: hour, kind: "slot"})
		for _, item := range byHour[hour] {
			rows = append(rows, calWeekRow{
				hour:     hour,
				kind:     "task",
				dayIndex: item.day,
				task:     item.task,
			})
		}
		if isThisWeek && !nowPlaced && nowHour == hour {
			rows = append(rows, calWeekRow{hour: hour, kind: "now", dayIndex: int(now.Weekday())})
			nowPlaced = true
		}
	}
	if isThisWeek && !nowPlaced {
		rows = append(rows, calWeekRow{hour: nowHour, kind: "now", dayIndex: int(now.Weekday())})
	}
	return rows
}

func calWeekColumnWidths(fullW int) ([7]int, int) {
	gap := dayTimelineGap()
	contentW := fullW - dayTimeColW - len(gap)
	if contentW < 42 {
		contentW = 42
	}
	return calMonthColumnWidths(contentW), contentW
}

func renderCalWeekDayHead(weekStart time.Time, sel time.Time, colW [7]int, contentW int) string {
	var parts []string
	for d := 0; d < 7; d++ {
		day := weekStart.AddDate(0, 0, d)
		label := day.Format("Mon")
		if colW[d] < 10 {
			label = label[:1]
		}
		num := fmt.Sprintf("%d", day.Day())
		line := label + " " + num
		st := calWeekHeadStyle
		if dateKey(day) == dateKey(sel) {
			st = calWeekHeadStyle.Copy().Bold(true).Foreground(colorMauve)
		} else if dateKey(day) == dateKey(time.Now()) {
			st = calWeekHeadStyle.Copy().Foreground(colorGreen)
		}
		parts = append(parts, padToWidth(st.Width(colW[d]).Align(lipgloss.Center).Render(line), colW[d]))
	}
	gap := dayTimelineGap()
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	return padToWidth(timeCol+gap+lipgloss.JoinHorizontal(lipgloss.Top, parts...), dayTimeColW+len(gap)+contentW)
}

func renderCalWeekSlotRow(hour int, weekStart, now time.Time, idx calIndex, colW [7]int, contentW, fullW int) string {
	label := fmt.Sprintf("%02d:00", hour)
	timeCol := pomoTimeStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, label))
	gap := dayTimelineGap()
	var cols []string
	for d := 0; d < 7; d++ {
		day := weekStart.AddDate(0, 0, d)
		busy := false
		for _, t := range idx.onSorted(day) {
			if h, ok := taskDueHour(t); ok && h == hour {
				busy = true
				break
			}
		}
		railW := colW[d] - 1
		if railW < 3 {
			railW = 3
		}
		rail := sectionRuleStyle.Render(strings.Repeat(daySlotRail, railW))
		if !busy {
			rail = hintStyle.Render(strings.Repeat(dayEmptyRail, railW))
		}
		cols = append(cols, padToWidth(rail, colW[d]))
	}
	return padToWidth(timeCol+gap+lipgloss.JoinHorizontal(lipgloss.Top, cols...), fullW)
}

func renderCalWeekTaskRow(row calWeekRow, weekStart time.Time, sel time.Time, colW [7]int, contentW, fullW int) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	gap := dayTimelineGap()
	var cols []string
	for d := 0; d < 7; d++ {
		cell := strings.Repeat(" ", colW[d])
		if d == row.dayIndex {
			t := row.task
			title := t.Title
			if title == "" {
				title = "…"
			}
			maxW := colW[d] - 2
			if maxW < 3 {
				maxW = 3
			}
			if lipgloss.Width(title) > maxW {
				title = truncateRunes(title, max(1, maxW-1)) + "…"
			}
			col := taskPriorityColor(t.Priority.Int())
			bar := lipgloss.NewStyle().Foreground(col).Render("▌")
			st := listIdleStyle
			if dateKey(weekStart.AddDate(0, 0, d)) == dateKey(sel) {
				st = taskSelStyle
			}
			cell = truncateInner(bar+st.Render(title), colW[d])
		}
		cols = append(cols, padToWidth(cell, colW[d]))
	}
	return padToWidth(timeCol+gap+lipgloss.JoinHorizontal(lipgloss.Top, cols...), fullW)
}

func renderCalWeekNowRow(row calWeekRow, now time.Time, colW [7]int, contentW, fullW int) string {
	timeCol := pomoNowStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, now.Format("15:04")))
	gap := dayTimelineGap()
	var cols []string
	for d := 0; d < 7; d++ {
		cell := strings.Repeat(" ", colW[d])
		if d == row.dayIndex {
			cell = pomoNowStyle.Render("● now")
		}
		cols = append(cols, padToWidth(cell, colW[d]))
	}
	return padToWidth(timeCol+gap+lipgloss.JoinHorizontal(lipgloss.Top, cols...), fullW)
}

func (m model) renderCalWeekTimeline(idx calIndex, l layout) string {
	weekStart := weekStart(m.calSelected())
	now := time.Now()
	colW, contentW := calWeekColumnWidths(l.fullW)
	fullRowW := dayTimeColW + len(dayTimelineGap()) + contentW
	rows := buildCalWeekTimeline(idx, weekStart, now)

	var b strings.Builder
	b.WriteString(renderCalWeekDayHead(weekStart, m.calSelected(), colW, contentW))
	b.WriteString("\n")

	overhead := 2 + 2 + 1 // calendar tabs + week title block + day head
	maxRows := l.innerLines - overhead
	if maxRows < 8 {
		maxRows = 8
	}
	cursor := m.calGridCursor
	if cursor >= len(rows) {
		cursor = 0
	}
	win := computeScrollWindow(cursor, len(rows), maxRows)
	if hint := m.scrollHint(win); hint != "" {
		b.WriteString(padToWidth(hint, l.fullW))
		b.WriteString("\n")
		maxRows--
	}
	for i := win.Start; i < win.End; i++ {
		row := rows[i]
		switch row.kind {
		case "slot":
			b.WriteString(renderCalWeekSlotRow(row.hour, weekStart, now, idx, colW, contentW, fullRowW))
		case "task":
			b.WriteString(renderCalWeekTaskRow(row, weekStart, m.calSelected(), colW, contentW, fullRowW))
		case "now":
			b.WriteString(renderCalWeekNowRow(row, now, colW, contentW, fullRowW))
		}
		b.WriteString("\n")
	}
	return b.String()
}
