package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var pomoTimelineSelStyle = lipgloss.NewStyle().
	Bold(true)

func isSelectablePomoGridRow(row dayGridRow) bool {
	switch row.kind {
	case "pomo", "pause", "live":
		return true
	default:
		return false
	}
}

func firstSelectablePomoGridRow(grid []dayGridRow) int {
	for i, row := range grid {
		if isSelectablePomoGridRow(row) {
			return i
		}
	}
	return -1
}

func nearestSelectablePomoGridRow(grid []dayGridRow, from, preferDir int) int {
	if len(grid) == 0 {
		return -1
	}
	if from < 0 {
		from = 0
	}
	if from >= len(grid) {
		from = len(grid) - 1
	}
	if isSelectablePomoGridRow(grid[from]) {
		return from
	}
	for dist := 1; dist < len(grid); dist++ {
		if preferDir >= 0 {
			if i := from + dist; i < len(grid) && isSelectablePomoGridRow(grid[i]) {
				return i
			}
		}
		if preferDir <= 0 {
			if i := from - dist; i >= 0 && isSelectablePomoGridRow(grid[i]) {
				return i
			}
		}
	}
	return -1
}

func pomoGridHasSessions(grid []dayGridRow) bool {
	return firstSelectablePomoGridRow(grid) >= 0
}

func nextPomoGridRow(grid []dayGridRow, cursor, delta int) int {
	if len(grid) == 0 {
		return 0
	}
	cursor += delta
	if cursor < 0 {
		return 0
	}
	if cursor >= len(grid) {
		return len(grid) - 1
	}
	return cursor
}

// nextPomoGridCursor always moves one visual row for smooth timeline scrolling.
func nextPomoGridCursor(grid []dayGridRow, cursor, delta int) int {
	return nextPomoGridRow(grid, cursor, delta)
}

func nextSelectablePomoGridRow(grid []dayGridRow, cursor, delta int) int {
	if len(grid) == 0 {
		return 0
	}
	if cursor < 0 || cursor >= len(grid) {
		cursor = 0
	}
	if !isSelectablePomoGridRow(grid[cursor]) {
		if idx := nearestSelectablePomoGridRow(grid, cursor, delta); idx >= 0 {
			return idx
		}
	}
	for i := 0; i < len(grid); i++ {
		cursor += delta
		if cursor < 0 {
			if idx := firstSelectablePomoGridRow(grid); idx >= 0 {
				return idx
			}
			return 0
		}
		if cursor >= len(grid) {
			for j := len(grid) - 1; j >= 0; j-- {
				if isSelectablePomoGridRow(grid[j]) {
					return j
				}
			}
			return len(grid) - 1
		}
		if isSelectablePomoGridRow(grid[cursor]) {
			return cursor
		}
	}
	return cursor
}

func pomoTimelineSelectionPrefix(selected bool) string {
	if !selected {
		return "  "
	}
	return taskSelStyle.Render("● ")
}

func highlightPomoTimelineRow(line string, selected bool) string {
	if !selected {
		return line
	}
	return pomoTimelineSelStyle.Render(line)
}
