package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const calYearMonthCardLines = 10

type calYearMonthStats struct {
	tasks      int
	done       int
	activeDays int
}

func (idx calIndex) yearMonthStats(year int, month time.Month) calYearMonthStats {
	stats := calYearMonthStats{}
	for key, entries := range idx.byDate {
		day, err := time.ParseInLocation("2006-01-02", key, time.Local)
		if err != nil || day.Year() != year || day.Month() != month {
			continue
		}
		if len(entries) > 0 {
			stats.activeDays++
		}
		stats.tasks += len(entries)
		for _, entry := range entries {
			if entry.Done() {
				stats.done++
			}
		}
	}
	return stats
}

func calYearGridColumns(fullW int) int {
	switch {
	case fullW >= 156:
		return 6
	case fullW >= 108:
		return 4
	case fullW >= 72:
		return 3
	default:
		return 2
	}
}

func calYearGridColumnsForSize(fullW, innerLines int) int {
	if fullW >= 156 && innerLines >= 5+3*calYearMonthCardLines {
		return 4
	}
	return calYearGridColumns(fullW)
}

func calYearDetailedFits(fullW, innerLines int) bool {
	cols := calYearGridColumnsForSize(fullW, innerLines)
	rows := (12 + cols - 1) / cols
	return fullW/cols >= 23 && 5+rows*calYearMonthCardLines <= innerLines
}

func calYearCardWidths(fullW, cols int) []int {
	const gap = 1
	usable := max(fullW-gap*(cols-1), cols)
	base, remainder := usable/cols, usable%cols
	widths := make([]int, cols)
	for i := range widths {
		widths[i] = base
		if i < remainder {
			widths[i]++
		}
	}
	return widths
}

func calYearBorderStyle(selected, current bool) lipgloss.Style {
	switch {
	case selected:
		return lipgloss.NewStyle().Foreground(colorMauve).Bold(true)
	case current:
		return lipgloss.NewStyle().Foreground(colorGreen)
	default:
		return lipgloss.NewStyle().Foreground(colorOverlay)
	}
}

func calYearCardLine(content string, width int, border lipgloss.Style) string {
	inner := max(width-2, 1)
	return border.Render("│") +
		padToWidth(truncateRenderedWidth(content, inner), inner) +
		border.Render("│")
}

func calYearCardBorder(label string, width int, top bool, border lipgloss.Style) string {
	inner := max(width-2, 1)
	if !top {
		return border.Render("╰" + strings.Repeat("─", inner) + "╯")
	}
	label = " " + label + " "
	label = truncateRenderedWidth(label, inner)
	return border.Render("╭") + label +
		border.Render(strings.Repeat("─", max(inner-lipgloss.Width(label), 0))+"╮")
}

func calYearDayCell(
	idx calIndex,
	day, selected, now time.Time,
	width int,
	focusByDate map[string]*ticktick.FocusStats,
) string {
	if day.IsZero() {
		return strings.Repeat(" ", width)
	}
	entries := idx.on(day)
	marker := ""
	if len(entries) > 0 {
		allDone := true
		for _, entry := range entries {
			if !entry.Done() {
				allDone = false
				break
			}
		}
		if allDone {
			marker = "✓"
		} else {
			marker = "•"
		}
	}
	if stats := focusByDate[dateKey(day)]; stats != nil && stats.FullPomoCount > 0 {
		if stats.FullPomoCount > 9 {
			marker = "+"
		} else {
			marker = fmt.Sprint(stats.FullPomoCount)
		}
	}
	label := fmt.Sprintf("%2d%s", day.Day(), marker)
	style := calMonthDayNumStyle
	switch {
	case dateKey(day) == dateKey(selected):
		style = calMonthDaySelStyle
	case dateKey(day) == dateKey(now):
		style = calMonthDayTodayStyle
	case len(entries) >= 3:
		style = lipgloss.NewStyle().Foreground(colorMauve).Bold(true)
	case len(entries) > 0:
		style = calDueStyle
	}
	return padToWidth(style.Align(lipgloss.Center).Render(label), width)
}

func renderCalYearMonthCard(
	idx calIndex,
	year int,
	month time.Month,
	selected, now time.Time,
	width int,
	monday bool,
	focusByDate map[string]*ticktick.FocusStats,
) []string {
	stats := idx.yearMonthStats(year, month)
	isSelected := selected.Year() == year && selected.Month() == month
	isCurrent := now.Year() == year && now.Month() == month
	border := calYearBorderStyle(isSelected, isCurrent)
	title := month.String()
	if stats.tasks > 0 {
		title += fmt.Sprintf(" · %d", stats.tasks)
	}
	lines := []string{calYearCardBorder(title, width, true, border)}

	inner := max(width-2, 1)
	colW := calMonthColumnWidths(inner)
	dayNames := []string{"Su", "Mo", "Tu", "We", "Th", "Fr", "Sa"}
	if monday {
		dayNames = []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	}
	headers := make([]string, 7)
	for i := range headers {
		headers[i] = padToWidth(
			calWeekHeadStyle.Align(lipgloss.Center).Render(dayNames[i]),
			colW[i],
		)
	}
	lines = append(lines, calYearCardLine(lipgloss.JoinHorizontal(lipgloss.Top, headers...), width, border))

	first := time.Date(year, month, 1, 0, 0, 0, 0, time.Local)
	offset := int(first.Weekday())
	if monday {
		offset = (offset + 6) % 7
	}
	days := time.Date(year, month+1, 0, 0, 0, 0, 0, time.Local).Day()
	dayNumber := 1
	for week := 0; week < 6; week++ {
		cells := make([]string, 7)
		for weekday := 0; weekday < 7; weekday++ {
			var day time.Time
			if !(week == 0 && weekday < offset) && dayNumber <= days {
				day = time.Date(year, month, dayNumber, 0, 0, 0, 0, time.Local)
				dayNumber++
			}
			cells[weekday] = calYearDayCell(idx, day, selected, now, colW[weekday], focusByDate)
		}
		lines = append(lines, calYearCardLine(lipgloss.JoinHorizontal(lipgloss.Top, cells...), width, border))
	}
	summary := fmt.Sprintf("%d done · %d active days", stats.done, stats.activeDays)
	lines = append(lines, calYearCardLine(hintStyle.Render(" "+summary), width, border))
	lines = append(lines, calYearCardBorder("", width, false, border))
	return lines
}

func renderCalYearCompactCard(
	idx calIndex,
	year int,
	month time.Month,
	selected, now time.Time,
	width int,
) string {
	stats := idx.yearMonthStats(year, month)
	label := month.String()[:3]
	if stats.tasks > 0 {
		label += fmt.Sprintf("  %d tasks · %d done", stats.tasks, stats.done)
	} else {
		label += "  no tasks"
	}
	style := calMonthIdleStyle
	if selected.Year() == year && selected.Month() == month {
		style = calMonthSelStyle
	} else if now.Year() == year && now.Month() == month {
		style = calTodayCellStyle
	}
	return padToWidth(truncateRenderedWidth(style.Render(label), width), width)
}

func renderCalYearCards(cards [][]string, widths []int, cols int) string {
	var out []string
	for start := 0; start < len(cards); start += cols {
		end := min(start+cols, len(cards))
		rowHeight := len(cards[start])
		for line := 0; line < rowHeight; line++ {
			parts := make([]string, 0, cols)
			for i := start; i < end; i++ {
				parts = append(parts, padToWidth(cards[i][line], widths[i-start]))
			}
			out = append(out, strings.Join(parts, " "))
		}
	}
	return strings.Join(out, "\n")
}

func (m model) renderCalYear(idx calIndex, l layout) string {
	year := m.calSelected().Year()
	selected := m.calSelected()
	now := time.Now()
	totalTasks, totalDone, activeDays := 0, 0, 0
	for month := time.January; month <= time.December; month++ {
		stats := idx.yearMonthStats(year, month)
		totalTasks += stats.tasks
		totalDone += stats.done
		activeDays += stats.activeDays
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render(fmt.Sprintf("%d", year)))
	b.WriteString(hintStyle.Render(fmt.Sprintf(
		"  · %d tasks · %d done · %d active days",
		totalTasks, totalDone, activeDays,
	)))
	b.WriteString("\n")
	if h := m.keyHint("h/l month · j/k month row · Enter open month · [/] year · t today"); h != "" {
		b.WriteString(h)
	}
	b.WriteString("\n\n")

	cols := calYearGridColumnsForSize(l.fullW, l.innerLines)
	widths := calYearCardWidths(l.fullW, cols)
	if calYearDetailedFits(l.fullW, l.innerLines) {
		cards := make([][]string, 0, 12)
		for month := time.January; month <= time.December; month++ {
			cards = append(cards, renderCalYearMonthCard(
				idx, year, month, selected, now,
				widths[(int(month)-1)%cols],
				m.uiSettings.weekStartsMonday(),
				m.calFocusByDate,
			))
		}
		b.WriteString(renderCalYearCards(cards, widths, cols))
		return b.String()
	}

	for month := time.January; month <= time.December; month++ {
		col := (int(month) - 1) % cols
		b.WriteString(renderCalYearCompactCard(idx, year, month, selected, now, widths[col]))
		if col == cols-1 || month == time.December {
			b.WriteString("\n")
		} else {
			b.WriteString(" ")
		}
	}
	return b.String()
}
