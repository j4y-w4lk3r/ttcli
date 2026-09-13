package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
	byDate map[string][]ticktick.Task
}

func dateOnly(t time.Time) time.Time {
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

func dateKey(t time.Time) string {
	return dateOnly(t).Format("2006-01-02")
}

func buildCalIndex(tasks []ticktick.Task) calIndex {
	idx := calIndex{byDate: make(map[string][]ticktick.Task)}
	for _, t := range tasks {
		d, ok := parseDueDay(t.DueDate)
		if !ok {
			continue
		}
		k := dateKey(d)
		idx.byDate[k] = append(idx.byDate[k], t)
	}
	return idx
}

func (idx calIndex) on(d time.Time) []ticktick.Task {
	return idx.byDate[dateKey(d)]
}

func (idx calIndex) countOn(d time.Time) int {
	return len(idx.on(d))
}

func (idx calIndex) countInMonth(y int, m time.Month) int {
	n := 0
	for k, ts := range idx.byDate {
		d, err := time.Parse("2006-01-02", k)
		if err != nil {
			continue
		}
		if d.Year() == y && d.Month() == m {
			n += len(ts)
		}
	}
	return n
}

func weekStart(d time.Time) time.Time {
	d = dateOnly(d)
	return d.AddDate(0, 0, -int(d.Weekday()))
}

func (m model) calIdx() calIndex {
	return buildCalIndex(m.calTasks)
}

func (m model) calSelected() time.Time {
	return dateOnly(m.calDate)
}

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

	return renderPane(b.String(), l, l.termW, false)
}

func (m model) renderCalModeTabs(maxW int) string {
	tabs := []struct {
		mode calMode
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
	overdue := []ticktick.Task{}
	if isToday {
		overdue = idx.overdueBefore(d)
	}
	dayTasks := idx.onSorted(d)
	rows, tasks := m.calDayRows(idx)

	var b strings.Builder
	title := d.Format("Monday, 2 January 2006")
	if isToday {
		title += "  " + calTodayStyle.Render("today")
	}
	b.WriteString(headerStyle.Render(title))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render(calDaySummary(dayTasks, overdue, d, now)))
	if h := m.keyHint("d day (today) · t jump today · [/] navigate · j/k scroll"); h != "" {
		b.WriteString("\n")
		b.WriteString(h)
	}
	b.WriteString("\n")
	b.WriteString(truncateInner(sectionHeader("Schedule", l.fullW), l.fullW))
	b.WriteString("\n")
	b.WriteString(truncateInner(hintStyle.Render("  time      task"), l.fullW))
	b.WriteString("\n\n")

	if len(tasks) == 0 {
		b.WriteString(hintStyle.Render("  (no due tasks)"))
		return b.String()
	}

	maxRows := l.maxScrollRows()
	if maxRows < 6 {
		maxRows = 6
	}
	gridCursor := m.calGridCursor
	if gridCursor >= len(rows) {
		gridCursor = 0
	}
	win := computeScrollWindow(gridCursor, len(rows), maxRows)
	if hint := m.scrollHint(win); hint != "" {
		b.WriteString(hint)
		b.WriteString("\n")
	}
	for i := win.Start; i < win.End; i++ {
		row := rows[i]
		selected := i == gridCursor && (row.kind == "overdue" || row.kind == "allday" || row.kind == "task")
		b.WriteString(renderCalDayGridRow(row, d, now, selected, l.fullW))
		b.WriteString("\n")
	}
	return b.String()
}

func (m model) renderCalWeek(idx calIndex, l layout) string {
	start := weekStart(m.calSelected())
	end := start.AddDate(0, 0, 6)
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%s – %s",
		start.Format("2 Jan"), end.Format("2 Jan 2006"))))
	b.WriteString("\n")
	if h := m.keyHint("j/k scroll · h/l day · Enter day · [/] week · d/w/m/y"); h != "" {
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
	for i := 0; i < 7; i++ {
		d := weekStart(m.calSelected()).AddDate(0, 0, i)
		n := idx.countOn(d)
		line := fmt.Sprintf("%s  %2d %s", d.Format("Mon"), d.Day(), d.Format("Jan"))
		if n > 0 {
			line += fmt.Sprintf("  · %d task(s)", n)
		}
		if dateKey(d) == dateKey(m.calSelected()) {
			line = listSelStyle.Render(iconTaskSel + " " + line)
		} else if dateKey(d) == dateKey(time.Now()) {
			line = calTodayStyle.Render("  "+line) + " " + calTodayStyle.Render("today")
		} else {
			line = listIdleStyle.Render("  " + line)
		}
		b.WriteString(line)
		b.WriteString("\n")
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

func (m model) renderCalYear(idx calIndex, l layout) string {
	y := m.calSelected().Year()
	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%d", y)))
	b.WriteString("\n")
	if h := m.keyHint("j/k month · Enter month view · [/] prev/next year"); h != "" {
		b.WriteString(h)
	}
	b.WriteString("\n\n")

	cols := 4
	if l.fullW < 80 {
		cols = 3
	}
	if l.fullW < 56 {
		cols = 2
	}
	cellW := l.fullW / cols
	if cellW < 14 {
		cellW = 14
	}

	monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	for mth := 1; mth <= 12; mth++ {
		col := (mth - 1) % cols
		if col == 0 && mth > 1 {
			b.WriteString("\n")
		}
		mo := time.Month(mth)
		n := idx.countInMonth(y, mo)
		label := monthNames[mth-1]
		count := hintStyle.Render(" 0 tasks")
		if n > 0 {
			count = calDueStyle.Render(fmt.Sprintf(" %d tasks", n))
		}
		line := label + count
		isSel := int(m.calSelected().Month()) == mth
		isNow := time.Now().Year() == y && time.Now().Month() == mo
		var cell string
		switch {
		case isSel:
			cell = calMonthSelStyle.Width(cellW - 1).Render(line)
		case isNow:
			cell = calTodayCellStyle.Width(cellW - 1).Render(line)
		default:
			cell = calMonthIdleStyle.Width(cellW - 1).Render(line)
		}
		b.WriteString(cell)
	}
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
	return m
}

func (m model) calMoveVert(delta int) model {
	idx := m.calIdx()
	switch m.calMode {
	case calModeDay:
		rows, _ := m.calDayRows(idx)
		if len(rows) > 0 {
			m.calGridCursor = clamp(m.calGridCursor+delta, 0, len(rows)-1)
			m.syncCalTaskFromGrid(rows)
		}
	case calModeWeek:
		rows := buildCalWeekTimeline(idx, weekStart(m.calSelected()), time.Now())
		if len(rows) > 0 {
			m.calGridCursor = clamp(m.calGridCursor+delta, 0, len(rows)-1)
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
		newMonth := clamp(int(d.Month())+delta, 1, 12)
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
	calSelStyle        = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorMauve)
	calDueStyle        = lipgloss.NewStyle().Foreground(colorTeal).Bold(true)
	calIdleStyle       = lipgloss.NewStyle().Foreground(colorText)
	calTodayStyle      = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	calTodayCellStyle  = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	calAllDayStyle     = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)
	calWeekHeadStyle   = lipgloss.NewStyle().Foreground(colorMuted).Bold(true)
	calWeekCellStyle   = lipgloss.NewStyle().Foreground(colorText)
	calMonthSelStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorBlue).Padding(0, 1)
	calMonthIdleStyle  = lipgloss.NewStyle().Foreground(colorSubtext).Padding(0, 1)
)
