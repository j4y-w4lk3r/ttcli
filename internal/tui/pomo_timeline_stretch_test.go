package tui

import "testing"

func TestExpandPomoGridInsertsGapRows(t *testing.T) {
	grid := []dayGridRow{
		{hour: 0, kind: "slot", hourLabel: "00:00"},
		{hour: 0, kind: "pomo", recIdx: 0},
		{hour: 1, kind: "slot", hourLabel: "01:00"},
	}
	expanded := expandPomoGrid(grid, 2)
	if len(expanded) != 7 {
		t.Fatalf("len=%d want 7 (1+2+1+1+2)", len(expanded))
	}
	if expanded[1].kind != "gap" || expanded[2].kind != "gap" {
		t.Fatalf("expected gaps after first slot, got %q %q", expanded[1].kind, expanded[2].kind)
	}
	if expanded[3].kind != "pomo" {
		t.Fatalf("pomo row should follow first slot block, got %q", expanded[3].kind)
	}
}

func TestPomoGapLinesPerSlotScalesWithHeight(t *testing.T) {
	if got := pomoGapLinesPerSlot(30); got < 2 {
		t.Fatalf("30 rows should get at least 2 gaps, got %d", got)
	}
	if got := pomoGapLinesPerSlot(80); got < 6 {
		t.Fatalf("80 rows should get generous gaps, got %d", got)
	}
}

func TestFillPomoTimelineStretch(t *testing.T) {
	lines := []string{"a", "b"}
	out := fillPomoTimelineStretch(lines, 5, 40, false)
	if len(out) != 5 {
		t.Fatalf("len=%d want 5", len(out))
	}
}
