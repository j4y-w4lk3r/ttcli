package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	calMonthWeekRows    = 6
	calMonthBorderLines = 2 // top + bottom border rows per cell
)

// calMonthLayout sizes the month grid to exactly fill the calendar pane.
type calMonthLayout struct {
	ColW       [7]int
	CellInnerH int // content lines inside a cell border
	WeekRows   int
	PadLines   int
	FullW      int
}

func (lay calMonthLayout) cellOuterH() int {
	return lay.CellInnerH + calMonthBorderLines
}

// calMonthColumnWidths splits fullW into seven columns; remainder goes to first columns.
func calMonthColumnWidths(fullW int) [7]int {
	if fullW < 7 {
		fullW = 7
	}
	base := fullW / 7
	rem := fullW % 7
	var cols [7]int
	for i := 0; i < 7; i++ {
		cols[i] = base
		if i < rem {
			cols[i]++
		}
	}
	return cols
}

// computeCalMonthLayout derives cell sizes from pane dimensions.
// overheadLines is every line above the grid body (tabs, titles, hints).
func computeCalMonthLayout(fullW, innerLines, overheadLines int) calMonthLayout {
	lay := calMonthLayout{
		ColW:     calMonthColumnWidths(fullW),
		WeekRows: calMonthWeekRows,
		FullW:    fullW,
	}
	minOuter := calMonthBorderLines + 2 // day number + at least one task line
	if innerLines < overheadLines+minOuter+calMonthWeekRows {
		lay.CellInnerH = 2
		return lay
	}
	available := innerLines - overheadLines
	weekBudget := available - 1 // single-line DOW header
	if weekBudget < calMonthWeekRows*minOuter {
		lay.CellInnerH = 2
		return lay
	}
	cellOuterH := weekBudget / calMonthWeekRows
	if cellOuterH < minOuter {
		cellOuterH = minOuter
	}
	lay.CellInnerH = cellOuterH - calMonthBorderLines
	if lay.CellInnerH < 2 {
		lay.CellInnerH = 2
	}
	lay.PadLines = weekBudget - cellOuterH*calMonthWeekRows
	return lay
}

func (lay calMonthLayout) colInnerW(col int) int {
	w := lay.ColW[col] - 2
	if w < 4 {
		w = 4
	}
	return w
}

func normalizeCellLines(lines []string, outerH, colW int) []string {
	if outerH < 1 {
		outerH = 1
	}
	if len(lines) > outerH {
		lines = lines[:outerH]
	}
	for len(lines) < outerH {
		lines = append(lines, strings.Repeat(" ", colW))
	}
	for i, ln := range lines {
		lines[i] = padToWidth(ln, colW)
	}
	return lines
}

func joinMonthWeekRow(cells [][]string, lay calMonthLayout) []string {
	if len(cells) == 0 {
		return nil
	}
	outerH := lay.cellOuterH()
	out := make([]string, outerH)
	for line := 0; line < outerH; line++ {
		parts := make([]string, len(cells))
		for i, cell := range cells {
			cell = normalizeCellLines(cell, outerH, lay.ColW[i])
			parts[i] = cell[line]
		}
		out[line] = padToWidth(lipgloss.JoinHorizontal(lipgloss.Top, parts...), lay.FullW)
	}
	return out
}

func padCalMonthGridLines(lines []string, lay calMonthLayout) []string {
	if lay.PadLines <= 0 {
		return lines
	}
	out := append([]string(nil), lines...)
	for i := 0; i < lay.PadLines; i++ {
		out = append(out, strings.Repeat(" ", lay.FullW))
	}
	return out
}

// calMonthOverheadBeforeGrid counts lines consumed before the grid (not including DOW).
func calMonthOverheadBeforeGrid(calendarViewOverhead, monthHeaderLines int) int {
	return calendarViewOverhead + monthHeaderLines
}
