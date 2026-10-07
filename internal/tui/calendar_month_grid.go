package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

var (
	calMonthDayNumStyle   = lipgloss.NewStyle().Foreground(colorSubtext)
	calMonthDaySelStyle   = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorMauve).Padding(0, 1)
	calMonthDayTodayStyle = lipgloss.NewStyle().Bold(true).Foreground(colorBase).Background(colorGreen).Padding(0, 1)
	// Days inside the open month read lighter than the padding cells around them.
	calMonthInFrame  = colorSubtext
	calMonthOutFrame = colorSurface
)

func renderCalMonthCellBordered(inner []string, innerW, innerH, colW int, fg lipgloss.Color) []string {
	if innerW < 1 {
		innerW = 1
	}
	if innerH < 1 {
		innerH = 1
	}
	for len(inner) < innerH {
		inner = append(inner, strings.Repeat(" ", innerW))
	}
	if len(inner) > innerH {
		inner = inner[:innerH]
	}
	border := lipgloss.NewStyle().Foreground(fg)
	top := border.Render("┌" + strings.Repeat("─", innerW) + "┐")
	bot := border.Render("└" + strings.Repeat("─", innerW) + "┘")
	side := border.Render("│")
	lines := make([]string, 0, innerH+calMonthBorderLines)
	lines = append(lines, top)
	for i := 0; i < innerH; i++ {
		content := padToWidth(truncateRenderedWidth(inner[i], innerW), innerW)
		lines = append(lines, side+content+side)
	}
	lines = append(lines, bot)
	return normalizeCellLines(lines, innerH+calMonthBorderLines, colW)
}

func renderCalMonthEmptyCell(colW, innerH int) []string {
	innerW := colW - 2
	if innerW < 1 {
		innerW = 1
	}
	inner := make([]string, innerH)
	return renderCalMonthCellBordered(inner, innerW, innerH, colW, calMonthOutFrame)
}

func renderCalMonthCell(
	_ calIndex,
	cur time.Time,
	colW, innerH int,
	selDay time.Time,
	now time.Time,
	focusStats *ticktick.FocusStats,
	dailyGoal int,
) []string {
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

	if innerH > 1 {
		completed, focusedMinutes := 0, 0
		if focusStats != nil {
			completed = focusStats.FullPomoCount
			focusedMinutes = int((focusStats.ClaimedSeconds() + 30) / 60)
		}
		focusLabel := fmt.Sprintf("%s %d/%d", iconPomodoro, completed, max(dailyGoal, 1))
		if innerW >= 20 && focusedMinutes > 0 {
			focusLabel += fmt.Sprintf(" · %dm", focusedMinutes)
		}
		inner[innerH-1] = truncateInner(pomoTodayStyle(completed, dailyGoal).Render(focusLabel), innerW)
	}

	fg := calMonthInFrame
	switch {
	case isSel:
		fg = colorMauve
	case isToday:
		fg = colorGreen
	}
	return renderCalMonthCellBordered(inner, innerW, innerH, colW, fg)
}

func renderCalMonthDOWHeader(lay calMonthLayout) string {
	headers := []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	parts := make([]string, 7)
	for i, label := range headers {
		parts[i] = padToWidth(
			calWeekHeadStyle.Align(lipgloss.Center).Render(label),
			lay.ColW[i],
		)
	}
	return joinFixedColumns(parts, lay.ColW[:], lay.FullW)
}

func (m model) renderCalMonthGrid(idx calIndex, l layout, monthHeaderLines int) string {
	overhead := calMonthOverheadBeforeGrid(calViewTabLines, monthHeaderLines)
	lay := computeCalMonthLayout(l.fullW, l.innerLines, overhead)

	first := time.Date(m.calSelected().Year(), m.calSelected().Month(), 1, 0, 0, 0, 0, time.Local)
	daysInMonth := time.Date(m.calSelected().Year(), m.calSelected().Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
	startDow := (int(first.Weekday()) + 6) % 7
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
			cells = append(cells, renderCalMonthCell(
				idx, cur, colW, lay.CellInnerH, sel, now,
				m.calFocusByDate[dateKey(cur)], m.uiSettings.pomoDailyGoal(),
			))
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
