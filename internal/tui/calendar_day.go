package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type calDayRow struct {
	hourLabel     string
	kind          string // overdue-hdr, overdue, allday-hdr, allday, slot, task, now
	slotBusy      bool
	slotSummary   slotSummary
	slotIndex     int
	slotCount     int
	taskIdx       int
	task          ticktick.Task
}

func (idx calIndex) onSorted(d time.Time) []ticktick.Task {
	tasks := append([]ticktick.Task(nil), idx.on(d)...)
	sortTasksByDue(tasks)
	return tasks
}

func (idx calIndex) overdueBefore(d time.Time) []ticktick.Task {
	key := dateKey(d)
	var out []ticktick.Task
	for k, ts := range idx.byDate {
		if k < key {
			out = append(out, ts...)
		}
	}
	sortTasksByDue(out)
	return out
}

func (m model) calDayRows(idx calIndex) (rows []calDayRow, tasks []ticktick.Task) {
	d := m.calSelected()
	now := time.Now()
	isToday := dateKey(d) == dateKey(now)

	var overdue []ticktick.Task
	if isToday {
		overdue = idx.overdueBefore(d)
	}
	dayTasks := idx.onSorted(d)
	tasks = append(append([]ticktick.Task(nil), overdue...), dayTasks...)

	taskIdx := 0
	if len(overdue) > 0 {
		rows = append(rows, calDayRow{kind: "overdue-hdr"})
		for _, t := range overdue {
			rows = append(rows, calDayRow{kind: "overdue", taskIdx: taskIdx, task: t})
			taskIdx++
		}
	}

	rows = append(rows, buildCalDayTimeline(dayTasks, d, now, taskIdx)...)
	return rows, tasks
}

func calDayHourRange(bySlot map[int]int, day, now time.Time) (first, last int) {
	first, last = 24, -1
	for slot, n := range bySlot {
		if n <= 0 {
			continue
		}
		if slot < first {
			first = slot
		}
		if slot > last {
			last = slot
		}
	}
	isToday := dateKey(day) == dateKey(now)
	nowSlot := (now.Hour() / dayHourStep) * dayHourStep
	if isToday {
		if last < 0 {
			first = nowSlot
			last = nowSlot
		} else {
			if nowSlot < first {
				first = nowSlot
			}
			if nowSlot > last {
				last = nowSlot
			}
		}
	}
	if last < 0 {
		first = 8
		last = 18
	} else {
		first -= dayHourStep
		last += dayHourStep
	}
	if first < 0 {
		first = 0
	}
	if last > 24-dayHourStep {
		last = 24 - dayHourStep
	}
	return first, last
}

func buildCalDayTimeline(dayTasks []ticktick.Task, day, now time.Time, taskIdxBase int) []calDayRow {
	type indexed struct {
		idx  int
		task ticktick.Task
		t    time.Time
		kind string
	}
	const dateOnlySlot = (9 / dayHourStep) * dayHourStep // 08:00 rail when step=2
	bySlot := map[int][]indexed{}
	for i, t := range dayTasks {
		kind := "task"
		slot := dateOnlySlot
		if t.IsAllDay || !hasDueTime(t.DueDate) {
			kind = "allday"
		} else {
			due, ok := parseDueTime(t.DueDate)
			if !ok {
				continue
			}
			slot = (due.Hour() / dayHourStep) * dayHourStep
			if slot > 24-dayHourStep {
				slot = 24 - dayHourStep
			}
		}
		bySlot[slot] = append(bySlot[slot], indexed{
			idx:  taskIdxBase + i,
			task: t,
			kind: kind,
		})
	}
	for slot := range bySlot {
		sort.Slice(bySlot[slot], func(a, b int) bool {
			ai, bi := bySlot[slot][a], bySlot[slot][b]
			if ai.kind != bi.kind {
				return ai.kind == "allday"
			}
			if ai.kind == "task" && bi.kind == "task" {
				da, oka := parseDueTime(ai.task.DueDate)
				db, okb := parseDueTime(bi.task.DueDate)
				if oka && okb && !da.Equal(db) {
					return da.Before(db)
				}
			}
			return ai.task.Title < bi.task.Title
		})
	}

	slotCounts := map[int]int{}
	for slot, items := range bySlot {
		slotCounts[slot] = len(items)
	}

	isToday := dateKey(day) == dateKey(now)
	nowSlot := (now.Hour() / dayHourStep) * dayHourStep
	firstHour, lastHour := calDayHourRange(slotCounts, day, now)
	var rows []calDayRow
	nowPlaced := false

	for hour := firstHour; hour <= lastHour; hour += dayHourStep {
		label := fmt.Sprintf("%02d:00", hour)
		items := bySlot[hour]
		sum := slotSummary{sessions: len(items)}
		busy := len(items) > 0 || (isToday && nowSlot == hour)
		rows = append(rows, calDayRow{hourLabel: label, kind: "slot", slotBusy: busy, slotSummary: sum})
		for j, item := range items {
			rows = append(rows, calDayRow{
				kind:      item.kind,
				taskIdx:   item.idx,
				task:      item.task,
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
		return truncateInner(sectionHeader("Overdue", width), width)
	case "allday-hdr":
		return truncateInner(sectionHeader("All day", width), width)
	case "slot":
		return renderDaySlotRow(row.hourLabel, row.slotBusy, width, row.slotSummary)
	case "now":
		return renderDayNowRow(now.Format("15:04"), width)
	case "overdue", "allday", "task":
		return renderCalTaskRow(row, day, now, selected, width)
	default:
		return ""
	}
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
	t := row.task
	overdue := calTaskShouldBeDone(t, day, now) || row.kind == "overdue"

	titleSt := listIdleStyle
	if overdue {
		titleSt = dueOverStyle
	}
	if selected {
		titleSt = taskSelStyle
		if overdue {
			titleSt = dueOverStyle.Bold(true)
		}
	}

	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	dot := lipgloss.NewStyle().Foreground(taskPriorityColor(t.Priority.Int())).Render("●")

	switch row.kind {
	case "allday":
		conn := dayPomoConnector(row.slotIndex, row.slotCount, selected)
		marker := iconTaskOpen
		if selected {
			marker = iconTaskSel
		}
		body := conn + " " + dot + " " + calAllDayStyle.Render("all day") + "  " + titleSt.Render(marker+" "+title)
		return renderDayTimelineEntryBody("", body, "", width, pomoTimelineLayout{})
	case "overdue":
		marker := iconOverdueDot
		if selected {
			marker = iconTaskSel
		}
		prefix := dayItemPrefix(selected)
		body := prefix + titleSt.Render(marker+" "+title)
		return renderDayTimelineEntryBody("", body, dueInline(t.DueDate), width, pomoTimelineLayout{})
	default:
		clock := dueTaskClock(t)
		conn := dayPomoConnector(row.slotIndex, row.slotCount, selected)
		marker := iconTaskOpen
		if selected {
			marker = iconTaskSel
		}
		body := conn + " " + dot + " " + titleSt.Render(marker + " " + title)
		if p := t.PriorityLabel(); p != "-" {
			body += " " + prioStyle(p).Render(p)
		}
		return renderDayTimelineEntryBody(clock, body, "", width, pomoTimelineLayout{})
	}
}

func (m *model) syncCalTaskFromGrid(rows []calDayRow) {
	if m.calGridCursor < 0 || m.calGridCursor >= len(rows) {
		return
	}
	row := rows[m.calGridCursor]
	switch row.kind {
	case "overdue", "allday", "task":
		m.calTaskCursor = row.taskIdx
	}
}

func calDaySummary(tasks, overdue []ticktick.Task, day time.Time, now time.Time) string {
	planned := len(tasks)
	doneByNow := 0
	for _, t := range tasks {
		if calTaskShouldBeDone(t, day, now) {
			doneByNow++
		}
	}
	parts := []string{fmt.Sprintf("%d planned", planned)}
	if len(overdue) > 0 {
		parts = append(parts, fmt.Sprintf("%d overdue", len(overdue)))
	}
	if dateKey(day) == dateKey(now) && doneByNow > 0 {
		parts = append(parts, fmt.Sprintf("%d past due today", doneByNow))
	}
	return strings.Join(parts, " · ")
}
