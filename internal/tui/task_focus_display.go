package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

var taskFocusInlineStyle = lipgloss.NewStyle().Foreground(colorPeach).Bold(true)

func (m model) taskFocusSummary(t ticktick.Task) (ticktick.TaskFocusSummary, bool) {
	if s, ok := m.taskFocusByID[t.ID]; ok && s.Visible() {
		return s, true
	}
	key := ticktick.NormalizeFocusTaskTitle(t.Title)
	if key != "" {
		if s, ok := m.taskFocusByTitle[key]; ok && s.Visible() {
			return s, true
		}
	}
	return ticktick.TaskFocusSummary{}, false
}

func renderTaskFocusInline(s ticktick.TaskFocusSummary) string {
	if !s.Visible() {
		return ""
	}
	sessions := s.FullSessions
	if sessions == 0 {
		sessions = s.LoggedSessions
	}
	label := fmt.Sprintf("%s %d · %s", iconPomodoro, sessions, formatFocusTotal(int(s.TotalSeconds)))
	return taskFocusInlineStyle.Render(label)
}

func maxTaskFocusInlineW(m model, rows []taskListRow) int {
	max := 0
	for _, r := range rows {
		if s, ok := m.taskFocusSummary(r.Task); ok {
			w := lipgloss.Width(renderTaskFocusInline(s))
			if w > max {
				max = w
			}
		}
	}
	return max
}

func renderTaskFocusDetail(s ticktick.TaskFocusSummary) string {
	if !s.Visible() {
		return ""
	}
	sessions := s.FullSessions
	if sessions == 0 {
		sessions = s.LoggedSessions
	}
	word := "sessions"
	if sessions == 1 {
		word = "session"
	}
	return fmt.Sprintf("focus %d %s · %s", sessions, word, formatFocusTotal(int(s.TotalSeconds)))
}
