package tui

import (
	"fmt"
	"strings"
)

// maxVisibleRows is a hard cap on list/task lines shown at once (scroll for more).
const maxVisibleRows = 40

// paneScrollRows is how many item lines fit below title+hint in a pane.
func paneScrollRows(innerLines, reservedAfterItems int) int {
	rows := innerLines - 2 - reservedAfterItems
	if rows > maxVisibleRows {
		rows = maxVisibleRows
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

// scrollWindow is a slice of [start,end) rows to render around cursor.
type scrollWindow struct {
	Start  int
	End    int
	Above  int
	Below  int
	Total  int
	Cursor int
}

func computeScrollWindow(cursor, total, maxRows int) scrollWindow {
	if total <= 0 {
		return scrollWindow{}
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= total {
		cursor = total - 1
	}
	if maxRows < 1 {
		maxRows = 1
	}
	if total <= maxRows {
		return scrollWindow{Start: 0, End: total, Total: total, Cursor: cursor}
	}

	start := cursor - maxRows/2
	if start < 0 {
		start = 0
	}
	end := start + maxRows
	if end > total {
		end = total
		start = end - maxRows
	}
	return scrollWindow{
		Start:  start,
		End:    end,
		Above:  start,
		Below:  total - end,
		Total:  total,
		Cursor: cursor,
	}
}

func computeViewportWindow(start, cursor, total, maxRows int) scrollWindow {
	if total <= 0 {
		return scrollWindow{}
	}
	maxRows = max(maxRows, 1)
	start = clamp(start, 0, max(total-maxRows, 0))
	cursor = clamp(cursor, 0, total-1)
	if cursor < start {
		start = cursor
	} else if cursor >= start+maxRows {
		start = cursor - maxRows + 1
	}
	start = clamp(start, 0, max(total-maxRows, 0))
	end := min(start+maxRows, total)
	return scrollWindow{
		Start: start, End: end, Above: start, Below: total - end,
		Total: total, Cursor: cursor,
	}
}

func renderScrollHint(win scrollWindow) string {
	if win.Total == 0 || (win.Above == 0 && win.Below == 0) {
		return ""
	}
	var parts []string
	if win.Above > 0 {
		parts = append(parts, fmt.Sprintf("↑%d", win.Above))
	}
	parts = append(parts, fmt.Sprintf("%d/%d", win.Cursor+1, win.Total))
	if win.Below > 0 {
		parts = append(parts, fmt.Sprintf("↓%d", win.Below))
	}
	return hintStyle.Render(strings.Join(parts, " "))
}

func maxRowsForPane(itemRows int) int {
	if itemRows < 3 {
		return 3
	}
	return itemRows
}
