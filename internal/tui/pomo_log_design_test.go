package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestPomoLogDesignCycle(t *testing.T) {
	design := PomoLogRails
	seen := map[PomoLogDesign]bool{}
	for i := 0; i < 5; i++ {
		seen[design] = true
		design = design.Next()
	}
	if design != PomoLogRails {
		t.Fatalf("cycle returned %q", design)
	}
	for _, want := range []PomoLogDesign{PomoLogRails, PomoLogSessions, PomoLogHours, PomoLogCards, PomoLogTape} {
		if !seen[want] {
			t.Fatalf("missing %q", want)
		}
	}
	if normalizePomoLogDesign("nope") != PomoLogRails {
		t.Fatal("unknown design should stay on rails")
	}
}

func TestPomoLogListDropsEmptyHours(t *testing.T) {
	now := time.Date(2026, 10, 9, 16, 20, 0, 0, time.Local)
	records := []ticktick.FocusRecord{
		{StartTime: "2026-10-09T16:14:00.000+0200", EndTime: "2026-10-09T16:41:00.000+0200", Tasks: taskLink("washing")},
	}
	grid := buildPomoHourGrid(records, nil, now, nil, true)
	list := buildPomoLogList(grid, now, true, false)
	var hours []int
	var kinds []string
	for _, row := range list {
		kinds = append(kinds, row.kind)
		if row.kind == "slot" {
			t.Fatal("sessions list should not keep hour rails")
		}
		hours = append(hours, row.hour)
	}
	if strings.Join(kinds, ",") != "pomo,now" {
		t.Fatalf("kinds=%v", kinds)
	}
	for _, hour := range hours {
		if hour != 16 {
			t.Fatalf("hour=%d", hour)
		}
	}

	logged := buildPomoLogList(grid, now, true, true)
	if logged[0].kind != "slot" || logged[0].hourLabel != "16:00" {
		t.Fatalf("log header=%+v", logged[0])
	}
	if len(logged) != 3 {
		t.Fatalf("log rows=%d %+v", len(logged), logged)
	}
}

func TestPomoLogDesignNamesRender(t *testing.T) {
	now := time.Date(2026, 10, 9, 16, 20, 0, 0, time.Local)
	row := dayGridRow{
		kind: "pomo",
		hour: 16,
		rec: ticktick.FocusRecord{
			StartTime: "2026-10-09T16:14:00.000+0200",
			EndTime:   "2026-10-09T16:41:00.000+0200",
			Tasks:     taskLink("washing options"),
		},
	}
	card := stripANSI(renderPomoLogRow(PomoLogCards, row, now, true, 80, nil, nil, pomoTimelineLayout{}))
	tape := stripANSI(renderPomoLogRow(PomoLogTape, row, now, false, 80, nil, nil, pomoTimelineLayout{}))
	if !strings.Contains(card, "┃") || !strings.Contains(card, "washing options") {
		t.Fatalf("card=%q", card)
	}
	if strings.Contains(tape, "┃") || !strings.Contains(tape, "16:14–16:41") {
		t.Fatalf("tape=%q", tape)
	}
	hour := stripANSI(renderPomoLogRow(PomoLogHours, dayGridRow{kind: "slot", hour: 16, hourLabel: "16:00"}, now, false, 40, nil, nil, pomoTimelineLayout{}))
	if !strings.Contains(hour, "16:00") || strings.Contains(hour, "╌╌") {
		t.Fatalf("hour=%q", hour)
	}
}
