package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/taskcheckin"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type calMode int

const (
	calModeDay calMode = iota
	calModeWeek
	calModeMonth
	calModeYear
)

type calIndex struct {
	byDate map[string][]calEntry
}

type calEntryState int

const (
	calEntryOpen calEntryState = iota
	calEntryNativeDone
	calEntryLocalDone
	calEntryPending
)

type calEntry struct {
	Task      ticktick.Task
	Date      time.Time
	State     calEntryState
	Checkin   *taskcheckin.Record
	Generated bool
}

func (e calEntry) Done() bool {
	return e.State == calEntryNativeDone || e.State == calEntryLocalDone
}

func (e calEntry) LocalOnly() bool {
	return e.State == calEntryLocalDone
}

func (e calEntry) NativeCheckin() bool {
	return e.State == calEntryNativeDone || (e.Task.Repeating() && !e.Generated)
}

func (e calEntry) SeriesID() string {
	if e.Checkin != nil && e.Checkin.SeriesID != "" {
		return e.Checkin.SeriesID
	}
	if id := e.Task.SeriesID(); id != "" {
		return id
	}
	return "anonymous:" + e.Task.Title + "\x00" + e.Task.DueDate
}

func dateOnly(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func dateKey(t time.Time) string {
	return dateOnly(t).Format("2006-01-02")
}

func calendarRange(day time.Time, mode calMode) (time.Time, time.Time) {
	return calendarRangeFor(day, mode, true)
}

func calendarRangeFor(day time.Time, mode calMode, monday bool) (time.Time, time.Time) {
	day = dateOnly(day)
	switch mode {
	case calModeDay:
		return day, day
	case calModeWeek:
		start := weekStartFor(day, monday)
		return start, start.AddDate(0, 0, 6)
	case calModeYear:
		return time.Date(day.Year(), 1, 1, 0, 0, 0, 0, time.Local),
			time.Date(day.Year(), 12, 31, 0, 0, 0, 0, time.Local)
	default:
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.Local)
		return start, start.AddDate(0, 1, -1)
	}
}

func buildCalIndex(tasks []ticktick.Task) calIndex {
	return buildCalendarIndex(tasks, nil, nil)
}

func buildCalendarIndex(openTasks, completed []ticktick.Task, checkins []taskcheckin.Record) calIndex {
	return buildCalendarIndexForRange(openTasks, completed, checkins, time.Time{}, time.Time{})
}

func buildCalendarIndexForRange(
	openTasks, completed []ticktick.Task,
	checkins []taskcheckin.Record,
	rangeStart, rangeEnd time.Time,
) calIndex {
	entries := make(map[string]calEntry)
	taskDateKeys := make(map[string]string)
	taskByID := make(map[string]ticktick.Task, len(openTasks)+len(completed))
	add := func(entry calEntry) {
		seriesID := entry.SeriesID()
		if seriesID == "" {
			seriesID = entry.Task.ID
		}
		key := seriesID + "\x00" + dateKey(entry.Date)
		entries[key] = entry
		if entry.Task.ID != "" {
			taskDateKeys[entry.Task.ID+"\x00"+dateKey(entry.Date)] = key
		}
	}

	for _, task := range openTasks {
		taskByID[task.ID] = task
		if task.SeriesID() != "" {
			taskByID[task.SeriesID()] = task
		}
		occurrences := []ticktick.Task{task}
		if !rangeStart.IsZero() && task.Repeating() {
			occurrences, _ = ticktick.ExpandTaskOccurrences(task, rangeStart, rangeEnd)
			hasCurrent := false
			for _, occurrence := range occurrences {
				if occurrence.DueDate == task.DueDate {
					hasCurrent = true
					break
				}
			}
			if !hasCurrent {
				occurrences = append(occurrences, task)
			}
		}
		for _, occurrence := range occurrences {
			day, ok := parseDueDay(occurrence.DueDate)
			if !ok {
				continue
			}
			add(calEntry{
				Task: occurrence, Date: day, State: calEntryOpen,
				Generated: occurrence.DueDate != task.DueDate,
			})
		}
	}
	for _, task := range completed {
		taskByID[task.ID] = task
		if task.SeriesID() != "" {
			taskByID[task.SeriesID()] = task
		}
		if !task.Repeating() && task.RepeatTaskID == "" {
			continue
		}
		completedAt, err := ticktick.ParseAPITime(task.CompletedT)
		if err != nil {
			continue
		}
		day := dateOnly(completedAt)
		entryKey := task.SeriesID() + "\x00" + dateKey(day)
		displayTask := task
		generated := false
		if existing, ok := entries[entryKey]; ok {
			displayTask = existing.Task
			generated = existing.Generated
		} else {
			displayTask.DueDate = dateKey(day)
			displayTask.IsAllDay = true
		}
		add(calEntry{Task: displayTask, Date: day, State: calEntryNativeDone, Generated: generated})
	}
	for i := range checkins {
		record := checkins[i]
		day, err := time.ParseInLocation("2006-01-02", record.Date, time.Local)
		if err != nil {
			continue
		}
		task, ok := taskByID[record.TaskID]
		if !ok {
			task, ok = taskByID[record.SeriesID]
		}
		if !ok {
			task = ticktick.Task{
				ID: record.TaskID, ProjectID: record.ProjectID, Title: record.Title,
			}
		}
		task.DueDate = record.Date
		task.IsAllDay = true
		state := calEntryLocalDone
		if record.Native {
			state = calEntryNativeDone
		}
		copyRecord := record
		entry := calEntry{Task: task, Date: day, State: state, Checkin: &copyRecord}
		key := entry.SeriesID() + "\x00" + dateKey(day)
		if existingKey, found := taskDateKeys[record.TaskID+"\x00"+dateKey(day)]; found {
			if existing, ok := entries[existingKey]; ok && existing.State == calEntryNativeDone {
				entry.State = calEntryNativeDone
			}
			if existingKey != key {
				delete(entries, existingKey)
			}
		}
		if existing, found := entries[key]; found {
			entry.Task = existing.Task
			entry.Generated = existing.Generated
			if existing.State == calEntryNativeDone {
				entry.State = calEntryNativeDone
			}
		}
		entries[key] = entry
	}

	idx := calIndex{byDate: make(map[string][]calEntry)}
	for _, entry := range entries {
		idx.byDate[dateKey(entry.Date)] = append(idx.byDate[dateKey(entry.Date)], entry)
	}
	return idx
}

func (idx calIndex) on(d time.Time) []calEntry {
	return idx.byDate[dateKey(d)]
}

func (idx calIndex) countOn(d time.Time) int {
	return len(idx.on(d))
}

func (idx calIndex) countInMonth(y int, m time.Month) int {
	n := 0
	for k, entries := range idx.byDate {
		d, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		if d.Year() == y && d.Month() == m {
			n += len(entries)
		}
	}
	return n
}

func weekStart(d time.Time) time.Time {
	return weekStartFor(d, true)
}

func weekStartFor(d time.Time, monday bool) time.Time {
	d = dateOnly(d)
	if !monday {
		return d.AddDate(0, 0, -int(d.Weekday()))
	}
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}

func (m model) calIdx() calIndex {
	start, end := calendarRangeFor(m.calDate, m.calMode, m.uiSettings.weekStartsMonday())
	idx := buildCalendarIndexForRange(m.calTasks, m.calCompleted, m.taskCheckins, start, end)
	if m.pendingCheckin == "" {
		return idx
	}
	for key, entries := range idx.byDate {
		for i := range entries {
			if taskCheckinKey(entries[i].SeriesID(), entries[i].Date) == m.pendingCheckin {
				entries[i].State = calEntryPending
			}
		}
		idx.byDate[key] = entries
	}
	return idx
}

func (m model) calSelected() time.Time {
	return dateOnly(m.calDate)
}

const calViewTabLines = 2 // mode tabs + spacer before the selected subview

func (m model) renderCalendarView(l layout) string {
	idx := m.calIdx()
	var b strings.Builder
	b.WriteString(m.renderCalModeTabs(l.fullW))
	b.WriteString("\n\n")

	switch m.calMode {
	case calModeDay:
		b.WriteString(m.renderCalDay(idx, l))
	case calModeWeek:
		b.WriteString(m.renderCalWeek(idx, l))
	case calModeYear:
		b.WriteString(m.renderCalYear(idx, l))
	default:
		b.WriteString(m.renderCalMonth(idx, l))
	}

	return renderPane(padBlockToSize(b.String(), l.innerLines, l.fullW), l, l.termW, false)
}

func (m model) renderCalModeTabs(maxW int) string {
	tabs := []struct {
		mode  calMode
		label string
	}{
		{calModeDay, "Day"},
		{calModeWeek, "Week"},
		{calModeMonth, "Month"},
		{calModeYear, "Year"},
	}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := iconCalendar + " " + t.label
		if m.calMode == t.mode {
			parts[i] = navActiveStyle.Render(label)
		} else {
			parts[i] = navIdleStyle.Render(label)
		}
	}
	return clipLine(lipgloss.JoinHorizontal(lipgloss.Top, strings.Join(parts, " ")), maxW)
}

func (m model) renderCalDay(idx calIndex, l layout) string {
	d := m.calSelected()
	now := time.Now()
	isToday := dateKey(d) == dateKey(now)
	overdue := []calEntry{}
	if isToday {
		overdue = idx.overdueBefore(d)
	}
	dayTasks := idx.onSorted(d)
	rows, _ := m.calDayRows(idx)
	plan := m.buildCalDayCoaching(idx, d, now)

	var b strings.Builder
	title := d.Format("Monday, 2 January 2006")
	if isToday {
		title += "  " + calTodayStyle.Render("today")
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(calDaySummary(dayTasks, overdue, d, now)))
	overdueAction := "show overdue"
	if m.uiSettings.CalendarDayShowOverdue {
		overdueAction = "hide overdue"
	}
	if h := m.keyHint("x check-in · o " + overdueAction + " · t today · [/] navigate · j/k select"); h != "" {
		b.WriteString("\n")
		b.WriteString(h)
	}
	b.WriteString("\n\n")
	header := b.String()
	bodyRows := max(l.innerLines-calViewTabLines-lineCount(strings.TrimSuffix(header, "\n")), 4)

	var selected *calDayRow
	if row, ok := selectedCalDayRow(rows, m.calGridCursor); ok {
		selected = &row
	}
	hiddenOverdue := 0
	if isToday && !m.uiSettings.CalendarDayShowOverdue {
		hiddenOverdue = len(overdue)
	}
	if l.fullW >= calDaySplitMinWidth && bodyRows >= 20 {
		if bodyRows < 24 {
			selected = nil
		}
		coachW := min(max(l.fullW*32/100, 38), 64)
		agendaW := max(l.fullW-coachW-2, 40)
		agenda := renderCalDayAgenda(m, rows, d, now, agendaW, bodyRows)
		coach := renderCalDayCoachBox(
			plan, selected, hiddenOverdue, coachW, bodyRows, m.uiSettings.planningConfig(),
		)
		b.WriteString(padBlockToSize(lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(agendaW).MaxHeight(bodyRows).Render(agenda),
			"  ",
			coach,
		), bodyRows, l.fullW))
		return b.String()
	}

	coach := renderCalDayCoachCompact(plan, hiddenOverdue, l.fullW)
	b.WriteString(coach)
	b.WriteString("\n\n")
	agendaRows := max(bodyRows-lipgloss.Height(coach)-1, 4)
	b.WriteString(renderCalDayAgenda(m, rows, d, now, l.fullW, agendaRows))
	return b.String()
}

func (m model) renderCalWeek(idx calIndex, l layout) string {
	start := weekStartFor(m.calSelected(), m.uiSettings.weekStartsMonday())
	end := start.AddDate(0, 0, 6)
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s – %s",
		start.Format("2 Jan"), end.Format("2 Jan 2006"))))
	b.WriteString("\n")
	if h := m.keyHint("j/k task · PgUp/PgDn scroll · z density · t now · x check-in · h/l day · Enter day"); h != "" {
		b.WriteString(h)
	}
	b.WriteString("\n")

	if l.fullW >= 56 {
		b.WriteString(m.renderCalWeekTimeline(idx, l))
	} else {
		b.WriteString("\n")
		b.WriteString(m.renderCalWeekList(idx, l))
	}
	return b.String()
}

func (m model) renderCalWeekList(idx calIndex, l layout) string {
	var b strings.Builder
	start := weekStartFor(m.calSelected(), m.uiSettings.weekStartsMonday())
	now := time.Now()
	config := m.uiSettings.planningConfig()
	weekRows := m.calWeekRows(idx)
	selectedRow := calWeekRow{}
	if len(weekRows) > 0 {
		selectedRow = weekRows[normalizeCalWeekGridCursor(weekRows, m.calGridCursor)]
	}
	for i := 0; i < 7; i++ {
		d := start.AddDate(0, 0, i)
		entries := idx.onSorted(d)
		line := fmt.Sprintf("%s  %2d %s", d.Format("Mon"), d.Day(), d.Format("Jan"))
		load := weekCapacityCell(buildCalDayPlan(idx, d, now, config), 18)
		line = alignPomoRowColumns(line, load, l.fullW, pomoTimelineLayout{})
		if dateKey(d) == dateKey(m.calSelected()) {
			line = listSelStyle.Render(iconTaskSel + " " + line)
		} else if dateKey(d) == dateKey(time.Now()) {
			line = calTodayStyle.Render("  "+line) + " " + calTodayStyle.Render("today")
		} else {
			line = listIdleStyle.Render("  " + line)
		}
		b.WriteString(truncateInner(line, l.fullW))
		b.WriteString("\n")
		displayEntries := append([]calEntry(nil), entries[:min(2, len(entries))]...)
		if calWeekRowSelectable(selectedRow) && selectedRow.dayIndex == i {
			found := false
			for _, entry := range displayEntries {
				found = entry.SeriesID() == selectedRow.entry.SeriesID()
				if found {
					break
				}
			}
			if !found && len(displayEntries) > 0 {
				displayEntries[len(displayEntries)-1] = selectedRow.entry
			}
		}
		for _, entry := range displayEntries {
			marker := iconTaskOpen
			style := listIdleStyle
			if entry.Done() {
				marker = iconCheck
				style = taskDoneStyle
			}
			if calWeekRowSelectable(selectedRow) &&
				entry.SeriesID() == selectedRow.entry.SeriesID() &&
				dateKey(entry.Date) == dateKey(selectedRow.entry.Date) {
				marker = iconTaskSel
				style = taskSelStyle
				if entry.Done() {
					style = taskDoneStyle.Bold(true)
				}
			}
			when := "all day"
			if due, ok := parseDueTime(entry.Task.DueDate); ok && !entry.Task.IsAllDay {
				when = due.Format("15:04")
			}
			estimate := planning.EstimateTask(entry.Task, config)
			taskLine := "    " + style.Render(marker+" "+entry.Task.Title)
			suffix := hintStyle.Render(when + " · " + calendarEstimateLabel(estimate))
			b.WriteString(truncateInner(alignPomoRowColumns(taskLine, suffix, l.fullW, pomoTimelineLayout{}), l.fullW))
			b.WriteString("\n")
		}
		if len(entries) > len(displayEntries) {
			b.WriteString(truncateInner(hintStyle.Render(fmt.Sprintf("    +%d more", len(entries)-len(displayEntries))), l.fullW))
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m model) renderCalMonth(idx calIndex, l layout) string {
	d := m.calSelected()
	var b strings.Builder
	b.WriteString(headerStyle.Render(d.Format("January 2006")))
	b.WriteString("\n")
	if h := m.keyHint("j/k day · h/l day · Enter day · [/] month · d/w/m/y"); h != "" {
		b.WriteString(h)
	}
	b.WriteString("\n")
	b.WriteString(m.renderCalMonthGrid(idx, l, m.calMonthHeaderLineCount()))
	return b.String()
}

func (m model) calNavPrev() model {
	switch m.calMode {
	case calModeDay:
		m.calDate = m.calSelected().AddDate(0, 0, -1)
	case calModeWeek:
		m.calDate = m.calSelected().AddDate(0, 0, -7)
	case calModeYear:
		m.calDate = m.calSelected().AddDate(-1, 0, 0)
	default:
		m.calDate = m.calSelected().AddDate(0, -1, 0)
	}
	m.calTaskCursor = 0
	m.calGridCursor = 0
	m.calDayCenterNow = false
	m.calWeekViewport = 0
	return m
}

func (m model) calNavNext() model {
	switch m.calMode {
	case calModeDay:
		m.calDate = m.calSelected().AddDate(0, 0, 1)
	case calModeWeek:
		m.calDate = m.calSelected().AddDate(0, 0, 7)
	case calModeYear:
		m.calDate = m.calSelected().AddDate(1, 0, 0)
	default:
		m.calDate = m.calSelected().AddDate(0, 1, 0)
	}
	m.calTaskCursor = 0
	m.calGridCursor = 0
	m.calDayCenterNow = false
	m.calWeekViewport = 0
	return m
}

func (m model) calMoveVert(delta int) model {
	idx := m.calIdx()
	switch m.calMode {
	case calModeDay:
		rows, _ := m.calDayRows(idx)
		if len(rows) > 0 {
			m.calGridCursor = moveCalDayGridCursor(rows, m.calGridCursor, delta)
			m.syncCalTaskFromGrid(rows)
		}
	case calModeWeek:
		rows := m.calWeekRows(idx)
		if len(rows) > 0 {
			m.calGridCursor = moveCalWeekGridCursor(rows, m.calGridCursor, delta)
		}
	case calModeMonth:
		d := m.calSelected()
		daysInMonth := time.Date(d.Year(), d.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
		newDay := clamp(d.Day()+delta, 1, daysInMonth)
		m.calDate = time.Date(d.Year(), d.Month(), newDay, 0, 0, 0, 0, time.Local)
		m.calTaskCursor = 0
		m.calGridCursor = 0
	case calModeYear:
		d := m.calSelected()
		layout := m.layout()
		cols := calYearGridColumnsForSize(layout.fullW, layout.innerLines)
		newMonth := clamp(int(d.Month())+delta*cols, 1, 12)
		m.calDate = time.Date(d.Year(), time.Month(newMonth), 1, 0, 0, 0, 0, time.Local)
	}
	return m
}

func (m model) calMoveHoriz(delta int) model {
	switch m.calMode {
	case calModeWeek:
		m.calDate = m.calSelected().AddDate(0, 0, delta)
	case calModeMonth:
		m.calDate = m.calSelected().AddDate(0, 0, delta)
	case calModeYear:
		d := m.calSelected()
		newMonth := clamp(int(d.Month())+delta, 1, 12)
		m.calDate = time.Date(d.Year(), time.Month(newMonth), 1, 0, 0, 0, 0, time.Local)
	default:
		m.calDate = m.calSelected().AddDate(0, 0, delta)
	}
	m.calTaskCursor = 0
	m.calGridCursor = 0
	m.calDayCenterNow = false
	m.calWeekViewport = 0
	return m
}

func (m model) calDrillDown() model {
	prev := m.calMode
	switch m.calMode {
	case calModeYear:
		m.calMode = calModeMonth
	case calModeWeek, calModeMonth:
		m.calMode = calModeDay
	}
	m.calTaskCursor = 0
	m.calGridCursor = 0
	m.calDayCenterNow = false
	if prev != calModeDay && m.calMode == calModeDay {
		m.toast = "day view · " + m.calSelected().Format("Mon 2 Jan")
	}
	return m
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var (
	calSelStyle       = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorMauve)
	calDueStyle       = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	calIdleStyle      = lipgloss.NewStyle().Foreground(colorText)
	calTodayStyle     = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	calTodayCellStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	calAllDayStyle    = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	calWeekHeadStyle  = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	calWeekCellStyle  = lipgloss.NewStyle().Foreground(colorText)
	calMonthSelStyle  = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorBlue).Padding(0, 1)
	calMonthIdleStyle = lipgloss.NewStyle().Foreground(colorSubtext).Padding(0, 1)
)
