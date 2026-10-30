package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestPausesInsideRecordStayWithThatSession(t *testing.T) {
	first := ticktick.FocusRecord{
		StartTime: "2026-10-10T09:33:00.000+0200",
		EndTime:   "2026-10-10T09:58:00.000+0200",
		Tasks:     taskLink("rui"),
	}
	second := ticktick.FocusRecord{
		StartTime: "2026-10-10T09:59:00.000+0200",
		EndTime:   "2026-10-10T10:24:00.000+0200",
		Tasks:     taskLink("rui"),
	}
	start, _ := focusTimeLocal(first.StartTime)
	pauses := []focus.PauseSpell{
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 48*time.Second), End: start.Add(21*time.Minute + 50*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 51*time.Second), End: start.Add(21*time.Minute + 52*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 52*time.Second), End: start.Add(21*time.Minute + 53*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 54*time.Second), End: start.Add(21*time.Minute + 58*time.Second)},
		{TaskTitle: "other", Start: start.Add(10 * time.Minute)},
	}
	if got := pausesInsideRecord(first, pauses); len(got) != 4 {
		t.Fatalf("first session pauses=%d", len(got))
	}
	if got := pausesInsideRecord(second, pauses); len(got) != 0 {
		t.Fatalf("next session pauses=%d", len(got))
	}

	now := start.Add(2 * time.Hour)
	grid := buildPomoHourGrid([]ticktick.FocusRecord{first, second}, pauses[:4], now, nil, false)
	var markedOpen, markedClose, plain int
	var order []string
	for _, row := range grid {
		if row.kind != "pomo" && row.kind != "pause" {
			continue
		}
		if row.hour != 9 {
			continue
		}
		switch {
		case row.sessionOpen:
			markedOpen++
			order = append(order, "open")
		case row.sessionClose:
			markedClose++
			order = append(order, "close")
		case row.kind == "pause":
			order = append(order, "pause")
		default:
			plain++
			order = append(order, "session")
		}
	}
	if markedOpen != 1 || markedClose != 1 || plain != 1 {
		t.Fatalf("open=%d close=%d plain=%d order=%v", markedOpen, markedClose, plain, order)
	}
	if strings.Join(order, ",") != "open,pause,pause,pause,pause,close,session" {
		t.Fatalf("order=%v", order)
	}
}

func TestSessionPauseTrackBubblesLateInTheBlock(t *testing.T) {
	start := time.Date(2026, 10, 10, 9, 33, 0, 0, time.Local)
	end := start.Add(25 * time.Minute)
	pauses := []focus.PauseSpell{
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 48*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 51*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 52*time.Second)},
		{TaskTitle: "rui", Start: start.Add(21*time.Minute + 54*time.Second)},
	}
	track := stripANSI(renderSessionPauseTrack(20, colorPeach, start, end, pauses, false))
	if strings.Count(track, "◉") != 1 {
		t.Fatalf("track=%q", track)
	}
	at := 0
	for _, r := range track {
		if r == '◉' {
			break
		}
		at++
	}
	if at < 14 {
		t.Fatalf("bubble too early at %d in %q", at, track)
	}

	open := stripANSI(renderHourPomoOpenRow(false, "rui", colorPeach, "09:33", 160))
	if !strings.Contains(open, "09:33") || strings.Contains(open, "09:58") || strings.Contains(open, "──▶") {
		t.Fatalf("open=%q", open)
	}
	closeLine := stripANSI(renderHourPomoCloseRow(
		false, "rui", colorPeach, "09:58", 25, 160, pomoTimelineLayout{suffixStartCol: 24},
		pauses, start, end,
	))
	if !strings.Contains(closeLine, "09:58") || !strings.Contains(closeLine, "◉") || !strings.Contains(closeLine, "4 pauses") || !strings.Contains(closeLine, "25m") {
		t.Fatalf("close=%q", closeLine)
	}
	if strings.Contains(closeLine, "09:33") {
		t.Fatalf("close=%q", closeLine)
	}
}
