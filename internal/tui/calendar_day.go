package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const calendarHourStep = 1

type calDayRow struct {
	hourLabel   string
	kind        string // section headers, overdue, allday, slot, task, done, now
	slotBusy    bool
	slotSummary slotSummary
	slotIndex   int
	slotCount   int
	taskIdx     int
	entry       calEntry
	estimate    planning.TaskEstimate
	start       time.Time
	end         time.Time
	logged      bool
}

func (idx calIndex) onSorted(d time.Time) []calEntry {
	entries := append([]calEntry(nil), idx.on(d)...)
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i].Task, entries[j].Task
		if a.DueDate != b.DueDate {
			return a.DueDate < b.DueDate
		}
		if entries[i].Done() != entries[j].Done() {
			return !entries[i].Done()
		}
		return a.Title < b.Title
	})
	return entries
}

func (idx calIndex) overdueBefore(d time.Time) []calEntry {
	key := dateKey(d)
	var out []calEntry
	for k, entries := range idx.byDate {
		if k < key {
			for _, entry := range entries {
				if !entry.Done() {
					out = append(out, entry)
				}
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Task.DueDate < out[j].Task.DueDate
	})
	return out
}

func (m model) calDayRows(idx calIndex) (rows []calDayRow, entries []calEntry) {
	d := m.calSelected()
	now := time.Now()
	isToday := dateKey(d) == dateKey(now)
	config := m.uiSettings.planningConfig()

	var overdue []calEntry
	if isToday && m.uiSettings.CalendarDayShowOverdue {
		overdue = idx.overdueBefore(d)
	}
	dayEntries := idx.onSorted(d)
	var allDay, scheduled, done []calEntry
	for _, entry := range dayEntries {
		switch {
		case entry.Done():
			done = append(done, entry)
		case entry.Task.IsAllDay || !hasDueTime(entry.Task.DueDate):
			allDay = append(allDay, entry)
		default:
			scheduled = append(scheduled, entry)
		}
	}

	appendSection := func(kind string, section []calEntry) {
		if len(section) == 0 {
			return
		}
		rows = append(rows, calDayRow{kind: kind + "-hdr", slotCount: len(section)})
		for _, entry := range section {
			taskIdx := len(entries)
			entries = append(entries, entry)
			rows = append(rows, calDayRow{
				kind:     kind,
				taskIdx:  taskIdx,
				entry:    entry,
				estimate: planning.EstimateTask(entry.Task, config),
			})
		}
	}

	appendSection("allday", allDay)
	if len(overdue) > 0 {
		appendSection("overdue", overdue)
	}
	logged := visibleLoggedSpans(idx, m.calFocusByDate[dateKey(d)], scheduled, d, config)
	rows = append(rows, calDayRow{kind: "schedule-hdr", slotCount: len(scheduled) + len(logged)})
	scheduledBase := len(entries)
	entries = append(entries, scheduled...)
	rows = append(rows, buildCalDayTimelineWithLogged(scheduled, logged, d, now, scheduledBase, config)...)
	appendSection("done", done)
	return rows, entries
}

func calDayHourRange(bySlot map[int]int, day, now time.Time, config planning.Config) (first, last int) {
	config = config.Normalized()
	first = config.WorkStartMinutes / 60
	last = (config.WorkEndMinutes + 59) / 60
	for slot, n := range bySlot {
		if n <= 0 {
			continue
		}
		if slot < first {
			first = slot
		}
		if slot+1 > last {
			last = slot + 1
		}
	}
	if dateKey(day) == dateKey(now) {
		if now.Hour() < first {
			first = now.Hour()
		}
		if now.Hour() > last {
			last = now.Hour()
		}
	}
	if first < 0 {
		first = 0
	}
	if last > 23 {
		last = 23
	}
	if last < first {
		last = first
	}
	return first, last
}

func calTaskSchedule(entry calEntry, config planning.Config) (planning.TaskEstimate, time.Time, time.Time) {
	estimate := planning.EstimateTask(entry.Task, config)
	due, ok := parseDueTime(entry.Task.DueDate)
	if !ok {
		return estimate, time.Time{}, time.Time{}
	}
	if estimate.Explicit {
		if start, ok := parseDueTime(entry.Task.StartDate); ok && due.After(start) {
			return estimate, start, due
		}
	}
	return estimate, due, time.Time{}
}

func buildCalDayTimeline(dayEntries []calEntry, day, now time.Time, taskIdxBase int, config planning.Config) []calDayRow {
	return buildCalDayTimelineWithLogged(dayEntries, nil, day, now, taskIdxBase, config)
}

func buildCalDayTimelineWithLogged(
	dayEntries []calEntry,
	logged []calLoggedSpan,
	day, now time.Time,
	taskIdxBase int,
	config planning.Config,
) []calDayRow {
	type indexed struct {
		idx      int
		entry    calEntry
		estimate planning.TaskEstimate
		start    time.Time
		end      time.Time
		logged   bool
	}
	bySlot := map[int][]indexed{}
	for i, entry := range dayEntries {
		estimate, start, end := calTaskSchedule(entry, config)
		if start.IsZero() {
			continue
		}
		slot := start.Hour()
		bySlot[slot] = append(bySlot[slot], indexed{
			idx:      taskIdxBase + i,
			entry:    entry,
			estimate: estimate,
			start:    start,
			end:      end,
		})
	}
	for _, span := range logged {
		if span.Start.IsZero() {
			continue
		}
		slot := span.Start.Hour()
		bySlot[slot] = append(bySlot[slot], indexed{
			idx:      -1,
			entry:    span.Entry,
			estimate: span.Estimate,
			start:    span.Start,
			end:      span.End,
			logged:   true,
		})
	}
	for slot := range bySlot {
		sort.Slice(bySlot[slot], func(a, b int) bool {
			ai, bi := bySlot[slot][a], bySlot[slot][b]
			if !ai.start.Equal(bi.start) {
				return ai.start.Before(bi.start)
			}
			return ai.entry.Task.Title < bi.entry.Task.Title
		})
	}

	slotCounts := map[int]int{}
	for slot, items := range bySlot {
		slotCounts[slot] = len(items)
	}

	isToday := dateKey(day) == dateKey(now)
	nowSlot := now.Hour()
	firstHour, lastHour := calDayHourRange(slotCounts, day, now, config)
	var rows []calDayRow
	nowPlaced := false

	for hour := firstHour; hour <= lastHour; hour += calendarHourStep {
		label := fmt.Sprintf("%02d:00", hour)
		items := bySlot[hour]
		sum := slotSummary{sessions: len(items), capacityMins: 60}
		for _, item := range items {
			sum.mins += item.estimate.Minutes
		}
		busy := len(items) > 0 || (isToday && nowSlot == hour)
		rows = append(rows, calDayRow{hourLabel: label, kind: "slot", slotBusy: busy, slotSummary: sum})
		for j, item := range items {
			if isToday && !nowPlaced && nowSlot == hour && now.Before(item.start) {
				rows = append(rows, calDayRow{kind: "now"})
				nowPlaced = true
			}
			rows = append(rows, calDayRow{
				hourLabel: item.start.Format("15:04"),
				kind:      "task",
				taskIdx:   item.idx,
				entry:     item.entry,
				estimate:  item.estimate,
				start:     item.start,
				end:       item.end,
				logged:    item.logged,
				slotIndex: j,
				slotCount: len(items),
			})
		}
		if isToday && !nowPlaced && nowSlot == hour && hour < 24 {
			rows = append(rows, calDayRow{kind: "now"})
			nowPlaced = true
		}
	}
	if isToday && !nowPlaced {
		rows = append(rows, calDayRow{hourLabel: now.Format("15:04"), kind: "now"})
	}
	return rows
}

func renderCalDayGridRow(row calDayRow, day, now time.Time, selected bool, width int) string {
	switch row.kind {
	case "overdue-hdr":
		return truncateInner(sectionHeader(fmt.Sprintf("Overdue (%d)", row.slotCount), width), width)
	case "allday-hdr":
		return truncateInner(sectionHeader(fmt.Sprintf("All day (%d)", row.slotCount), width), width)
	case "schedule-hdr":
		title := "Schedule"
		if row.slotCount > 0 {
			title = fmt.Sprintf("Schedule (%d)", row.slotCount)
		}
		return truncateInner(sectionHeader(title, width), width)
	case "done-hdr":
		return truncateInner(sectionHeader(fmt.Sprintf("Done (%d)", row.slotCount), width), width)
	case "slot":
		return renderDaySlotRow(row.hourLabel, row.slotBusy, width, row.slotSummary)
	case "now":
		return renderDayNowRow(now.Format("15:04"), width)
	case "overdue", "allday", "task", "done":
		return renderCalTaskRow(row, day, now, selected, width)
	default:
		return ""
	}
}

func calendarEstimateLabel(estimate planning.TaskEstimate) string {
	label := planning.FormatMinutes(estimate.Minutes)
	if !estimate.Explicit {
		label = "~" + label
	}
	return label
}

func taskPriorityColor(priority int) lipgloss.Color {
	switch priority {
	case 5:
		return colorRed
	case 3:
		return colorPeach
	case 1:
		return colorBlue
	default:
		return colorMuted
	}
}

func renderCalTaskRow(row calDayRow, day, now time.Time, selected bool, width int) string {
	entry := row.entry
	t := entry.Task
	overdue := !entry.Done() && (calTaskIsPastDue(t, day, now) || row.kind == "overdue")

	titleSt := listIdleStyle
	if entry.Done() {
		titleSt = taskDoneStyle
	} else if overdue {
		titleSt = dueOverStyle
	}
	if selected {
		titleSt = taskSelStyle
		if entry.Done() {
			titleSt = taskDoneStyle.Bold(true)
		} else if overdue {
			titleSt = dueOverStyle.Bold(true)
		}
	}

	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	dot := lipgloss.NewStyle().Foreground(taskPriorityColor(t.Priority.Int())).Render("●")
	marker := iconTaskOpen
	if row.logged {
		marker = iconPomodoro
	} else if entry.Done() {
		marker = iconCheck
	} else if entry.State == calEntryPending {
		marker = iconRefresh
	}
	if selected {
		marker = iconTaskSel
	}
	local := ""
	if entry.LocalOnly() {
		local = " " + hintStyle.Render("local")
	}

	switch row.kind {
	case "allday":
		prefix := dayItemPrefix(selected)
		body := prefix + dot + " " + titleSt.Render(marker+" "+title) + local
		return renderDayTimelineEntryBody("", body, hintStyle.Render(calendarEstimateLabel(row.estimate)), width, pomoTimelineLayout{})
	case "overdue":
		if !entry.Done() {
			marker = iconOverdueDot
		}
		if selected {
			marker = iconTaskSel
		}
		prefix := dayItemPrefix(selected)
		body := prefix + titleSt.Render(marker+" "+title) + local
		suffix := dueInline(t.DueDate)
		if estimate := calendarEstimateLabel(row.estimate); estimate != "" {
			suffix += " · " + estimate
		}
		return renderDayTimelineEntryBody("", body, hintStyle.Render(suffix), width, pomoTimelineLayout{})
	case "done":
		prefix := dayItemPrefix(selected)
		body := prefix + titleSt.Render(marker+" "+title) + local
		return renderDayTimelineEntryBody("", body, "", width, pomoTimelineLayout{})
	default:
		clock := row.hourLabel
		conn := dayPomoConnector(row.slotIndex, row.slotCount, selected)
		body := conn + " " + dot + " " + titleSt.Render(marker+" "+title) + local
		if p := t.PriorityLabel(); p != "-" {
			body += " " + prioStyle(p).Render(p)
		}
		suffix := calendarEstimateLabel(row.estimate)
		if row.logged {
			if !row.end.IsZero() {
				suffix = row.end.Format("15:04") + " · " + suffix + " logged"
			} else {
				suffix = suffix + " logged"
			}
		} else if !row.end.IsZero() {
			suffix = row.end.Format("15:04") + " · " + suffix
		} else {
			suffix = "due · " + suffix
		}
		return renderDayTimelineEntryBody(clock, body, hintStyle.Render(suffix), width, pomoTimelineLayout{})
	}
}

func renderCalDayTaskContinuation(row calDayRow, selected, last bool, width int) string {
	style := listIdleStyle
	if selected {
		style = taskSelStyle
	}
	border := "│"
	detail := calendarEstimateLabel(row.estimate) + " planned"
	if row.logged {
		detail = calendarEstimateLabel(row.estimate) + " logged"
	}
	if last {
		border = "╰─"
		if !row.end.IsZero() {
			detail = "ends " + row.end.Format("15:04")
		}
	}
	rail := lipgloss.NewStyle().
		Foreground(taskPriorityColor(row.entry.Task.Priority.Int())).
		Render(border)
	return renderDayTimelineEntryBody("", "  "+rail+" "+style.Render(detail), "", width, pomoTimelineLayout{})
}

func (m *model) syncCalTaskFromGrid(rows []calDayRow) {
	if m.calGridCursor < 0 || m.calGridCursor >= len(rows) {
		return
	}
	row := rows[m.calGridCursor]
	switch row.kind {
	case "overdue", "allday", "task", "done":
		m.calTaskCursor = row.taskIdx
	}
}

func calDayRowSelectable(row calDayRow) bool {
	switch row.kind {
	case "overdue", "allday", "task", "done":
		return true
	default:
		return false
	}
}

func normalizeCalDayGridCursor(rows []calDayRow, cursor int) int {
	if len(rows) == 0 {
		return 0
	}
	cursor = clamp(cursor, 0, len(rows)-1)
	if calDayRowSelectable(rows[cursor]) {
		return cursor
	}
	for i := cursor + 1; i < len(rows); i++ {
		if calDayRowSelectable(rows[i]) {
			return i
		}
	}
	for i := cursor - 1; i >= 0; i-- {
		if calDayRowSelectable(rows[i]) {
			return i
		}
	}
	return cursor
}

func moveCalDayGridCursor(rows []calDayRow, cursor, delta int) int {
	if len(rows) == 0 || delta == 0 {
		return normalizeCalDayGridCursor(rows, cursor)
	}
	cursor = normalizeCalDayGridCursor(rows, cursor)
	step := 1
	if delta < 0 {
		step = -1
	}
	steps := delta
	if steps < 0 {
		steps = -steps
	}
	for moved := 0; moved < steps; {
		next := cursor + step
		for next >= 0 && next < len(rows) && !calDayRowSelectable(rows[next]) {
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

func calDaySummary(entries, overdue []calEntry, day time.Time, now time.Time) string {
	planned := 0
	done := 0
	doneByNow := 0
	for _, entry := range entries {
		if entry.Done() {
			done++
			continue
		}
		planned++
		if calTaskIsPastDue(entry.Task, day, now) {
			doneByNow++
		}
	}
	parts := []string{fmt.Sprintf("%d planned", planned)}
	if done > 0 {
		parts = append(parts, fmt.Sprintf("%d done", done))
	}
	if len(overdue) > 0 {
		parts = append(parts, fmt.Sprintf("%d overdue", len(overdue)))
	}
	if dateKey(day) == dateKey(now) && doneByNow > 0 {
		parts = append(parts, fmt.Sprintf("%d past due today", doneByNow))
	}
	return strings.Join(parts, " · ")
}

func buildCalDayPlan(idx calIndex, day, now time.Time, config planning.Config) planning.DayPlan {
	var inputs []planning.TaskInput
	for _, entry := range idx.onSorted(day) {
		if !entry.Done() {
			inputs = append(inputs, planning.TaskInput{Task: entry.Task})
		}
	}
	if dateKey(day) == dateKey(now) {
		for _, entry := range idx.overdueBefore(day) {
			inputs = append(inputs, planning.TaskInput{Task: entry.Task, Overdue: true})
		}
	}
	return planning.BuildDayPlan(day, now, inputs, config)
}

func (m model) buildCalDayCoaching(idx calIndex, day, now time.Time) planning.DayPlan {
	entries := append([]calEntry(nil), idx.onSorted(day)...)
	if dateKey(day) == dateKey(now) {
		entries = append(idx.overdueBefore(day), entries...)
	}
	stats := m.calFocusByDate[dateKey(day)]
	focusByID := map[string]ticktick.TaskFocusSummary{}
	focusByTitle := map[string]ticktick.TaskFocusSummary{}
	totalMinutes, totalPomos := 0, 0
	if stats != nil {
		focusIndex := ticktick.AggregateTaskFocus(stats.Records)
		focusByID, focusByTitle = focusIndex.ByID, focusIndex.ByTitle
		for _, record := range stats.Records {
			totalMinutes += int(ticktick.RecordDuration(record).Minutes() + 0.5)
		}
		totalPomos = ticktick.CountFullPomos(stats.Records)
	}

	titleCounts := make(map[string]int)
	for _, entry := range entries {
		titleCounts[ticktick.NormalizeFocusTaskTitle(entry.Task.Title)]++
	}
	usedFocus := make(map[string]bool)
	assignedMinutes, assignedPomos := 0, 0
	inputs := make([]planning.TaskInput, 0, len(entries))
	for _, entry := range entries {
		taskInput := planning.TaskInput{
			Task: entry.Task, Done: entry.Done(),
			Overdue: entry.Date.Before(dateOnly(day)),
		}
		focusKey := ""
		summary, found := focusByID[entry.Task.ID]
		if found {
			focusKey = "id:" + entry.Task.ID
		} else if seriesID := entry.Task.SeriesID(); seriesID != entry.Task.ID {
			summary, found = focusByID[seriesID]
			if found {
				focusKey = "id:" + seriesID
			}
		}
		if !found {
			titleKey := ticktick.NormalizeFocusTaskTitle(entry.Task.Title)
			if titleKey != "" && titleCounts[titleKey] == 1 {
				summary, found = focusByTitle[titleKey]
				if found {
					focusKey = "title:" + titleKey
				}
			}
		}
		if found && !usedFocus[focusKey] {
			taskInput.LoggedMinutes = int((summary.TotalSeconds + 30) / 60)
			taskInput.LoggedPomos = summary.FullSessions
			assignedMinutes += taskInput.LoggedMinutes
			assignedPomos += taskInput.LoggedPomos
			usedFocus[focusKey] = true
		}
		inputs = append(inputs, taskInput)
	}

	activeMinutes, activeTaskID := 0, ""
	if dateKey(day) == dateKey(now) {
		if session, err := focus.Load(); err == nil && session.Active() &&
			dateKey(session.StartedAt) == dateKey(day) {
			elapsed := session.CurrentSegmentElapsed()
			if session.PlannedLogged {
				elapsed = session.OvertimeElapsed()
			}
			activeMinutes = int(elapsed.Minutes())
			activeTaskID = session.TaskID
		}
	}
	return planning.BuildCoachingPlan(planning.CoachingInput{
		Day: day, Now: now, Tasks: inputs,
		UnassignedLoggedMinutes: max(totalMinutes-assignedMinutes, 0),
		UnassignedLoggedPomos:   max(totalPomos-assignedPomos, 0),
		ActiveMinutes:           activeMinutes,
		ActiveTaskID:            activeTaskID,
		Config:                  m.uiSettings.planningConfig(),
	})
}

func renderCalDayPlanSummary(plan planning.DayPlan) string {
	needed := planning.FormatMinutes(plan.NeededMinutes)
	if plan.InferredTasks > 0 {
		needed = "~" + needed
	}
	if plan.Historical {
		return hintStyle.Render(needed + " planned · historical day")
	}

	label := strings.ToUpper(string(plan.Feasibility))
	labelStyle := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	switch plan.Feasibility {
	case planning.FeasibilityTight:
		labelStyle = lipgloss.NewStyle().Foreground(colorPeach).Bold(true)
	case planning.FeasibilityOverCapacity:
		labelStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	}
	parts := []string{
		needed + " needed",
		planning.FormatMinutes(plan.AvailableMinutes) + " available",
		labelStyle.Render(label),
		string(plan.Evidence) + " evidence",
	}
	if plan.InferredTasks > 0 {
		parts = append(parts, fmt.Sprintf("%d inferred", plan.InferredTasks))
	}
	return hintStyle.Render(strings.Join(parts[:2], " · ")) +
		" · " + strings.Join([]string{parts[2], hintStyle.Render(strings.Join(parts[3:], " · "))}, " · ")
}
