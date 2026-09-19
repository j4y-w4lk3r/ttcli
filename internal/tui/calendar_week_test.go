package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/planning"
	"github.com/j4y-w4lk3r/ttcli/internal/taskcheckin"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
	"github.com/mattn/go-runewidth"
)

func TestWeekStartConfiguration(t *testing.T) {
	wednesday := time.Date(2026, 9, 16, 0, 0, 0, 0, time.Local)
	if got := weekStartFor(wednesday, true); got.Weekday() != time.Monday {
		t.Fatalf("Monday-first start=%s", got.Weekday())
	}
	if got := weekStartFor(wednesday, false); got.Weekday() != time.Sunday {
		t.Fatalf("Sunday-first start=%s", got.Weekday())
	}
}

func TestCalWeekAdaptiveRange(t *testing.T) {
	start := time.Date(2026, 9, 14, 0, 0, 0, 0, time.Local)
	now := start.AddDate(0, 0, 10).Add(12 * time.Hour)
	byHour := map[int][]calWeekRow{
		6:  {{kind: "task"}},
		21: {{kind: "task"}},
	}
	first, last := calWeekAdaptiveRange(byHour, start, now, planning.DefaultConfig())
	if first != 6 || last != 22 {
		t.Fatalf("range=%02d:00–%02d:00", first, last)
	}
}

func TestCalWeekBuildsStickyAllDayAndCapacityRows(t *testing.T) {
	start := weekStart(time.Now())
	day := dateKey(start)
	idx := buildCalIndex([]ticktick.Task{
		{ID: "all", Title: "All day", DueDate: day, IsAllDay: true},
		{ID: "timed", Title: "Timed", StartDate: day + "T09:00:00.000+0200", DueDate: day + "T10:00:00.000+0200"},
	})
	rows := buildCalWeekTimelineWithConfig(idx, start, time.Now(), planning.DefaultConfig())
	if len(rows) == 0 || rows[0].kind != "allday-label" {
		t.Fatalf("first row=%+v", rows)
	}
	foundAllDay, foundCapacity, foundTask := false, false, false
	for _, row := range rows {
		switch row.kind {
		case "allday":
			foundAllDay = true
		case "capacity":
			foundCapacity = true
		case "task":
			foundTask = true
		}
	}
	if !foundAllDay || !foundCapacity || !foundTask {
		t.Fatalf("allDay=%v capacity=%v task=%v", foundAllDay, foundCapacity, foundTask)
	}
}

func TestCalWeekPacksSameHourTasksAcrossDayColumns(t *testing.T) {
	start := weekStart(time.Now())
	var tasks []ticktick.Task
	for day := 0; day < 7; day++ {
		date := start.AddDate(0, 0, day).Format("2006-01-02")
		tasks = append(tasks, ticktick.Task{
			ID: fmt.Sprintf("wake-%d", day), Title: "Wake",
			StartDate: date + "T05:00:00.000+0200",
			DueDate:   date + "T05:25:00.000+0200",
		})
	}
	m := fixtureModel(250, 50)
	m.view, m.calMode, m.calDate = viewCalendar, calModeWeek, start
	m.calTasks = tasks
	output := stripANSI(m.renderCalWeekTimeline(m.calIdx(), m.layout()))
	wakeLines := 0
	for _, line := range strings.Split(output, "\n") {
		if strings.Contains(line, "Wake") {
			wakeLines++
			if count := strings.Count(line, "Wake"); count != 7 {
				t.Fatalf("packed Wake count=%d line=%q", count, line)
			}
		}
	}
	if wakeLines != 1 {
		t.Fatalf("Wake rendered on %d rows, want one packed row:\n%s", wakeLines, output)
	}
}

func TestCalWeekColumnBudgetAndRowBoundariesAlign(t *testing.T) {
	for _, fullW := range []int{76, 116, 196, 246} {
		colW, contentW := calWeekColumnWidths(fullW)
		total := 0
		minContent, maxContent := int(^uint(0)>>1), 0
		for dayIndex, width := range colW {
			total += width
			cellW := calWeekCellContentWidth(width, dayIndex)
			minContent = min(minContent, cellW)
			maxContent = max(maxContent, cellW)
		}
		if total != contentW {
			t.Fatalf("fullW=%d columns sum=%d contentW=%d", fullW, total, contentW)
		}
		if maxContent-minContent > 1 {
			t.Fatalf("fullW=%d uneven content widths=%v", fullW, colW)
		}
		var cells [7]string
		for day := range cells {
			cells[day] = fmt.Sprintf("day-%d", day)
		}
		row := renderCalWeekColumns(cells, colW)
		if lipgloss.Width(row) != contentW || runewidth.StringWidth(stripANSI(row)) != contentW {
			t.Fatalf("fullW=%d row widths lipgloss=%d runewidth=%d want=%d",
				fullW, lipgloss.Width(row), runewidth.StringWidth(stripANSI(row)), contentW)
		}
	}

	start := weekStart(time.Now())
	colW, contentW := calWeekColumnWidths(246)
	fullRowW := dayTimeColW + len(dayTimelineGap()) + contentW
	idx := buildCalIndex(nil)
	rows := []string{
		renderCalWeekDayHead(start, start, colW, contentW),
		renderCalWeekAllDayRow(calWeekRow{kind: "allday-label"}, false, colW, fullRowW),
		renderCalWeekSlotRow(9, start, idx, planning.DefaultConfig(), colW, fullRowW),
	}
	wantDivider := -1
	for _, row := range rows {
		plain := stripANSI(row)
		byteIndex := strings.Index(plain, "┊")
		divider := runewidth.StringWidth(plain[:byteIndex])
		if wantDivider < 0 {
			wantDivider = divider
		} else if divider != wantDivider {
			t.Fatalf("week boundary drift: got divider=%d want=%d row=%q", divider, wantDivider, plain)
		}
	}
}

func TestWeekSelectedTaskCanCheckIn(t *testing.T) {
	today := dateOnly(time.Now())
	task := ticktick.Task{
		ID: "week-task", ProjectID: "home", Title: "Week task",
		DueDate: dateKey(today), IsAllDay: true,
	}
	m := fixtureModel(100, 30)
	m.view = viewCalendar
	m.calMode = calModeWeek
	m.calDate = today
	m.calTasks = []ticktick.Task{task}
	m.checkinStore = taskcheckin.NewStoreAt(filepath.Join(t.TempDir(), "checkins.json"))
	rows := buildCalWeekTimelineWithConfig(
		m.calIdx(),
		weekStartFor(today, m.uiSettings.weekStartsMonday()),
		time.Now(),
		m.uiSettings.planningConfig(),
	)
	for i, row := range rows {
		if row.kind == "allday" && row.entry.Task.ID == task.ID {
			m.calGridCursor = i
			break
		}
	}
	out, cmd := m.updateCalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil || out.pendingCheckin == "" {
		t.Fatalf("pending=%q cmd=%v", out.pendingCheckin, cmd)
	}
}

func TestNarrowWeekCardsShowTopTasksAndMore(t *testing.T) {
	today := dateOnly(time.Now())
	var tasks []ticktick.Task
	for i, title := range []string{"Alpha", "Beta", "Gamma"} {
		tasks = append(tasks, ticktick.Task{
			ID: fmt.Sprint(i), Title: title, DueDate: dateKey(today), IsAllDay: true,
		})
	}
	m := fixtureModel(50, 30)
	m.calDate = today
	m.calTasks = tasks
	output := stripANSI(m.renderCalWeekList(m.calIdx(), m.layout()))
	if !strings.Contains(output, "Alpha") || !strings.Contains(output, "Beta") || !strings.Contains(output, "+1 more") {
		t.Fatalf("narrow cards:\n%s", output)
	}
}

func TestCalendarDayAndWeekFramesAtResponsiveWidths(t *testing.T) {
	today := dateOnly(time.Now())
	tasks := []ticktick.Task{
		{ID: "all", Title: "Groceries", DueDate: dateKey(today), IsAllDay: true},
		{
			ID: "timed", Title: "Deep work",
			StartDate: dateKey(today) + "T09:00:00.000+0200",
			DueDate:   dateKey(today) + "T10:30:00.000+0200",
		},
	}
	for _, width := range []int{50, 80, 120, 200, 250} {
		m := fixtureModel(width, 40)
		m.view = viewCalendar
		m.calDate = today
		m.calTasks = tasks
		for _, mode := range []calMode{calModeDay, calModeWeek} {
			m.calMode = mode
			assertViewOK(t, m, fmt.Sprintf("calendar mode=%d width=%d", mode, width))
		}
	}
}

func TestCalendarDayResponsiveCoachLayouts(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	today := dateOnly(time.Now())
	task := ticktick.Task{
		ID: "food", Title: "Food", DueDate: dateKey(today), IsAllDay: true,
		FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 4500, EstimatedPomo: 3}},
	}
	wide := fixtureModel(120, 40)
	wide.view, wide.calMode, wide.calDate = viewCalendar, calModeDay, today
	wide.calTasks = []ticktick.Task{task}
	wideView := stripANSI(wide.View())
	for _, want := range []string{"FOCUS PLAN", "Planned", "Remaining", "Pomos", "Next move", "Selected"} {
		if !strings.Contains(wideView, want) {
			t.Fatalf("wide day view missing %q", want)
		}
	}

	narrow := fixtureModel(50, 30)
	narrow.view, narrow.calMode, narrow.calDate = viewCalendar, calModeDay, today
	narrow.calTasks = []ticktick.Task{task}
	narrowView := stripANSI(narrow.View())
	if strings.Contains(narrowView, "FOCUS PLAN") || !strings.Contains(narrowView, "remaining") {
		t.Fatalf("unexpected narrow coach layout:\n%s", narrowView)
	}
}

func TestDayTaskBlocksScaleWithDuration(t *testing.T) {
	day := dateOnly(time.Now())
	rows := []calDayRow{
		{kind: "slot"},
		{
			kind: "task", entry: calEntry{Task: ticktick.Task{ID: "one", Title: "One hour"}},
			start: day.Add(9 * time.Hour), end: day.Add(10 * time.Hour),
			estimate: planning.TaskEstimate{Minutes: 60},
		},
		{kind: "slot"},
		{
			kind: "task", entry: calEntry{Task: ticktick.Task{ID: "three", Title: "Three hours"}},
			start: day.Add(10 * time.Hour), end: day.Add(13 * time.Hour),
			estimate: planning.TaskEstimate{Minutes: 180},
		},
		{kind: "slot"},
	}
	visual := buildCalDayVisualRows(rows, 12)
	oneRows, threeRows := 0, 0
	for _, row := range visual {
		switch row.row.entry.Task.ID {
		case "one":
			oneRows++
		case "three":
			threeRows++
		}
	}
	if oneRows == 0 || threeRows != oneRows*3 {
		t.Fatalf("one-hour rows=%d three-hour rows=%d", oneRows, threeRows)
	}
}

func TestWeekStretchTaskBlocksScaleWithDuration(t *testing.T) {
	day := dateOnly(time.Now())
	rows := []calWeekRow{
		{kind: "slot", hour: 9},
		{
			kind: "task", dayIndex: 0, start: day.Add(9 * time.Hour), end: day.Add(10 * time.Hour),
			entry: calEntry{Task: ticktick.Task{ID: "one"}}, estimate: planning.TaskEstimate{Minutes: 60},
		},
		{kind: "slot", hour: 10},
		{
			kind: "task", dayIndex: 1, start: day.Add(10 * time.Hour), end: day.Add(13 * time.Hour),
			entry: calEntry{Task: ticktick.Task{ID: "three"}}, estimate: planning.TaskEstimate{Minutes: 180},
		},
		{kind: "slot", hour: 11},
		{kind: "slot", hour: 12},
	}
	visual := buildCalWeekVisualBodyScaled(rows, 0, 3, day.Add(9*time.Hour))
	oneRows, threeRows := 0, 0
	for _, row := range visual {
		for _, cell := range row.cells {
			switch cell.row.entry.Task.ID {
			case "one":
				oneRows++
			case "three":
				threeRows++
			}
		}
	}
	if oneRows == 0 || threeRows != oneRows*3 {
		t.Fatalf("one-hour rows=%d three-hour rows=%d", oneRows, threeRows)
	}
}

func TestCalendarNowRulesAreProminent(t *testing.T) {
	dayLine := stripANSI(renderDayNowRow("12:34", 80))
	if !strings.Contains(dayLine, "● NOW") || strings.Count(dayLine, "━") < 20 {
		t.Fatalf("day now line=%q", dayLine)
	}
	colW, contentW := calWeekColumnWidths(120)
	weekLine := stripANSI(renderCalWeekNowRow(
		calWeekRow{kind: "now", dayIndex: 3}, time.Date(2026, 9, 18, 12, 34, 0, 0, time.Local),
		colW, dayTimeColW+len(dayTimelineGap())+contentW,
	))
	if !strings.Contains(weekLine, "● NOW") || strings.Count(weekLine, "━") < 20 {
		t.Fatalf("week now line=%q", weekLine)
	}
}

func TestCalendarDayTodayKeyCentersNowRow(t *testing.T) {
	m := fixtureModel(100, 30)
	m.view = viewCalendar
	m.calMode = calModeDay
	out, _ := m.updateCalKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !out.calDayCenterNow {
		t.Fatal("today key did not enable Day follow-now centering")
	}
	rows := []calDayVisualRow{
		{row: calDayRow{kind: "slot"}},
		{row: calDayRow{kind: "task"}},
		{row: calDayRow{kind: "slot"}},
		{row: calDayRow{kind: "now"}},
		{row: calDayRow{kind: "slot"}},
	}
	nowCursor := calDayNowVisualCursor(rows)
	window := computeScrollWindow(nowCursor, len(rows), 3)
	if nowCursor < window.Start || nowCursor >= window.End {
		t.Fatalf("now row %d outside centered window %+v", nowCursor, window)
	}
}

func TestCalWeekTimelineShowsLoggedFocus(t *testing.T) {
	day := time.Date(2026, 9, 14, 0, 0, 0, 0, time.Local)
	start := time.Date(2026, 9, 14, 6, 45, 0, 0, time.Local)
	end := time.Date(2026, 9, 14, 7, 10, 0, 0, time.Local)
	task := ticktick.Task{
		ID: "shower", Title: "Shower + Shave + Cream", DueDate: "2026-09-14", IsAllDay: true,
	}
	record := ticktick.FocusRecord{
		ID:        "focus-shower",
		StartTime: start.Format("2006-01-02T15:04:05.000-0700"),
		EndTime:   end.Format("2006-01-02T15:04:05.000-0700"),
	}
	record.SetTaskTitle("Shower + Shave + Cream")
	if len(record.Tasks) > 0 {
		record.Tasks[0].TaskID = "shower"
	}
	idx := buildCalIndex([]ticktick.Task{task})
	rows := buildCalWeekTimelineWithFocus(
		idx,
		weekStartFor(day, true),
		day.Add(12*time.Hour),
		planning.DefaultConfig(),
		map[string]*ticktick.FocusStats{
			dateKey(day): {Date: dateKey(day), Records: []ticktick.FocusRecord{record}},
		},
	)
	found := false
	for _, row := range rows {
		if row.logged && row.entry.Task.Title == "Shower + Shave + Cream" {
			found = true
			if row.start.Format("15:04") != "06:45" {
				t.Fatalf("logged start=%s", row.start.Format("15:04"))
			}
		}
	}
	if !found {
		t.Fatal("logged shower session missing from week timeline")
	}
}
