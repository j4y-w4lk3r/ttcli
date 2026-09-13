package tui

import (
	"strings"
	"testing"
)

func TestIsUnclaimedFocusTitle(t *testing.T) {
	if !isUnclaimedFocusTitle("Unclaimed · Write docs") {
		t.Fatal("expected unclaimed title")
	}
	if isUnclaimedFocusTitle("Write docs") {
		t.Fatal("expected normal title")
	}
}

func TestRenderUnclaimedTimelineTitle(t *testing.T) {
	got := renderUnclaimedTimelineTitle("Unclaimed · Buy milk", false)
	if !strings.Contains(got, "×") {
		t.Fatalf("missing unclaimed mark: %q", got)
	}
	if strings.Contains(got, "UNCLAIMED") {
		t.Fatalf("should not show UNCLAIMED label: %q", got)
	}
	if !strings.Contains(got, "Buy milk") {
		t.Fatalf("missing task name: %q", got)
	}
	if strings.Contains(got, "Unclaimed ·") {
		t.Fatalf("should strip prefix: %q", got)
	}
}

func TestUnclaimedDurationBarUsesDashedFill(t *testing.T) {
	got := unclaimedDurationBarOnly(30, false)
	if !strings.Contains(got, "▪") {
		t.Fatalf("expected dashed fill: %q", got)
	}
}
