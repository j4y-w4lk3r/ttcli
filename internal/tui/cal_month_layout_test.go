package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestCalMonthColumnWidthsSumToFull(t *testing.T) {
	for _, fullW := range []int{80, 120, 77, 40, 200} {
		cols := calMonthColumnWidths(fullW)
		sum := 0
		for _, w := range cols {
			sum += w
		}
		if sum != fullW {
			t.Fatalf("fullW=%d sum=%d cols=%v", fullW, sum, cols)
		}
	}
}

func TestComputeCalMonthLayoutFillsHeight(t *testing.T) {
	lay := computeCalMonthLayout(140, 40, 5)
	used := 1 + lay.cellOuterH()*lay.WeekRows + lay.PadLines
	if 5+used > 40 {
		t.Fatalf("grid exceeds innerLines: overhead+used=%d outerH=%d innerH=%d pad=%d",
			5+used, lay.cellOuterH(), lay.CellInnerH, lay.PadLines)
	}
	if lay.CellInnerH < 2 {
		t.Fatalf("cellInnerH=%d too small", lay.CellInnerH)
	}
}

func TestCalMonthCellOuterHeight(t *testing.T) {
	lay := computeCalMonthLayout(140, 40, 5)
	cell := renderCalMonthEmptyCell(lay.ColW[0], lay.CellInnerH)
	if len(cell) != lay.cellOuterH() {
		t.Fatalf("empty cell lines=%d want outerH=%d", len(cell), lay.cellOuterH())
	}
}

func TestJoinMonthWeekRowExactWidth(t *testing.T) {
	lay := computeCalMonthLayout(77, 30, 5)
	cells := make([][]string, 7)
	for i := range cells {
		cells[i] = renderCalMonthEmptyCell(lay.ColW[i], lay.CellInnerH)
	}
	for _, ln := range joinMonthWeekRow(cells, lay) {
		if lipgloss.Width(ln) != lay.FullW {
			t.Fatalf("row width=%d want %d", lipgloss.Width(ln), lay.FullW)
		}
	}
	if len(joinMonthWeekRow(cells, lay)) != lay.cellOuterH() {
		t.Fatal("week row height mismatch")
	}
}

func TestCalMonthColumnWidthsRemainder(t *testing.T) {
	cols := calMonthColumnWidths(80)
	if cols[0] != 12 || cols[3] != 11 {
		t.Fatalf("expected remainder on first cols, got %v", cols)
	}
}

func TestCalMonthDOWHeaderSingleLine(t *testing.T) {
	lay := computeCalMonthLayout(80, 30, 5)
	header := renderCalMonthDOWHeader(lay)
	if strings.Contains(header, "\n") {
		t.Fatal("DOW header must be a single line")
	}
}

func TestNormalizeCellLinesExactHeight(t *testing.T) {
	got := normalizeCellLines([]string{"a", "b", "c"}, 5, 10)
	if len(got) != 5 {
		t.Fatalf("len=%d want 5", len(got))
	}
}
