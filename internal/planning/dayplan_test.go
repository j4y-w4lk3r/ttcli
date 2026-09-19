package planning

import (
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestScheduledMinutes(t *testing.T) {
	task := ticktick.Task{
		StartDate: "2026-09-17T09:00:00.000+0200",
		DueDate:   "2026-09-17T10:30:00.000+0200",
	}
	minutes, ok := ScheduledMinutes(task)
	if !ok || minutes != 90 {
		t.Fatalf("minutes=%d ok=%v", minutes, ok)
	}
	task.IsAllDay = true
	if _, ok := ScheduledMinutes(task); ok {
		t.Fatal("all-day task must use fallback estimate")
	}
}

func TestEstimateTaskPrefersNativeFocusBudget(t *testing.T) {
	task := ticktick.Task{
		StartDate: "2026-09-17T09:00:00.000+0200",
		DueDate:   "2026-09-17T10:00:00.000+0200",
		FocusSummaries: []ticktick.FocusSummary{{
			EstimatedDuration: 4500,
			EstimatedPomo:     3,
		}},
	}
	estimate := EstimateTask(task, DefaultConfig())
	if estimate.Minutes != 75 || estimate.Pomos != 3 || estimate.Source != EstimateNativeFocus {
		t.Fatalf("estimate=%+v", estimate)
	}
}

func TestBuildDayPlanCapacityAndEvidence(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	config := Config{
		WorkStartMinutes: 9 * 60,
		WorkEndMinutes:   18 * 60,
		BufferMinutes:    30,
		DefaultMinutes:   25,
	}
	plan := BuildDayPlan(day, now, []TaskInput{
		{Task: ticktick.Task{
			StartDate: "2026-09-17T13:00:00.000+0200",
			DueDate:   "2026-09-17T15:00:00.000+0200",
		}},
		{Task: ticktick.Task{Title: "inferred"}, Overdue: true},
	}, config)
	if plan.NeededMinutes != 145 {
		t.Fatalf("needed=%d", plan.NeededMinutes)
	}
	if plan.AvailableMinutes != 330 {
		t.Fatalf("available=%d", plan.AvailableMinutes)
	}
	if plan.Feasibility != FeasibilityComfortable {
		t.Fatalf("feasibility=%q", plan.Feasibility)
	}
	if plan.Evidence != EvidenceHigh {
		t.Fatalf("evidence=%q", plan.Evidence)
	}
	if plan.InferredTasks != 1 || plan.OverdueCount != 1 {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestFeasibilityThresholds(t *testing.T) {
	tests := []struct {
		needed    int
		available int
		want      Feasibility
	}{
		{0, 100, FeasibilityClear},
		{75, 100, FeasibilityComfortable},
		{76, 100, FeasibilityTight},
		{100, 100, FeasibilityTight},
		{101, 100, FeasibilityOverCapacity},
	}
	for _, test := range tests {
		if got := feasibilityFor(test.needed, test.available, false); got != test.want {
			t.Fatalf("needed=%d available=%d got=%q want=%q", test.needed, test.available, got, test.want)
		}
	}
}

func TestPastDayHasNoFeasibilityClaim(t *testing.T) {
	day := time.Date(2026, 9, 16, 0, 0, 0, 0, time.Local)
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.Local)
	plan := BuildDayPlan(day, now, []TaskInput{{Task: ticktick.Task{}}}, DefaultConfig())
	if !plan.Historical || plan.Feasibility != FeasibilityNone {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestBuildCoachingPlanCreditsCompletionAndActiveSession(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	config := Config{
		WorkStartMinutes: 9 * 60, WorkEndMinutes: 17 * 60,
		DefaultMinutes: 25,
	}
	tasks := []TaskInput{
		{Task: ticktick.Task{
			ID: "done", Title: "Done",
			FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 6000, EstimatedPomo: 4}},
		}, Done: true},
		{Task: ticktick.Task{
			ID: "food", Title: "Food",
			FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 6000, EstimatedPomo: 4}},
		}, LoggedMinutes: 20, LoggedPomos: 1},
	}
	plan := BuildCoachingPlan(CoachingInput{
		Day: day, Now: day.Add(4 * time.Hour).Add(9 * time.Hour),
		Tasks: tasks, ActiveMinutes: 10, ActiveTaskID: "food", Config: config,
	})
	if plan.PlannedMinutes != 200 || plan.LoggedMinutes != 30 ||
		plan.CreditedMinutes != 130 || plan.RemainingMinutes != 70 {
		t.Fatalf("minutes=%+v", plan)
	}
	if plan.PlannedPomos != 8 || plan.CompletedPomos != 5 {
		t.Fatalf("pomos=%+v", plan)
	}
	if plan.PaceDeltaMinutes != 30 || plan.SlackMinutes != 170 {
		t.Fatalf("pace/slack=%+v", plan)
	}
	if plan.Guidance != "30m ahead · start Food next" {
		t.Fatalf("guidance=%q", plan.Guidance)
	}
	if tasks[1].LoggedMinutes != 20 {
		t.Fatal("coaching model mutated task inputs")
	}
}

func TestBuildCoachingPlanSuggestsDeferral(t *testing.T) {
	day := time.Date(2026, 9, 17, 0, 0, 0, 0, time.Local)
	plan := BuildCoachingPlan(CoachingInput{
		Day: day, Now: day.Add(17 * time.Hour),
		Tasks: []TaskInput{{Task: ticktick.Task{
			Title:          "Large task",
			FocusSummaries: []ticktick.FocusSummary{{EstimatedDuration: 7200}},
		}}},
		Config: Config{WorkStartMinutes: 9 * 60, WorkEndMinutes: 18 * 60, DefaultMinutes: 25},
	})
	if plan.SlackMinutes != -60 || plan.Guidance != "Defer 1h to get on track" {
		t.Fatalf("plan=%+v", plan)
	}
}

func TestParseClockMinutes(t *testing.T) {
	if got, err := ParseClockMinutes("09:30"); err != nil || got != 570 {
		t.Fatalf("got=%d err=%v", got, err)
	}
	if _, err := ParseClockMinutes("25:00"); err == nil {
		t.Fatal("expected invalid clock")
	}
}
