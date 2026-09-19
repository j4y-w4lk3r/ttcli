package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
)

const calDaySplitMinWidth = 92

var calDayBorderStyle = lipgloss.NewStyle().Foreground(colorOverlay)

type calDayVisualRow struct {
	row          calDayRow
	sourceIndex  int
	continuation int
	span         int
}

func calDayRowsPerHour(rows []calDayRow, maxRows int) int {
	hours := 0
	for _, row := range rows {
		if row.kind == "slot" {
			hours++
		}
	}
	if hours == 0 {
		return 1
	}
	return clamp(maxRows/hours, 1, 4)
}

func buildCalDayVisualRows(rows []calDayRow, maxRows int) []calDayVisualRow {
	rowsPerHour := calDayRowsPerHour(rows, maxRows)
	visual := make([]calDayVisualRow, 0, len(rows))
	for sourceIndex, row := range rows {
		span := 1
		if row.kind == "task" {
			minutes := max(row.estimate.Minutes, 1)
			if !row.end.IsZero() && row.end.After(row.start) {
				minutes = max(int(row.end.Sub(row.start).Minutes()+0.5), 1)
			}
			span = max((minutes*rowsPerHour+59)/60, 1)
			span = min(span, 16)
		}
		visual = append(visual, calDayVisualRow{row: row, sourceIndex: sourceIndex, span: span})
		for continuation := 1; continuation < span; continuation++ {
			visual = append(visual, calDayVisualRow{
				row: row, sourceIndex: sourceIndex, continuation: continuation, span: span,
			})
		}
	}
	return visual
}

func calDayVisualCursor(rows []calDayVisualRow, sourceCursor int) int {
	for i, row := range rows {
		if row.sourceIndex == sourceCursor {
			return i
		}
	}
	return 0
}

func calDayNowVisualCursor(rows []calDayVisualRow) int {
	for i, row := range rows {
		if row.row.kind == "now" {
			return i
		}
	}
	return -1
}

func selectedCalDayRow(rows []calDayRow, cursor int) (calDayRow, bool) {
	if len(rows) == 0 {
		return calDayRow{}, false
	}
	cursor = normalizeCalDayGridCursor(rows, cursor)
	if !calDayRowSelectable(rows[cursor]) {
		return calDayRow{}, false
	}
	return rows[cursor], true
}

func selectedCalDayEntry(rows []calDayRow, cursor int) (calEntry, bool) {
	row, ok := selectedCalDayRow(rows, cursor)
	if !ok {
		return calEntry{}, false
	}
	return row.entry, true
}

func renderCalDayAgenda(
	m model,
	rows []calDayRow,
	day, now time.Time,
	width, maxRows int,
) string {
	if width < 1 {
		return ""
	}
	if len(rows) == 0 {
		return hintStyle.Render("  (no tasks or check-ins)")
	}
	if maxRows < 4 {
		maxRows = 4
	}
	cursor := normalizeCalDayGridCursor(rows, m.calGridCursor)
	visual := buildCalDayVisualRows(rows, maxRows)
	visualCursor := calDayVisualCursor(visual, cursor)
	if m.calDayCenterNow && dateKey(day) == dateKey(now) {
		if nowCursor := calDayNowVisualCursor(visual); nowCursor >= 0 {
			visualCursor = nowCursor
		}
	}
	hintBudget := 0
	window := computeScrollWindow(visualCursor, len(visual), maxRows)
	if hint := m.scrollHint(window); hint != "" {
		hintBudget = 1
		window = computeScrollWindow(visualCursor, len(visual), max(maxRows-1, 1))
	}
	lines := make([]string, 0, maxRows)
	if hintBudget == 1 {
		lines = append(lines, truncateInner(m.scrollHint(window), width))
	}
	for i := window.Start; i < window.End && len(lines) < maxRows; i++ {
		visualRow := visual[i]
		row := visualRow.row
		selected := visualRow.sourceIndex == cursor && calDayRowSelectable(row)
		line := ""
		if visualRow.continuation > 0 {
			line = renderCalDayTaskContinuation(
				row, selected, visualRow.continuation == visualRow.span-1, width,
			)
		} else {
			line = renderCalDayGridRow(row, day, now, selected, width)
		}
		lines = append(lines, padToWidth(truncateRenderedWidth(line, width), width))
	}
	return padBlockToSize(strings.Join(lines, "\n"), maxRows, width)
}

func calCoachStatus(plan planning.DayPlan) string {
	label := strings.ToUpper(string(plan.Feasibility))
	if label == "" {
		label = "HISTORICAL"
	}
	style := lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	switch plan.Feasibility {
	case planning.FeasibilityTight:
		style = lipgloss.NewStyle().Foreground(colorPeach).Bold(true)
	case planning.FeasibilityOverCapacity:
		style = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	case planning.FeasibilityClear:
		label = "PLAN CLEAR"
	}
	return style.Render(label)
}

func calCoachPace(plan planning.DayPlan) string {
	if plan.Historical {
		return "—"
	}
	switch {
	case plan.PaceDeltaMinutes > 0:
		return planning.FormatMinutes(plan.PaceDeltaMinutes) + " ahead"
	case plan.PaceDeltaMinutes < 0:
		return planning.FormatMinutes(-plan.PaceDeltaMinutes) + " behind"
	default:
		return "on pace"
	}
}

func calCoachSlack(plan planning.DayPlan) string {
	if plan.Historical {
		return "—"
	}
	if plan.SlackMinutes < 0 {
		return "−" + planning.FormatMinutes(-plan.SlackMinutes)
	}
	return "+" + planning.FormatMinutes(plan.SlackMinutes)
}

func calCoachCapacityBar(plan planning.DayPlan, width int) string {
	width = max(width, 6)
	ratio := 0.0
	if plan.AvailableMinutes > 0 {
		ratio = float64(plan.RemainingMinutes) / float64(plan.AvailableMinutes)
	} else if plan.RemainingMinutes > 0 {
		ratio = 1
	}
	filled := int(ratio*float64(width) + 0.5)
	filled = min(max(filled, 0), width)
	fillStyle := lipgloss.NewStyle().Foreground(colorGreen)
	if plan.Feasibility == planning.FeasibilityTight {
		fillStyle = lipgloss.NewStyle().Foreground(colorPeach)
	} else if plan.Feasibility == planning.FeasibilityOverCapacity {
		fillStyle = lipgloss.NewStyle().Foreground(colorRed)
	}
	return fillStyle.Render(strings.Repeat("━", filled)) +
		hintStyle.Render(strings.Repeat("─", width-filled))
}

func calCoachSelectedLines(entry calEntry, config planning.Config) []string {
	return calCoachSelectedRowLines(calDayRow{entry: entry, estimate: planning.EstimateTask(entry.Task, config)}, config)
}

func calCoachSelectedRowLines(row calDayRow, config planning.Config) []string {
	entry := row.entry
	estimate, start, end := row.estimate, row.start, row.end
	if start.IsZero() && end.IsZero() {
		estimate, start, end = calTaskSchedule(entry, config)
	} else if estimate.Minutes == 0 {
		estimate = planning.EstimateTask(entry.Task, config)
	}
	title := strings.TrimSpace(entry.Task.Title)
	if title == "" {
		title = "(untitled)"
	}
	status := "open"
	if entry.Done() {
		status = "done"
	}
	if row.logged || estimate.Source == planning.EstimateLogged {
		status = "logged"
	}
	schedule := "all day"
	if !start.IsZero() {
		schedule = start.Format("15:04")
		if !end.IsZero() {
			schedule += "–" + end.Format("15:04")
		}
	}
	return []string{
		sectionHeader("Selected", 24),
		listSelStyle.Render(title),
		schedule + " · " + status,
		fmt.Sprintf("%s focus · %d pomos · %s",
			planning.FormatMinutes(estimate.Minutes), estimate.Pomos, estimate.Source),
	}
}

func wrapCalCoachText(text string, width int) []string {
	width = max(width, 8)
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if lipgloss.Width(lines[last]+" "+word) <= width {
			lines[last] += " " + word
			continue
		}
		lines = append(lines, word)
	}
	return lines
}

func renderCalDayCoachBox(
	plan planning.DayPlan,
	selected *calDayRow,
	hiddenOverdue, width, height int,
	config planning.Config,
) string {
	width = max(width, 28)
	height = max(height, 16)
	inner := width - 2
	topLabel := " FOCUS PLAN "
	top := "╭─" + topLabel + strings.Repeat("─", max(inner-lipgloss.Width(topLabel)-1, 0)) + "╮"
	lines := []string{
		calCoachStatus(plan) + "  " + hintStyle.Render(string(plan.Evidence)+" evidence"),
		"",
		sectionHeader("Progress", inner),
		fmt.Sprintf("Planned    %s", planning.FormatMinutes(plan.PlannedMinutes)),
		fmt.Sprintf("Logged     %s", planning.FormatMinutes(plan.LoggedMinutes)),
		fmt.Sprintf("Remaining  %s", planning.FormatMinutes(plan.RemainingMinutes)),
		fmt.Sprintf("Pomos      %d / %d", plan.CompletedPomos, plan.PlannedPomos),
		"",
		sectionHeader("Capacity", inner),
		calCoachCapacityBar(plan, max(inner-2, 6)),
		fmt.Sprintf("Available  %s", planning.FormatMinutes(plan.AvailableMinutes)),
		fmt.Sprintf("Slack      %s", calCoachSlack(plan)),
		fmt.Sprintf("Pace       %s", calCoachPace(plan)),
		"",
		sectionHeader("Next move", inner),
	}
	lines = append(lines, wrapCalCoachText(plan.Guidance, inner)...)
	if hiddenOverdue > 0 {
		lines = append(lines,
			"",
			hintStyle.Render(fmt.Sprintf("%d overdue hidden · o to show", hiddenOverdue)),
		)
	}
	if selected != nil {
		lines = append(lines, "")
		lines = append(lines, calCoachSelectedRowLines(*selected, config)...)
	}
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	if len(lines) > height-2 {
		lines = lines[:height-2]
	}
	out := []string{calDayBorderStyle.Render(truncateRenderedWidth(top, width))}
	for _, line := range lines {
		line = truncateRenderedWidth(line, inner)
		out = append(out,
			calDayBorderStyle.Render("│")+padToWidth(line, inner)+calDayBorderStyle.Render("│"),
		)
	}
	out = append(out, calDayBorderStyle.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(out, "\n")
}

func renderCalDayCoachCompact(plan planning.DayPlan, hiddenOverdue, width int) string {
	first := calCoachStatus(plan) +
		hintStyle.Render(fmt.Sprintf(
			" · %s remaining · %d/%d pomos · %s",
			planning.FormatMinutes(plan.RemainingMinutes),
			plan.CompletedPomos, plan.PlannedPomos,
			calCoachPace(plan),
		))
	lines := []string{
		truncateInner(first, width),
		truncateInner(hintStyle.Render(plan.Guidance), width),
	}
	if hiddenOverdue > 0 {
		lines = append(lines,
			truncateInner(hintStyle.Render(fmt.Sprintf("%d overdue hidden · o to show", hiddenOverdue)), width),
		)
	}
	return strings.Join(lines, "\n")
}
