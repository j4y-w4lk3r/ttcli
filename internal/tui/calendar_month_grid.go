package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

var (
	calMonthCellBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorSurface)
	calMonthCellSelBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorMauve)
	calMonthCellTodayBorder = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(colorGreen)
	calMonthDayNumStyle = lipgloss.NewStyle().Foreground(colorMuted)
	calMonthDaySelStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorMauve).Padding(0, 1)
	calMonthDayTodayStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorGreen).Padding(0, 1)
)

func calMonthTaskChip(t ticktick.Task, width int, done bool) string {
	if width < 6 {
		width = 6
	}
	clock := dueTaskClock(t)
	if t.IsAllDay || clock == "" {
		clock = "·"
	}
	title := t.Title
	if title == "" {
		title = "(untitled)"
	}
	clockPart := pomoTimeStyle.Render(clock)
	clockW := lipgloss.Width(clockPart)
	maxTitle := width - clockW - 2
	if maxTitle < 3 {
		maxTitle = 3
	}
	if lipgloss.Width(title) > maxTitle {
		title = truncateRunes(title, max(1, maxTitle-1)) + "…"
	}
	col := taskPriorityColor(t.Priority.Int())
	bar := lipgloss.NewStyle().Foreground(col).Render("▌")
	titleSt := listIdleStyle
	if done {
		titleSt = taskDoneStyle
	}
	line := bar + titleSt.Render(title)
	gap := width - lipgloss.Width(line) - clockW
	if gap < 1 {
		gap = 1
	}
	return truncateInner(line+strings.Repeat(" ", gap)+clockPart, width)
}

func renderCalMonthCellBordered(inner []string, innerW, innerH, colW int, border lipgloss.Style) []string {
	for len(inner) < innerH {
		inner = append(inner, strings.Repeat(" ", innerW))
	}
	if len(inner) > innerH {
		inner = inner[:innerH]
	}
	boxed := border.Width(innerW).Height(innerH).Render(strings.Join(inner, "\n"))
	lines := strings.Split(strings.TrimSuffix(boxed, "\n"), "\n")
	return normalizeCellLines(lines, innerH+calMonthBorderLines, colW)
}

func renderCalMonthEmptyCell(colW, innerH int) []string {
	innerW := colW - 2
	if innerW < 1 {
		innerW = 1
	}
	inner := make([]string, innerH)
	border := lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(colorMuted)
	return renderCalMonthCellBordered(inner, innerW, innerH, colW, border)
}

func renderCalMonthCell(
	idx calIndex,
	cur time.Time,
	colW, innerH int,
	selDay time.Time,
	now time.Time,
) []string {
	tasks := idx.onSorted(cur)
	inMonth := cur.Month() == selDay.Month() && cur.Year() == selDay.Year()
	isSel := inMonth && cur.Day() == selDay.Day()
	isToday := dateKey(cur) == dateKey(now)

	innerW := colW - 2
	if innerW < 4 {
		innerW = 4
	}
	inner := make([]string, innerH)
	for i := range inner {
		inner[i] = strings.Repeat(" ", innerW)
	}

	dayLabel := fmt.Sprintf("%d", cur.Day())
	var dayStyled string
	switch {
	case isSel:
		dayStyled = calMonthDaySelStyle.Render(dayLabel)
	case isToday:
		dayStyled = calMonthDayTodayStyle.Render(dayLabel)
	default:
		dayStyled = calMonthDayNumStyle.Render(dayLabel)
	}
	inner[0] = truncateInner(alignRightInWidth("", dayStyled, innerW), innerW)

	taskLines := innerH - 1
	if taskLines < 1 {
		taskLines = 1
	}
	show := taskLines
	if len(tasks) > show {
		show = taskLines - 1
		if show < 1 {
			show = 1
		}
	}
	for i := 0; i < show && i < len(tasks); i++ {
		done := calTaskShouldBeDone(tasks[i], cur, now)
		inner[i+1] = truncateInner(calMonthTaskChip(tasks[i], innerW-1, done), innerW)
	}
	if len(tasks) > show && taskLines > show && show+1 < innerH {
		extra := len(tasks) - show
		inner[show+1] = truncateInner(hintStyle.Render(fmt.Sprintf("+%d more", extra)), innerW)
	}

	border := calMonthCellBorder
	switch {
	case isSel:
		border = calMonthCellSelBorder
	case isToday:
		border = calMonthCellTodayBorder
	}
	return renderCalMonthCellBordered(inner, innerW, innerH, colW, border)
}

func renderCalMonthDOWHeader(lay calMonthLayout) string {
	headers := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	parts := make([]string, 7)
	for i, label := range headers {
		parts[i] = padToWidth(
			calWeekHeadStyle.Align(lipgloss.Center).Render(label),
			lay.ColW[i],
		)
	}
	return padToWidth(lipgloss.JoinHorizontal(lipgloss.Top, parts...), lay.FullW)
}

func (m model) renderCalMonthGrid(idx calIndex, l layout, monthHeaderLines int) string {
	overhead := calMonthOverheadBeforeGrid(2, monthHeaderLines)
	lay := computeCalMonthLayout(l.fullW, l.innerLines, overhead)

	first := time.Date(m.calSelected().Year(), m.calSelected().Month(), 1, 0, 0, 0, 0, time.Local)
	daysInMonth := time.Date(m.calSelected().Year(), m.calSelected().Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	startDow := int(first.Weekday())
	now := time.Now()
	sel := m.calSelected()

	var gridLines []string
	gridLines = append(gridLines, renderCalMonthDOWHeader(lay))

	day := 1
	for week := 0; week < lay.WeekRows; week++ {
		var cells [][]string
		for dow := 0; dow < 7; dow++ {
			colW := lay.ColW[dow]
			if week == 0 && dow < startDow {
				cells = append(cells, renderCalMonthEmptyCell(colW, lay.CellInnerH))
				continue
			}
			if day > daysInMonth {
				cells = append(cells, renderCalMonthEmptyCell(colW, lay.CellInnerH))
				continue
			}
			cur := time.Date(sel.Year(), sel.Month(), day, 0, 0, 0, 0, time.Local)
			cells = append(cells, renderCalMonthCell(idx, cur, colW, lay.CellInnerH, sel, now))
			day++
		}
		gridLines = append(gridLines, joinMonthWeekRow(cells, lay)...)
	}
	gridLines = padCalMonthGridLines(gridLines, lay)
	return strings.Join(gridLines, "\n")
}

func (m model) calMonthHeaderLineCount() int {
	n := 2 // title + blank after header block
	if h := m.keyHint("j/k day · h/l day · Enter day · [/] month · d/w/m/y"); h != "" {
		n = 3 // title + hint + blank
	}
	return n
}
