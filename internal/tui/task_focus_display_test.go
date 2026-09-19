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

func TestTaskFocusSummaryFallsBackToNativeCounters(t *testing.T) {
	m := model{}
	s, ok := m.taskFocusSummary(ticktick.Task{FocusSummaries: []ticktick.FocusSummary{{
		PomoCount: 2, PomoDuration: 3000, StopwatchDuration: 600,
	}}})
	if !ok || s.FullSessions != 2 || s.TotalSeconds != 3600 {
		t.Fatalf("summary=%+v ok=%v", s, ok)
	}
}

func TestRenderTaskFocusProgressAgainstNativePlan(t *testing.T) {
	task := ticktick.Task{FocusSummaries: []ticktick.FocusSummary{{
		EstimatedDuration: 75 * 60,
		EstimatedPomo:     3,
	}}}
	summary := ticktick.TaskFocusSummary{TotalSeconds: 25 * 60, FullSessions: 1}
	inline := stripANSI(renderTaskFocusProgressInline(task, summary))
	for _, want := range []string{"1/3", "50m left"} {
		if !strings.Contains(inline, want) {
			t.Fatalf("inline=%q missing %q", inline, want)
		}
	}
	detail := renderTaskFocusProgressDetail(task, summary)
	for _, want := range []string{"25m / 1h15m planned", "50m left", "1/3 pomos"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail=%q missing %q", detail, want)
		}
	}
}

func TestTaskRowsShowInferredRemainingTimeAndPomos(t *testing.T) {
	m := model{uiSettings: defaultUISettings()}
	task := ticktick.Task{Title: "Unestimated"}
	summary, ok := m.taskFocusDisplay(task)
	if !ok {
		t.Fatal("default task estimate should be visible")
	}
	inline := stripANSI(renderTaskFocusProgressInlineWithConfig(
		task, summary, m.uiSettings.planningConfig(),
	))
	for _, want := range []string{"0/1", "~25m left"} {
		if !strings.Contains(inline, want) {
			t.Fatalf("inline=%q missing %q", inline, want)
		}
	}
}

func TestFormattedTaskRowShowsPomoRatioAndRemainingTime(t *testing.T) {
	task := ticktick.Task{
		ID: "planned", Title: "Deep work",
		FocusSummaries: []ticktick.FocusSummary{{
			EstimatedDuration: 75 * 60,
			EstimatedPomo:     3,
		}},
	}
	m := model{
		uiSettings: defaultUISettings(),
		taskFocusByID: map[string]ticktick.TaskFocusSummary{
			task.ID: {FullSessions: 1, TotalSeconds: 25 * 60},
		},
	}
	rows := []taskListRow{{Task: task}}
	layout := computeTaskRowLayout(rows, 80, maxTaskFocusInlineW(m, rows))
	line := stripANSI(m.formatTaskLine(task, 0, false, false, 80, layout))
	for _, want := range []string{"1/3", "50m left"} {
		if !strings.Contains(line, want) {
			t.Fatalf("row=%q missing %q", line, want)
		}
	}
}

func TestListFocusEstimateIncludesExplicitAndInferredTasks(t *testing.T) {
	m := model{
		uiSettings: defaultUISettings(),
		tasks: []ticktick.Task{
			{
				ID: "planned", Title: "Planned",
				FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 60 * 60}},
			},
			{ID: "inferred", Title: "Inferred"},
		},
		taskFocusByID: map[string]ticktick.TaskFocusSummary{
			"planned": {TotalSeconds: 15 * 60},
		},
	}
	planned, remaining, inferred := m.listFocusEstimate()
	if planned != 85 || remaining != 70 || inferred != 1 {
		t.Fatalf("planned=%d remaining=%d inferred=%d", planned, remaining, inferred)
	}
	if hint := m.openDoneHint(); !strings.Contains(hint, "list ~1h25m planned · 1h10m left") {
		t.Fatalf("list hint=%q", hint)
	}
}

func noopTaskFocus(ticktick.Task) (ticktick.TaskFocusSummary, bool) {
	return ticktick.TaskFocusSummary{}, false
}
