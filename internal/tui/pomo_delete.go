package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func deletePauseSpellCmd(spell focus.PauseSpell) tea.Cmd {
	return func() tea.Msg {
		return pomoChangedMsg{op: "delete-pause", err: focus.DeletePauseSpell(spell)}
	}
}

func (m model) selectedPomoGridRow() (dayGridRow, bool) {
	grid := m.pomoDayGrid()
	if m.pomoGridCursor < 0 || m.pomoGridCursor >= len(grid) {
		return dayGridRow{}, false
	}
	return grid[m.pomoGridCursor], true
}

func (m model) pomoDeleteCmd() tea.Cmd {
	if row, ok := m.selectedPomoGridRow(); ok && row.kind == "pause" {
		return deletePauseSpellCmd(row.pause)
	}
	r, ok := m.selectedPomoRecord()
	if !ok {
		return nil
	}
	return deletePomodoroCmd(m.client, r.ID)
}

func (m *model) syncPomoSelectionFromLegend() {
	slices := m.pomoLegendSlices()
	if m.pomoLegendCursor < 0 || m.pomoLegendCursor >= len(slices) {
		return
	}
	slice := slices[m.pomoLegendCursor]
	grid := m.pomoDayGrid()
	m.pomoFollowNow = false

	if slice.Kind == pomoSessionPause {
		base := focus.PauseLegendBaseTitle(slice.Title)
		for i, row := range grid {
			if row.kind == "pause" && tasksMatchPauseTitle(row.pause.TaskTitle, base) {
				m.pomoGridCursor = i
				return
			}
		}
		return
	}

	recs := m.focusStatsRecords()
	for i, r := range recs {
		if r.TaskTitle() != slice.Title {
			continue
		}
		m.pomoCursor = i
		m.pomoGridCursor = gridRowForRecord(grid, i)
		return
	}
}

func tasksMatchPauseTitle(a, b string) bool {
	return focus.PauseLegendTitle(a) == focus.PauseLegendTitle(b) || a == b
}

func (m model) pomoDeleteHint() string {
	if row, ok := m.selectedPomoGridRow(); ok && row.kind == "pause" {
		return "x delete pause"
	}
	if _, ok := m.selectedPomoRecord(); ok {
		return "x delete session"
	}
	return "x delete"
}
