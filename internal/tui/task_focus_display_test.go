package tui

import (
	"strings"
	"testing"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestRenderTaskFocusInline(t *testing.T) {
	got := renderTaskFocusInline(ticktick.TaskFocusSummary{
		FullSessions:   11,
		LoggedSessions: 11,
		TotalSeconds:   4*3600 + 35*60,
	})
	if !strings.Contains(got, "11") {
		t.Fatalf("missing session count: %q", got)
	}
	if !strings.Contains(got, "4h35m") {
		t.Fatalf("missing duration: %q", got)
	}
}

func TestTaskFocusSummaryLookupByTitle(t *testing.T) {
	m := model{
		taskFocusByTitle: map[string]ticktick.TaskFocusSummary{
			"typ (apple keyboard)": {FullSessions: 3, LoggedSessions: 3, TotalSeconds: 4500},
		},
	}
	s, ok := m.taskFocusSummary(ticktick.Task{Title: "typ (apple keyboard)"})
	if !ok || s.FullSessions != 3 {
		t.Fatalf("lookup failed: ok=%v summary=%+v", ok, s)
	}
}

func noopTaskFocus(ticktick.Task) (ticktick.TaskFocusSummary, bool) {
	return ticktick.TaskFocusSummary{}, false
}
