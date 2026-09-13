package tui

import "testing"

func TestPomoStackHeightsFavorsTimeline(t *testing.T) {
	focus, timeline := pomoStackHeights(30)
	if timeline <= focus {
		t.Fatalf("timeline=%d focus=%d want timeline > focus", timeline, focus)
	}
	if focus+timeline != 30 {
		t.Fatalf("sum=%d want 30", focus+timeline)
	}
	if timeline < 18 {
		t.Fatalf("timeline=%d want at least ~60%% of 30", timeline)
	}
}

func TestPomoStackHeightsSmallTerminal(t *testing.T) {
	focus, timeline := pomoStackHeights(10)
	if focus+timeline != 10 {
		t.Fatalf("sum=%d", focus+timeline)
	}
	if timeline < focus {
		t.Fatalf("timeline=%d focus=%d", timeline, focus)
	}
}
