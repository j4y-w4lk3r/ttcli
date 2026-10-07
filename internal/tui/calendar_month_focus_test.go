package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
	"github.com/mattn/go-runewidth"
)

func TestCalendarMonthIsMondayFirst(t *testing.T) {
	lay := computeCalMonthLayout(140, 40, 4)
	fields := strings.Fields(stripANSI(renderCalMonthDOWHeader(lay)))
	want := []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"}
	if len(fields) < len(want) {
		t.Fatalf("headers=%v", fields)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Fatalf("headers=%v want=%v", fields[:7], want)
		}
	}
	first := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.Local)
	if offset := (int(first.Weekday()) + 6) % 7; offset != 1 {
		t.Fatalf("September 2026 offset=%d want Tuesday in column 2", offset)
	}
}

func TestFocusBadgeWidthIncludesPadding(t *testing.T) {
	withTrueColor(t)
	label := fmt.Sprintf("%s %d/%d · %dm", iconPomodoro, 18, 30, 452)
	rendered := pomoTodayStyle(18, 30).Render(label)
	if lipgloss.Width(rendered) != runewidth.StringWidth(stripANSI(rendered)) {
		t.Fatalf("lipgloss=%d runewidth=%d plain=%q", lipgloss.Width(rendered), runewidth.StringWidth(stripANSI(rendered)), stripANSI(rendered))
	}
	clipped := truncateInner(rendered, 18)
	if lipgloss.Width(clipped) > 18 || runewidth.StringWidth(stripANSI(clipped)) > 18 {
		t.Fatalf("clipped lipgloss=%d runewidth=%d %q", lipgloss.Width(clipped), runewidth.StringWidth(stripANSI(clipped)), stripANSI(clipped))
	}
	day := calMonthDaySelStyle.Render("22")
	row := alignRightInWidth("", day, 12)
	if lipgloss.Width(row) > 12 || runewidth.StringWidth(stripANSI(row)) > 12 {
		t.Fatalf("day row lipgloss=%d runewidth=%d %q", lipgloss.Width(row), runewidth.StringWidth(stripANSI(row)), stripANSI(row))
	}
}

func TestMonthCellLinesNeverEmbedNewlines(t *testing.T) {
	withTrueColor(t)
	day := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.Local)
	tasks := []ticktick.Task{
		{ID: "a", Title: "wake up", DueDate: "2026-09-22T05:00:00.000+0200"},
		{ID: "b", Title: "Keyboard Typing (TypingClub)", DueDate: "2026-09-22T06:00:00.000+0200"},
		{ID: "c", Title: "Bed", DueDate: "2026-09-22T21:00:00.000+0200"},
		{ID: "d", Title: "line\nbreak", DueDate: "2026-09-22T13:00:00.000+0200"},
	}
	idx := buildCalendarIndexForRange(tasks, nil, nil, day.AddDate(0, 0, -10), day.AddDate(0, 0, 10))
	stats := &ticktick.FocusStats{FullPomoCount: 4, TotalSeconds: 100 * 60, Records: []ticktick.FocusRecord{{
		StartTime: "2026-09-22T10:00:00.000+0200",
		EndTime:   "2026-09-22T10:25:00.000+0200",
	}}}
	for _, innerH := range []int{3, 4, 5, 6, 8} {
		for colW := 8; colW <= 40; colW++ {
			cell := renderCalMonthCell(idx, day, colW, innerH, day, day, stats, 30)
			if len(cell) != innerH+2 {
				t.Fatalf("colW=%d innerH=%d lines=%d", colW, innerH, len(cell))
			}
			for i, line := range cell {
				if strings.Contains(line, "\n") || strings.Contains(line, "\r") {
					t.Fatalf("embedded newline colW=%d innerH=%d line=%d", colW, innerH, i)
				}
				if lipgloss.Width(line) != colW {
					t.Fatalf("colW=%d innerH=%d line=%d width=%d", colW, innerH, i, lipgloss.Width(line))
				}
			}
		}
	}
}

func TestCalendarMonthSelectedDayRowStaysSingleWidth(t *testing.T) {
	withTrueColor(t)
	day := time.Date(2026, time.September, 22, 0, 0, 0, 0, time.Local)
	tasks := []ticktick.Task{
		{ID: "a", Title: "wake up", DueDate: "2026-09-22T05:00:00.000+0200"},
		{ID: "b", Title: "Keyboard Typing (TypingClub)", DueDate: "2026-09-22T06:00:00.000+0200"},
		{ID: "c", Title: "Bed", DueDate: "2026-09-22T21:00:00.000+0200"},
		{ID: "d", Title: "get package", DueDate: "2026-09-22T13:15:00.000+0200"},
	}
	for _, size := range []struct{ w, h int }{{200, 48}, {220, 52}, {240, 55}, {180, 44}, {260, 60}, {120, 40}} {
		m := fixtureModel(size.w, size.h)
		m.view = viewCalendar
		m.calMode = calModeMonth
		m.calDate = day
		m.calTasks = tasks
		for _, hints := range []bool{false, true} {
			m.showKeyHints = hints
			view := m.View()
			lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
			if len(lines) != size.h {
				t.Fatalf("%dx%d hints=%v lines=%d", size.w, size.h, hints, len(lines))
			}
			var prevBottom bool
			for i, line := range lines {
				plain := stripANSI(line)
				wantW := viewDrawWidth(size.w)
				if lipgloss.Width(line) != wantW || ansi.StringWidth(line) != wantW || runewidth.StringWidth(plain) != wantW {
					t.Fatalf("%dx%d line %d lipgloss=%d ansi=%d runes=%d want %d %q", size.w, size.h, i, lipgloss.Width(line), ansi.StringWidth(line), runewidth.StringWidth(plain), wantW, previewLine(plain, 40))
				}
				bottom := strings.Count(plain, "└") >= 6
				if bottom && prevBottom {
					t.Fatalf("%dx%d hints=%v duplicate bottoms at %d", size.w, size.h, hints, i)
				}
				prevBottom = bottom
			}
			m.calMode = calModeWeek
			week := m.View()
			if strings.Contains(stripANSI(week), "└") {
				t.Fatalf("%dx%d hints=%v month bottoms leaked into week view", size.w, size.h, hints)
			}
			m.calMode = calModeMonth
		}
	}
}

func TestCalendarMonthCellShowsPomoGoalProgress(t *testing.T) {
	day := time.Date(2026, time.September, 18, 0, 0, 0, 0, time.Local)
	lines := renderCalMonthCell(
		buildCalIndex(nil), day, 28, 6, day, day,
		&ticktick.FocusStats{FullPomoCount: 3, TotalSeconds: 75 * 60},
		30,
	)
	output := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(output, "3/30") || !strings.Contains(output, "75m") {
		t.Fatalf("month focus cell:\n%s", output)
	}
	if len(lines) != 8 {
		t.Fatalf("cell lines=%d want 8", len(lines))
	}
}

func TestMonthCellShowsFocusWithoutTasks(t *testing.T) {
	withTrueColor(t)
	day := time.Date(2026, time.September, 22, 0, 0, 0, 0, time.Local)
	other := day.AddDate(0, 0, 1)
	tasks := []ticktick.Task{{
		ID: "a", Title: "wake up", DueDate: "2026-09-22T05:00:00.000+0200",
	}}
	idx := buildCalendarIndexForRange(tasks, nil, nil, day.AddDate(0, 0, -2), day.AddDate(0, 0, 2))
	stats := &ticktick.FocusStats{
		FullPomoCount: 4,
		Records: []ticktick.FocusRecord{{
			StartTime: "2026-09-22T10:00:00.000+0200",
			EndTime:   "2026-09-22T10:25:00.000+0200",
		}},
	}
	lines := renderCalMonthCell(idx, day, 28, 6, other, other.AddDate(0, 0, 1), stats, 30)
	output := stripANSI(strings.Join(lines, "\n"))
	if strings.Contains(output, "wake up") || strings.Contains(output, "more") {
		t.Fatalf("tasks leaked into month cell:\n%s", output)
	}
	if !strings.Contains(output, "22") || !strings.Contains(output, "4/30") || !strings.Contains(output, "25m") {
		t.Fatalf("month cell:\n%s", output)
	}
	inLight := frameLightness(lines[0])
	outLight := frameLightness(renderCalMonthEmptyCell(28, 6)[0])
	if inLight <= outLight {
		t.Fatalf("in-month frame lightness %d, padding %d", inLight, outLight)
	}
}

func frameLightness(s string) int {
	seq := foregroundSeq(s)
	var r, g, b int
	if _, err := fmt.Sscanf(seq, "38;2;%d;%d;%d", &r, &g, &b); err != nil {
		return 0
	}
	return r + g + b
}

func foregroundSeq(s string) string {
	i := strings.Index(s, "38;2;")
	if i < 0 {
		return ""
	}
	rest := s[i:]
	end := strings.IndexByte(rest, 'm')
	if end < 0 {
		return rest
	}
	return rest[:end]
}

func TestCalendarMonthCellExcludesUnclaimedMinutes(t *testing.T) {
	day := time.Date(2026, time.September, 22, 0, 0, 0, 0, time.Local)
	claimed := ticktick.FocusRecord{
		StartTime: "2026-09-22T10:00:00.000+0200",
		EndTime:   "2026-09-22T10:25:00.000+0200",
	}
	claimed.SetTaskTitle("write")
	unclaimed := ticktick.FocusRecord{
		StartTime: "2026-09-22T10:26:00.000+0200",
		EndTime:   "2026-09-22T18:00:00.000+0200",
	}
	unclaimed.SetTaskTitle(ticktick.UnclaimedTitlePrefix + "write")
	stats := &ticktick.FocusStats{
		FullPomoCount: 4,
		TotalSeconds:  475 * 60,
		Records:       []ticktick.FocusRecord{claimed, unclaimed},
	}
	lines := renderCalMonthCell(buildCalIndex(nil), day, 28, 6, day, day, stats, 30)
	output := stripANSI(strings.Join(lines, "\n"))
	if !strings.Contains(output, "4/30") || !strings.Contains(output, "25m") {
		t.Fatalf("month focus cell:\n%s", output)
	}
	if strings.Contains(output, "475m") {
		t.Fatalf("unclaimed minutes leaked into month cell:\n%s", output)
	}
}

func TestCalendarYearDayMarkerCarriesPomoCount(t *testing.T) {
	day := time.Date(2026, time.September, 18, 0, 0, 0, 0, time.Local)
	line := stripANSI(calYearDayCell(
		buildCalIndex(nil), day, day.AddDate(0, 0, 1), day.AddDate(0, 0, 2), 5,
		map[string]*ticktick.FocusStats{dateKey(day): {FullPomoCount: 3}},
	))
	if !strings.Contains(line, "3") {
		t.Fatalf("year focus marker=%q", line)
	}
}
