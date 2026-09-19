package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
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
	var native ticktick.TaskFocusSummary
	for _, summary := range t.FocusSummaries {
		native.FullSessions += summary.PomoCount
		native.LoggedSessions += summary.PomoCount
		native.TotalSeconds += summary.PomoDuration + summary.StopwatchDuration
	}
	if native.Visible() {
		return native, true
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

func taskPlannedFocus(t ticktick.Task) (minutes, pomos int, ok bool) {
	seconds, pomos, ok := t.FocusEstimate()
	if !ok {
		return 0, 0, false
	}
	minutes = int((seconds + 59) / 60)
	if minutes == 0 {
		minutes = pomos * ticktick.StandardPomoMinutes
	}
	if pomos == 0 && minutes > 0 {
		pomos = (minutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes
	}
	return minutes, pomos, true
}

func completedTaskPomos(s ticktick.TaskFocusSummary) int {
	if s.FullSessions > 0 {
		return s.FullSessions
	}
	return s.LoggedSessions
}

func taskFocusPlan(t ticktick.Task, config planning.Config) (minutes, pomos int, inferred bool) {
	if minutes, pomos, ok := taskPlannedFocus(t); ok {
		return minutes, pomos, false
	}
	estimate := planning.EstimateTask(t, config)
	return estimate.Minutes, estimate.Pomos, estimate.Source == planning.EstimateDefault
}

func renderTaskFocusProgressInlineWithConfig(
	t ticktick.Task,
	s ticktick.TaskFocusSummary,
	config planning.Config,
) string {
	planned, plannedPomos, inferred := taskFocusPlan(t, config)
	invested := int((s.TotalSeconds + 30) / 60)
	remaining := max(planned-invested, 0)
	completedPomos := completedTaskPomos(s)
	approx := ""
	if inferred {
		approx = "~"
	}
	label := fmt.Sprintf(
		"%s %d/%d · %s%s left",
		iconPomodoro, completedPomos, plannedPomos,
		approx, planning.FormatMinutes(remaining),
	)
	return taskFocusInlineStyle.Render(label)
}

func renderTaskFocusProgressInline(t ticktick.Task, s ticktick.TaskFocusSummary) string {
	return renderTaskFocusProgressInlineWithConfig(t, s, planning.DefaultConfig())
}

func (m model) taskFocusDisplay(t ticktick.Task) (ticktick.TaskFocusSummary, bool) {
	s, actual := m.taskFocusSummary(t)
	planned, _, _ := taskFocusPlan(t, m.uiSettings.planningConfig())
	return s, actual || planned > 0
}

func maxTaskFocusInlineW(m model, rows []taskListRow) int {
	max := 0
	for _, r := range rows {
		if s, ok := m.taskFocusDisplay(r.Task); ok {
			w := lipgloss.Width(renderTaskFocusProgressInlineWithConfig(
				r.Task, s, m.uiSettings.planningConfig(),
			))
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

func renderTaskFocusProgressDetail(t ticktick.Task, s ticktick.TaskFocusSummary) string {
	planned, pomos, hasPlan := taskPlannedFocus(t)
	if !hasPlan {
		return renderTaskFocusDetail(s)
	}
	invested := int((s.TotalSeconds + 30) / 60)
	remaining := max(planned-invested, 0)
	progress := 0
	if planned > 0 {
		progress = min(invested*100/planned, 100)
	}
	return fmt.Sprintf(
		"focus %s / %s planned · %s left · %d%% · %d/%d pomos",
		planning.FormatMinutes(invested), planning.FormatMinutes(planned),
		planning.FormatMinutes(remaining), progress, completedTaskPomos(s), pomos,
	)
}
