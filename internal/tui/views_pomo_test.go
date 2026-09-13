package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func taskLink(title string) []struct {
	TaskID      string `json:"taskId"`
	Title       string `json:"title"`
	ProjectName string `json:"projectName"`
	StartTime   string `json:"startTime"`
	EndTime     string `json:"endTime"`
} {
	return []struct {
		TaskID      string `json:"taskId"`
		Title       string `json:"title"`
		ProjectName string `json:"projectName"`
		StartTime   string `json:"startTime"`
		EndTime     string `json:"endTime"`
	}{{Title: title}}
}

func TestFocusRecordClockUsesLocalTime(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	rec := ticktick.FocusRecord{
		StartTime: "2026-08-31T14:15:00.000+0000",
		EndTime:   "2026-08-31T14:16:00.000+0000",
	}
	if got := focusRecordClock(rec); got != "16:15" {
		t.Fatalf("start clock=%q want 16:15 (CEST)", got)
	}
	if got := focusRecordEndClock(rec); got != "16:16" {
		t.Fatalf("end clock=%q want 16:16 (CEST)", got)
	}
}

func TestBuildPomoHourGridFullDay(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 47, 0, 0, time.Local)
	records := []ticktick.FocusRecord{
		{StartTime: "2026-08-30T08:37:00.000+0200", EndTime: "2026-08-30T08:42:00.000+0200", Tasks: taskLink("morning")},
	}
	grid := buildPomoHourGrid(records, nil, now, nil, true)

	var slotLabels []string
	var nowIdx, lastPomoBeforeNow int
	nowIdx = -1
	lastPomoBeforeNow = -1
	for i, row := range grid {
		if row.kind == "slot" {
			slotLabels = append(slotLabels, row.hourLabel)
		}
		if row.kind == "now" {
			nowIdx = i
		}
		if row.kind == "pomo" && row.hour == 15 && nowIdx < 0 {
			lastPomoBeforeNow = i
		}
	}
	if len(slotLabels) != 24 {
		t.Fatalf("expected 24 hour slot rails, got %d: %v", len(slotLabels), slotLabels)
	}
	if slotLabels[0] != "00:00" || slotLabels[23] != "23:00" {
		t.Fatalf("unexpected hour labels: first=%q last=%q", slotLabels[0], slotLabels[23])
	}

	foundPomo := false
	foundNow := false
	for _, row := range grid {
		if row.kind == "pomo" {
			foundPomo = true
		}
		if row.kind == "now" {
			foundNow = true
			if row.hourLabel != "15:47" {
				t.Fatalf("now row clock=%q want 15:47", row.hourLabel)
			}
		}
	}
	if !foundPomo || !foundNow {
		t.Fatalf("pomo=%v now=%v", foundPomo, foundNow)
	}
	if nowIdx >= 0 && lastPomoBeforeNow >= 0 && nowIdx <= lastPomoBeforeNow {
		t.Fatalf("now row should follow earlier sessions in the hour")
	}
}

func TestNowRowAfterEarlierSessionsInHour(t *testing.T) {
	now := time.Date(2026, 8, 31, 16, 41, 0, 0, time.Local)
	records := []ticktick.FocusRecord{
		{StartTime: "2026-08-31T16:12:00.000+0200", EndTime: "2026-08-31T16:14:00.000+0200", Tasks: taskLink("a")},
		{StartTime: "2026-08-31T16:15:00.000+0200", EndTime: "2026-08-31T16:16:00.000+0200", Tasks: taskLink("b")},
	}
	grid := buildPomoHourGrid(records, nil, now, nil, true)

	var kinds []string
	for _, row := range grid {
		if row.hour != 16 {
			continue
		}
		switch row.kind {
		case "slot", "pomo", "now":
			kinds = append(kinds, row.kind)
		}
	}
	want := []string{"slot", "pomo", "pomo", "now"}
	if len(kinds) != len(want) {
		t.Fatalf("hour 16 rows=%v want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("hour 16 rows=%v want %v", kinds, want)
		}
	}
}

func TestRenderDayGridRowNowShowsCurrentClock(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 47, 0, 0, time.Local)
	row := dayGridRow{kind: "now", hourLabel: "15:47", hour: 15}
	line := renderDayGridRow(row, now, false, 80, nil, nil, pomoTimelineLayout{})
	plain := stripANSI(line)
	if !strings.HasPrefix(strings.TrimLeft(plain, " "), "15:47") {
		t.Fatalf("now row should show current clock in time column: %q", plain)
	}
	if !strings.Contains(plain, "● now") {
		t.Fatalf("now row should include now marker: %q", plain)
	}
	if strings.Contains(plain, "15:00") {
		t.Fatalf("now row should not floor to hour label: %q", plain)
	}
}

func TestRenderHourGridPomoSuffixColumnsAlign(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 47, 0, 0, time.Local)
	recs := []ticktick.FocusRecord{
		{StartTime: "2026-08-30T08:56:00.000+0200", EndTime: "2026-08-30T09:01:00.000+0200", Tasks: taskLink("CLI")},
		{StartTime: "2026-08-30T10:30:00.000+0200", EndTime: "2026-08-30T10:34:00.000+0200", Tasks: taskLink("new task1")},
	}
	colors := map[string]lipgloss.Color{"CLI": colorPeach, "new task1": colorTeal}
	width := 80
	grid := buildDayGrid(recs, nil, now, nil, true)
	layout := computePomoTimelineLayout(grid, dayTimelineContentW(width), colors, nil, now)
	var arrowCols []int
	for i, rec := range recs {
		row := dayGridRow{kind: "pomo", recIdx: i, rec: rec, inHourGrid: true, pomoSlotIndex: 0, pomoSlotCount: 1}
		plain := stripANSI(renderDayGridRow(row, now, false, width, colors, nil, layout))
		idx := strings.Index(plain, "▶")
		if idx < 0 {
			t.Fatalf("expected end marker: %q", plain)
		}
		arrowCols = append(arrowCols, idx)
	}
	if arrowCols[0] != arrowCols[1] {
		t.Fatalf("suffix columns misaligned: %v", arrowCols)
	}
	if arrowCols[0] > 50 {
		t.Fatalf("suffix too far right: col=%d", arrowCols[0])
	}
}

func TestRenderHourGridPomoBlockInHourRow(t *testing.T) {
	now := time.Date(2026, 8, 30, 15, 47, 0, 0, time.Local)
	rec := ticktick.FocusRecord{
		StartTime: "2026-08-30T08:56:00.000+0200",
		EndTime:   "2026-08-30T09:01:00.000+0200",
		Tasks:     taskLink("CLI"),
	}
	row := dayGridRow{kind: "pomo", rec: rec, inHourGrid: true, pomoSlotIndex: 0, pomoSlotCount: 1}
	colors := map[string]lipgloss.Color{"CLI": colorPeach}
	line := stripANSI(renderDayGridRow(row, now, false, 80, colors, nil, pomoTimelineLayout{}))
	if !strings.Contains(line, "▮") {
		t.Fatalf("expected duration bar: %q", line)
	}
	if strings.HasPrefix(strings.TrimLeft(line, " "), "08:56") {
		t.Fatalf("start time should be in block area, not time column: %q", line)
	}
	if !strings.Contains(line, "08:56") {
		t.Fatalf("expected start time in row: %q", line)
	}
	if !strings.Contains(line, "▌") {
		t.Fatalf("expected colored block marker: %q", line)
	}
	if !strings.Contains(line, "09:01") {
		t.Fatalf("expected end time hint: %q", line)
	}
}

func TestRenderHourSlotRowFullWidth(t *testing.T) {
	busy := renderHourSlotRow("08:00", true, 80, false)
	empty := renderHourSlotRow("10:00", false, 80, false)
	if lipgloss.Width(busy) < 70 {
		t.Fatalf("busy slot should span width: %d", lipgloss.Width(busy))
	}
	if lipgloss.Width(empty) < 70 {
		t.Fatalf("empty slot should span width: %d", lipgloss.Width(empty))
	}
	plain := stripANSI(empty)
	if !strings.Contains(plain, "─") {
		t.Fatalf("empty slot should use hour rail: %q", previewLine(empty, 80))
	}
	if !strings.HasPrefix(strings.TrimLeft(plain, " "), "10:00") {
		t.Fatalf("hour label should be in time column: %q", plain)
	}
}

func TestTaskDetailPanel(t *testing.T) {
	task := ticktick.Task{
		Content:  "notes here",
		Priority: 3,
		DueDate:  "2026-08-30",
		Items: []ticktick.ChecklistItem{
			{Title: "sub one", Status: 0},
			{Title: "sub two", Status: 1},
		},
	}
	lines := taskDetailPanel(task, noopTaskFocus, 60, 12)
	if len(lines) < 3 {
		t.Fatalf("expected detail lines, got %d", len(lines))
	}
}

func TestPomoTimelineGridCursorFollowsNow(t *testing.T) {
	now := time.Date(2026, 9, 7, 9, 11, 0, 0, time.Local)
	grid := buildPomoHourGrid(nil, nil, now, nil, true)
	nowIdx := gridRowForNow(grid)
	if nowIdx < 0 {
		t.Fatal("expected now row in grid")
	}

	m := model{pomoFollowNow: true, pomoViewDate: dateOnly(now), pomoNowTick: now}
	if got := m.pomoTimelineGridCursor(grid); got != nowIdx {
		t.Fatalf("follow now cursor=%d want %d", got, nowIdx)
	}

	win := computeScrollWindow(nowIdx, len(grid), 8)
	if win.Cursor != nowIdx {
		t.Fatalf("scroll cursor=%d want %d", win.Cursor, nowIdx)
	}
	if nowIdx < win.Start || nowIdx >= win.End {
		t.Fatalf("now row %d outside window [%d,%d)", nowIdx, win.Start, win.End)
	}
	mid := win.Start + (win.End-win.Start)/2
	if abs(nowIdx-mid) > 1 {
		t.Fatalf("now row %d not near window center %d (window %d..%d)", nowIdx, mid, win.Start, win.End)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
