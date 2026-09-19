package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type calWeekRow struct {
	hour      int
	kind      string // allday-label, allday, capacity, slot, task, now
	dayIndex  int
	entry     calEntry
	estimate  planning.TaskEstimate
	start     time.Time
	end       time.Time
	taskIndex int
	logged    bool
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

func calWeekAdaptiveRange(byHour map[int][]calWeekRow, weekStart, now time.Time, config planning.Config) (int, int) {
	config = config.Normalized()
	first := config.WorkStartMinutes / 60
	last := (config.WorkEndMinutes + 59) / 60
	for hour, rows := range byHour {
		if len(rows) == 0 {
			continue
		}
		if hour < first {
			first = hour
		}
		if hour+1 > last {
			last = hour + 1
		}
	}
	if !now.Before(weekStart) && !now.After(weekStart.AddDate(0, 0, 7)) {
		if now.Hour() < first {
			first = now.Hour()
		}
		if now.Hour() > last {
			last = now.Hour()
		}
	}
	first = clamp(first, 0, 23)
	last = clamp(last, first, 23)
	return first, last
}

func buildCalWeekTimelineWithConfig(idx calIndex, weekStart, now time.Time, config planning.Config) []calWeekRow {
	return buildCalWeekTimelineWithFocus(idx, weekStart, now, config, nil)
}

func buildCalWeekTimelineWithFocus(
	idx calIndex,
	weekStart, now time.Time,
	config planning.Config,
	focusByDate map[string]*ticktick.FocusStats,
) []calWeekRow {
	var allDay []calWeekRow
	byHour := make(map[int][]calWeekRow)
	taskIndex := 0
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		day := weekStart.AddDate(0, 0, dayIndex)
		var scheduled []calEntry
		for _, entry := range idx.onSorted(day) {
			estimate := planning.EstimateTask(entry.Task, config)
			if entry.Task.IsAllDay || !hasDueTime(entry.Task.DueDate) {
				allDay = append(allDay, calWeekRow{
					kind: "allday", dayIndex: dayIndex, entry: entry,
					estimate: estimate, taskIndex: taskIndex,
				})
				taskIndex++
				continue
			}
			estimate, start, end := calTaskSchedule(entry, config)
			if start.IsZero() {
				continue
			}
			scheduled = append(scheduled, entry)
			byHour[start.Hour()] = append(byHour[start.Hour()], calWeekRow{
				hour: start.Hour(), kind: "task", dayIndex: dayIndex, entry: entry,
				estimate: estimate, start: start, end: end, taskIndex: taskIndex,
			})
			taskIndex++
		}
		var stats *ticktick.FocusStats
		if focusByDate != nil {
			stats = focusByDate[dateKey(day)]
		}
		for _, span := range visibleLoggedSpans(idx, stats, scheduled, day, config) {
			byHour[span.Start.Hour()] = append(byHour[span.Start.Hour()], calWeekRow{
				hour: span.Start.Hour(), kind: "task", dayIndex: dayIndex, entry: span.Entry,
				estimate: span.Estimate, start: span.Start, end: span.End,
				taskIndex: taskIndex, logged: true,
			})
			taskIndex++
		}
	}
	sort.SliceStable(allDay, func(i, j int) bool {
		if allDay[i].dayIndex != allDay[j].dayIndex {
			return allDay[i].dayIndex < allDay[j].dayIndex
		}
		return allDay[i].entry.Task.Title < allDay[j].entry.Task.Title
	})
	for hour := range byHour {
		sort.SliceStable(byHour[hour], func(i, j int) bool {
			a, b := byHour[hour][i], byHour[hour][j]
			if a.dayIndex != b.dayIndex {
				return a.dayIndex < b.dayIndex
			}
			if !a.start.Equal(b.start) {
				return a.start.Before(b.start)
			}
			return a.entry.Task.Title < b.entry.Task.Title
		})
	}

	rows := []calWeekRow{{kind: "allday-label"}}
	rows = append(rows, allDay...)
	rows = append(rows, calWeekRow{kind: "capacity"})

	first, last := calWeekAdaptiveRange(byHour, weekStart, now, config)
	thisWeek := !dateOnly(now).Before(weekStart) && !dateOnly(now).After(weekStart.AddDate(0, 0, 6))
	nowPlaced := false
	nowDay := int(dateOnly(now).Sub(weekStart).Hours() / 24)
	for hour := first; hour <= last; hour++ {
		rows = append(rows, calWeekRow{hour: hour, kind: "slot"})
		for _, row := range byHour[hour] {
			if thisWeek && !nowPlaced && nowDay == row.dayIndex && now.Hour() == hour && now.Before(row.start) {
				rows = append(rows, calWeekRow{hour: hour, kind: "now", dayIndex: nowDay})
				nowPlaced = true
			}
			rows = append(rows, row)
		}
		if thisWeek && !nowPlaced && now.Hour() == hour {
			rows = append(rows, calWeekRow{hour: hour, kind: "now", dayIndex: nowDay})
			nowPlaced = true
		}
	}
	return rows
}

func buildCalWeekTimeline(idx calIndex, weekStart, now time.Time) []calWeekRow {
	return buildCalWeekTimelineWithConfig(idx, weekStart, now, planning.DefaultConfig())
}

func calWeekRowSelectable(row calWeekRow) bool {
	return row.kind == "allday" || row.kind == "task"
}

func normalizeCalWeekGridCursor(rows []calWeekRow, cursor int) int {
	if len(rows) == 0 {
		return 0
	}
	cursor = clamp(cursor, 0, len(rows)-1)
	if calWeekRowSelectable(rows[cursor]) {
		return cursor
	}
	for i := cursor + 1; i < len(rows); i++ {
		if calWeekRowSelectable(rows[i]) {
			return i
		}
	}
	for i := cursor - 1; i >= 0; i-- {
		if calWeekRowSelectable(rows[i]) {
			return i
		}
	}
	return cursor
}

func moveCalWeekGridCursor(rows []calWeekRow, cursor, delta int) int {
	if len(rows) == 0 || delta == 0 {
		return normalizeCalWeekGridCursor(rows, cursor)
	}
	cursor = normalizeCalWeekGridCursor(rows, cursor)
	step, count := 1, delta
	if delta < 0 {
		step, count = -1, -delta
	}
	for moved := 0; moved < count; {
		next := cursor + step
		for next >= 0 && next < len(rows) && !calWeekRowSelectable(rows[next]) {
			next += step
		}
		if next < 0 || next >= len(rows) {
			break
		}
		cursor = next
		moved++
	}
	return cursor
}

func calWeekTimelineStart(rows []calWeekRow) int {
	for i, row := range rows {
		if row.kind == "slot" {
			return i
		}
	}
	return len(rows)
}

func calWeekColumnWidths(fullW int) ([7]int, int) {
	gap := dayTimelineGap()
	contentW := fullW - dayTimeColW - len(gap)
	if contentW < 42 {
		contentW = 42
	}
	widths := calMonthColumnWidths(max(contentW-6, 7))
	for dayIndex := 1; dayIndex < 7; dayIndex++ {
		widths[dayIndex]++
	}
	return widths, contentW
}

var calWeekDividerStyle = lipgloss.NewStyle().Foreground(colorSurface)

func calWeekCellContentWidth(width, dayIndex int) int {
	if dayIndex > 0 {
		return max(width-1, 1)
	}
	return max(width, 1)
}

func renderCalWeekColumns(cells [7]string, colW [7]int) string {
	var parts []string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		cell := padToWidth(truncateRenderedWidth(cells[dayIndex], cellW), cellW)
		if dayIndex > 0 {
			cell = calWeekDividerStyle.Render("┊") + cell
		}
		parts = append(parts, padToWidth(cell, colW[dayIndex]))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func renderCalWeekDayHead(weekStart time.Time, selected time.Time, colW [7]int, contentW int) string {
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		day := weekStart.AddDate(0, 0, dayIndex)
		label := day.Format("Mon")
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		if cellW < 10 {
			label = label[:1]
		}
		line := label + " " + fmt.Sprint(day.Day())
		style := calWeekHeadStyle
		if dateKey(day) == dateKey(selected) {
			style = style.Copy().Bold(true).Foreground(colorMauve)
		} else if dateKey(day) == dateKey(time.Now()) {
			style = style.Copy().Foreground(colorGreen)
		}
		cells[dayIndex] = style.Width(cellW).Align(lipgloss.Center).Render(line)
	}
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), dayTimeColW+len(dayTimelineGap())+contentW)
}

func weekTaskCell(row calWeekRow, width int, selected bool) string {
	return weekTaskCellSegment(row, width, selected, false, true)
}

func weekTaskCellSegment(row calWeekRow, width int, selected, continuation, last bool) string {
	title := displayText(row.entry.Task.Title)
	if title == "" {
		title = "…"
	}
	marker := iconTaskOpen
	style := listIdleStyle
	if row.logged {
		marker = iconPomodoro
	} else if row.entry.Done() {
		marker = iconCheck
		style = taskDoneStyle
	} else if row.entry.State == calEntryPending {
		marker = iconRefresh
	}
	if selected {
		style = taskSelStyle
		if row.entry.Done() {
			style = taskDoneStyle.Bold(true)
		}
		marker = iconTaskSel
	}
	duration := calendarEstimateLabel(row.estimate)
	if row.logged && !continuation {
		duration += " logged"
	}
	text := marker + " " + title
	border := "╭"
	if continuation {
		border = "│"
		text = duration
		if last && !row.end.IsZero() {
			border = "╰"
			text = "until " + row.end.Format("15:04")
		}
	} else if last {
		border = "▌"
	}
	if !continuation && width >= 10 {
		text += " " + duration
	}
	bar := lipgloss.NewStyle().Foreground(taskPriorityColor(row.entry.Task.Priority.Int())).Render(border)
	return truncateRenderedWidth(bar+style.Render(text), width)
}

func renderCalWeekAllDayRow(row calWeekRow, selected bool, colW [7]int, fullW int) string {
	label := ""
	if row.kind == "allday-label" {
		label = "all day"
	}
	prefix := hintStyle.Render(strings.Repeat(" ", dayTimeColW)) + dayTimelineGap()
	if label != "" {
		prefix = hintStyle.Render(padToWidth(label, dayTimeColW+len(dayTimelineGap())))
	}
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		if row.kind == "allday-label" {
			cells[dayIndex] = hintStyle.Render(strings.Repeat("─", cellW))
		} else if row.dayIndex == dayIndex {
			cells[dayIndex] = weekTaskCell(row, cellW, selected)
		}
	}
	return padToWidth(prefix+renderCalWeekColumns(cells, colW), fullW)
}

func weekCapacityCell(plan planning.DayPlan, width int) string {
	needed := planning.FormatMinutes(plan.NeededMinutes)
	if plan.InferredTasks > 0 {
		needed = "~" + needed
	}
	label := "PAST"
	color := colorMuted
	if !plan.Historical {
		switch plan.Feasibility {
		case planning.FeasibilityClear:
			label, color = "CLEAR", colorGreen
		case planning.FeasibilityComfortable:
			label, color = "OK", colorGreen
		case planning.FeasibilityTight:
			label, color = "TIGHT", colorPeach
		case planning.FeasibilityOverCapacity:
			label, color = "OVER", colorRed
		}
	}
	text := needed + " " + label
	if width >= 12 && !plan.Historical {
		text = needed + "/" + planning.FormatMinutes(plan.AvailableMinutes) + " " + label
	}
	return truncateRenderedWidth(lipgloss.NewStyle().Foreground(color).Render(text), width)
}

func renderCalWeekCapacityRow(idx calIndex, weekStart, now time.Time, config planning.Config, colW [7]int, fullW int) string {
	timeCol := hintStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, "load"))
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		day := weekStart.AddDate(0, 0, dayIndex)
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		cells[dayIndex] = weekCapacityCell(buildCalDayPlan(idx, day, now, config), cellW)
	}
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

func renderCalWeekSlotRow(hour int, weekStart time.Time, idx calIndex, config planning.Config, colW [7]int, fullW int) string {
	timeCol := pomoTimeStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, fmt.Sprintf("%02d:00", hour)))
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		day := weekStart.AddDate(0, 0, dayIndex)
		busy := false
		for _, entry := range idx.onSorted(day) {
			_, start, _ := calTaskSchedule(entry, config)
			if !start.IsZero() && start.Hour() == hour {
				busy = true
				break
			}
		}
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		rail := dayEmptySlotStyle.Render(strings.Repeat(dayEmptyRail, cellW))
		if busy {
			rail = sectionRuleStyle.Render(strings.Repeat(daySlotRail, cellW))
		}
		cells[dayIndex] = rail
	}
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

func renderCalWeekTaskRow(row calWeekRow, selected bool, colW [7]int, fullW int) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		if dayIndex == row.dayIndex {
			cells[dayIndex] = weekTaskCell(row, calWeekCellContentWidth(colW[dayIndex], dayIndex), selected)
		}
	}
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

func renderCalWeekNowRow(row calWeekRow, now time.Time, colW [7]int, fullW int) string {
	timeCol := pomoNowStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, now.Format("15:04")))
	var cells [7]string
	for dayIndex := 0; dayIndex < 7; dayIndex++ {
		cellW := calWeekCellContentWidth(colW[dayIndex], dayIndex)
		cells[dayIndex] = pomoNowStyle.Render(strings.Repeat("━", cellW))
		if dayIndex == row.dayIndex {
			label := "● NOW "
			cells[dayIndex] = pomoNowStyle.Render(label + strings.Repeat("━", max(cellW-lipgloss.Width(label), 0)))
		}
	}
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

func renderCalWeekSpacerRow(colW [7]int, fullW int) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	var cells [7]string
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

type calWeekIndexedRow struct {
	row          calWeekRow
	sourceIndex  int
	continuation bool
	last         bool
}

type calWeekVisualCell struct {
	row          calWeekRow
	sourceIndex  int
	present      bool
	continuation bool
	last         bool
}

type calWeekVisualRow struct {
	kind        string
	hour        int
	cells       [7]calWeekVisualCell
	now         calWeekRow
	sourceIndex int
}

func packCalWeekTaskLanes(rows []calWeekIndexedRow) []calWeekVisualRow {
	var lanes []calWeekVisualRow
	var used [7]int
	for _, indexed := range rows {
		day := clamp(indexed.row.dayIndex, 0, 6)
		laneIndex := used[day]
		used[day]++
		for len(lanes) <= laneIndex {
			lanes = append(lanes, calWeekVisualRow{kind: "tasks"})
		}
		lanes[laneIndex].cells[day] = calWeekVisualCell{
			row: indexed.row, sourceIndex: indexed.sourceIndex, present: true,
			continuation: indexed.continuation, last: indexed.last,
		}
	}
	return lanes
}

func buildCalWeekVisualBody(rows []calWeekRow, timelineStart int) []calWeekVisualRow {
	var visual []calWeekVisualRow
	for i := timelineStart; i < len(rows); {
		row := rows[i]
		if row.kind != "slot" {
			if row.kind == "now" {
				visual = append(visual, calWeekVisualRow{
					kind: "now", now: row, sourceIndex: i,
				})
			}
			i++
			continue
		}
		visual = append(visual, calWeekVisualRow{kind: "slot", hour: row.hour, sourceIndex: i})
		i++
		var tasks []calWeekIndexedRow
		var nowRows []calWeekVisualRow
		for i < len(rows) && rows[i].kind != "slot" {
			switch rows[i].kind {
			case "task":
				tasks = append(tasks, calWeekIndexedRow{row: rows[i], sourceIndex: i, last: true})
			case "now":
				nowRows = append(nowRows, calWeekVisualRow{
					kind: "now", now: rows[i], sourceIndex: i,
				})
			}
			i++
		}
		visual = append(visual, packCalWeekTaskLanes(tasks)...)
		visual = append(visual, nowRows...)
	}
	return visual
}

func calWeekRowsPerHour(bodyBudget, hourCount int, density PomoTimelineDensity) int {
	if density == PomoDensityCompact {
		return 1
	}
	if hourCount < 1 {
		hourCount = 1
	}
	return clamp(bodyBudget/hourCount, 2, 5)
}

func calWeekTaskEnd(row calWeekRow) time.Time {
	if !row.end.IsZero() && row.end.After(row.start) {
		return row.end
	}
	minutes := max(row.estimate.Minutes, 1)
	return row.start.Add(time.Duration(minutes) * time.Minute)
}

func buildCalWeekVisualBodyScaled(rows []calWeekRow, timelineStart, rowsPerHour int, now time.Time) []calWeekVisualRow {
	if rowsPerHour <= 1 {
		return buildCalWeekVisualBody(rows, timelineStart)
	}
	var tasks []calWeekIndexedRow
	var slots []calWeekIndexedRow
	var nowRow *calWeekIndexedRow
	for i := timelineStart; i < len(rows); i++ {
		indexed := calWeekIndexedRow{row: rows[i], sourceIndex: i}
		switch rows[i].kind {
		case "slot":
			slots = append(slots, indexed)
		case "task":
			tasks = append(tasks, indexed)
		case "now":
			copyRow := indexed
			nowRow = &copyRow
		}
	}

	segments := max(rowsPerHour-1, 1)
	var visual []calWeekVisualRow
	nowPlaced := false
	for _, slot := range slots {
		hour := slot.row.hour
		visual = append(visual, calWeekVisualRow{
			kind: "slot", hour: hour, sourceIndex: slot.sourceIndex,
		})
		for segment := 0; segment < segments; segment++ {
			segmentStart := hour*60 + segment*60/segments
			segmentEnd := hour*60 + (segment+1)*60/segments
			var active []calWeekIndexedRow
			for _, indexed := range tasks {
				startMinute := indexed.row.start.Hour()*60 + indexed.row.start.Minute()
				endAt := calWeekTaskEnd(indexed.row)
				endMinute := endAt.Hour()*60 + endAt.Minute()
				if dateKey(endAt) != dateKey(indexed.row.start) {
					endMinute = 24 * 60
				}
				if startMinute >= segmentEnd || endMinute <= segmentStart {
					continue
				}
				indexed.continuation = startMinute < segmentStart
				indexed.last = endMinute <= segmentEnd
				active = append(active, indexed)
			}
			lanes := packCalWeekTaskLanes(active)
			if len(lanes) == 0 {
				visual = append(visual, calWeekVisualRow{kind: "spacer", hour: hour})
			} else {
				visual = append(visual, lanes...)
			}
			if nowRow != nil && !nowPlaced && nowRow.row.hour == hour {
				nowMinute := now.Minute()
				if nowMinute <= (segment+1)*60/segments {
					visual = append(visual, calWeekVisualRow{
						kind: "now", now: nowRow.row, sourceIndex: nowRow.sourceIndex,
					})
					nowPlaced = true
				}
			}
		}
	}
	if nowRow != nil && !nowPlaced {
		visual = append(visual, calWeekVisualRow{
			kind: "now", now: nowRow.row, sourceIndex: nowRow.sourceIndex,
		})
	}
	return visual
}

func calWeekVisualCursor(rows []calWeekVisualRow, sourceCursor int) int {
	for i, row := range rows {
		if row.sourceIndex == sourceCursor {
			return i
		}
		for _, cell := range row.cells {
			if cell.present && cell.sourceIndex == sourceCursor {
				return i
			}
		}
	}
	return 0
}

func calWeekNowVisualCursor(rows []calWeekVisualRow) int {
	for i, row := range rows {
		if row.kind == "now" {
			return i
		}
	}
	return 0
}

func calWeekViewportWindow(start, cursor, total, maxRows int) scrollWindow {
	if total <= 0 {
		return scrollWindow{}
	}
	maxRows = max(maxRows, 1)
	start = clamp(start, 0, max(total-maxRows, 0))
	end := min(start+maxRows, total)
	cursor = clamp(cursor, 0, total-1)
	return scrollWindow{
		Start: start, End: end, Above: start, Below: total - end,
		Total: total, Cursor: cursor,
	}
}

func renderCalWeekTaskLane(row calWeekVisualRow, sourceCursor int, colW [7]int, fullW int) string {
	timeCol := hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	var cells [7]string
	for dayIndex, cell := range row.cells {
		if !cell.present {
			continue
		}
		cells[dayIndex] = weekTaskCellSegment(
			cell.row,
			calWeekCellContentWidth(colW[dayIndex], dayIndex),
			cell.sourceIndex == sourceCursor,
			cell.continuation,
			cell.last,
		)
	}
	return padToWidth(timeCol+dayTimelineGap()+renderCalWeekColumns(cells, colW), fullW)
}

func (m model) calWeekRows(idx calIndex) []calWeekRow {
	return buildCalWeekTimelineWithFocus(
		idx,
		weekStartFor(m.calSelected(), m.uiSettings.weekStartsMonday()),
		time.Now(),
		m.uiSettings.planningConfig(),
		m.calFocusByDate,
	)
}

func (m model) renderCalWeekTimeline(idx calIndex, layout layout) string {
	start := weekStartFor(m.calSelected(), m.uiSettings.weekStartsMonday())
	now := time.Now()
	config := m.uiSettings.planningConfig()
	colW, contentW := calWeekColumnWidths(layout.fullW)
	fullRowW := dayTimeColW + len(dayTimelineGap()) + contentW
	rows := m.calWeekRows(idx)
	cursor := normalizeCalWeekGridCursor(rows, m.calGridCursor)
	timelineStart := calWeekTimelineStart(rows)

	var b strings.Builder
	b.WriteString(renderCalWeekDayHead(start, m.calSelected(), colW, contentW))
	b.WriteString("\n")
	b.WriteString(renderCalWeekAllDayRow(rows[0], false, colW, fullRowW))
	b.WriteString("\n")
	var indexedAllDay []calWeekIndexedRow
	for i := 1; i < timelineStart; i++ {
		if rows[i].kind == "allday" {
			indexedAllDay = append(indexedAllDay, calWeekIndexedRow{row: rows[i], sourceIndex: i, last: true})
		}
	}
	allDayLanes := packCalWeekTaskLanes(indexedAllDay)
	for _, lane := range allDayLanes {
		b.WriteString(renderCalWeekTaskLane(lane, cursor, colW, fullRowW))
		b.WriteString("\n")
	}
	b.WriteString(renderCalWeekCapacityRow(idx, start, now, config, colW, fullRowW))
	b.WriteString("\n")

	stickyLines := 3 + len(allDayLanes)
	maxRows := layout.innerLines - stickyLines - 7
	if maxRows < 5 {
		maxRows = 5
	}
	hourCount := 0
	for i := timelineStart; i < len(rows); i++ {
		if rows[i].kind == "slot" {
			hourCount++
		}
	}
	rowsPerHour := calWeekRowsPerHour(maxRows, hourCount, m.uiSettings.calendarWeekDensity())
	body := buildCalWeekVisualBodyScaled(rows, timelineStart, rowsPerHour, now)
	bodyCursor := calWeekVisualCursor(body, cursor)
	window := scrollWindow{}
	switch m.calWeekViewport {
	case -2:
		window = computeScrollWindow(bodyCursor, len(body), maxRows)
	case -1:
		nowCursor := calWeekNowVisualCursor(body)
		window = computeScrollWindow(nowCursor, len(body), maxRows)
	default:
		window = calWeekViewportWindow(m.calWeekViewport, bodyCursor, len(body), maxRows)
	}
	b.WriteString(padToWidth(m.scrollHint(window), layout.fullW))
	b.WriteString("\n")
	for i := window.Start; i < window.End; i++ {
		row := body[i]
		switch row.kind {
		case "slot":
			b.WriteString(renderCalWeekSlotRow(row.hour, start, idx, config, colW, fullRowW))
		case "tasks":
			b.WriteString(renderCalWeekTaskLane(row, cursor, colW, fullRowW))
		case "now":
			b.WriteString(renderCalWeekNowRow(row.now, now, colW, fullRowW))
		case "spacer":
			b.WriteString(renderCalWeekSpacerRow(colW, fullRowW))
		}
		b.WriteString("\n")
	}
	return b.String()
}
