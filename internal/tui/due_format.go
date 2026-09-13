package tui

import (
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// dueColWidth is the visual width of the fixed due-date column (measured once).
var dueColWidth int

func init() {
	sample := dueOverStyle.Render(iconOverdueDot + " 31/12/2099 23:59")
	dueColWidth = lipgloss.Width(sample)
	if dueColWidth < 14 {
		dueColWidth = 17
	}
}

func parseDueDay(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	// TickTick all-day tasks use a plain YYYY-MM-DD (no timezone shift).
	if len(raw) == 10 && raw[4] == '-' && raw[7] == '-' && !strings.Contains(raw, "T") {
		d, err := time.ParseInLocation("2006-01-02", raw, time.Local)
		if err != nil {
			return time.Time{}, false
		}
		return dateOnly(d), true
	}
	if tm, ok := parseDueTime(raw); ok {
		return dateOnly(tm.In(time.Local)), true
	}
	return time.Time{}, false
}

func formatDueDMY(raw string) string {
	d, ok := parseDueDay(raw)
	if !ok {
		return ""
	}
	return d.Format("02/01/2006")
}

// dueColumn renders a fixed-width due date cell (legacy far-right column).
func dueColumn(raw string) string {
	return padDueWidth(dueInlineRaw(raw, false))
}

func dueLabel(raw string, allDay bool) string {
	dmy := formatDueDMY(raw)
	if dmy == "" {
		return ""
	}
	if allDay || !hasDueTime(raw) {
		return dmy
	}
	if clock := dueClockFromRaw(raw); clock != "" {
		return dmy + " " + clock
	}
	return dmy
}

func dueClockFromRaw(raw string) string {
	if len(raw) >= 16 && raw[10] == 'T' {
		return raw[11:16]
	}
	if tm, ok := parseDueTime(raw); ok {
		return tm.Format("15:04")
	}
	return ""
}

// dueInlineTask renders due date/time with overdue styling.
func dueInlineTask(t ticktick.Task) string {
	return dueInlineRaw(t.DueDate, t.IsAllDay)
}

// dueInlineRaw renders due date/time from API dueDate string.
func dueInlineRaw(raw string, allDay bool) string {
	label := dueLabel(raw, allDay)
	if label == "" {
		return lipgloss.NewStyle().Foreground(colorMuted).Render("—")
	}
	d, _ := parseDueDay(raw)
	today := dateOnly(time.Now())
	overdue := d.Before(today)
	if !overdue && hasDueTime(raw) && !allDay {
		if tm, ok := parseDueTime(raw); ok && tm.Before(time.Now()) {
			overdue = true
		}
	}
	if overdue {
		marker := dueOverStyle.Render(iconOverdueDot) + " "
		return padDueWidth(marker + dueOverStyle.Render(label))
	}
	marker := dueStyle.Render(iconOverdueDot) + " "
	return padDueWidth(marker + dueStyle.Render(label))
}

// dueInline is kept for callers that only have a raw dueDate string.
func dueInline(raw string) string {
	return dueInlineRaw(raw, false)
}

func padDueWidth(s string) string {
	w := lipgloss.Width(s)
	if w > dueColWidth {
		return truncateRenderedWidth(s, dueColWidth)
	}
	if w < dueColWidth {
		s += strings.Repeat(" ", dueColWidth-w)
	}
	return s
}

// formatDue is kept for any legacy callers; prefer dueColumn for aligned rows.
func formatDue(raw string) string {
	return strings.TrimSpace(dueColumn(raw))
}

func hasDueTime(raw string) bool {
	return len(raw) >= 16 && raw[10] == 'T'
}

func parseDueTime(raw string) (time.Time, bool) {
	if raw == "" {
		return time.Time{}, false
	}
	t, err := ticktick.ParseAPITime(raw)
	if err == nil {
		return t, true
	}
	if len(raw) >= 16 {
		t, err = time.ParseInLocation("2006-01-02T15:04", raw[:16], time.Local)
		if err == nil {
			return t, true
		}
	}
	if d, ok := parseDueDay(raw); ok {
		return d, true
	}
	return time.Time{}, false
}

func dueTaskClock(t ticktick.Task) string {
	if t.IsAllDay || !hasDueTime(t.DueDate) {
		return ""
	}
	if len(t.DueDate) >= 16 {
		return t.DueDate[11:16]
	}
	if tm, ok := parseDueTime(t.DueDate); ok {
		return tm.Format("15:04")
	}
	return ""
}

func dueSortKey(t ticktick.Task) (time.Time, bool) {
	if t.IsAllDay || !hasDueTime(t.DueDate) {
		d, ok := parseDueDay(t.DueDate)
		return d, ok
	}
	tm, ok := parseDueTime(t.DueDate)
	return tm, ok
}

func sortTasksByDue(tasks []ticktick.Task) {
	sort.Slice(tasks, func(i, j int) bool {
		return compareTasksByDue(tasks[i], tasks[j])
	})
}

func compareTasksByDue(a, b ticktick.Task) bool {
	ta, aHas := dueSortKey(a)
	tb, bHas := dueSortKey(b)
	if aHas != bHas {
		return aHas
	}
	if !aHas {
		if a.SortOrder != b.SortOrder {
			return a.SortOrder < b.SortOrder
		}
		return a.Title < b.Title
	}
	if !ta.Equal(tb) {
		return ta.Before(tb)
	}
	if a.Title != b.Title {
		return a.Title < b.Title
	}
	return a.ID < b.ID
}

func sortTasksForProject(tasks []ticktick.Task, mode TaskSortMode) {
	sortTasks(tasks, mode)
}

func splitAllDayTasks(tasks []ticktick.Task) (allDay, timed []ticktick.Task) {
	for _, t := range tasks {
		if t.IsAllDay || !hasDueTime(t.DueDate) {
			allDay = append(allDay, t)
		} else {
			timed = append(timed, t)
		}
	}
	return allDay, timed
}

func calTaskShouldBeDone(t ticktick.Task, day time.Time, now time.Time) bool {
	dueDay, ok := parseDueDay(t.DueDate)
	if !ok {
		return false
	}
	today := dateOnly(now)
	if dueDay.Before(today) {
		return true
	}
	if dateKey(dueDay) != dateKey(day) {
		return false
	}
	if t.IsAllDay || !hasDueTime(t.DueDate) {
		return false
	}
	tm, ok := parseDueTime(t.DueDate)
	return ok && tm.Before(now)
}
