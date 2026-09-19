package planning

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	DefaultWorkStartMinutes = 9 * 60
	DefaultWorkEndMinutes   = 18 * 60
	DefaultBufferMinutes    = 30
	DefaultTaskMinutes      = 25
)

type Config struct {
	WorkStartMinutes int
	WorkEndMinutes   int
	BufferMinutes    int
	DefaultMinutes   int
}

func DefaultConfig() Config {
	return Config{
		WorkStartMinutes: DefaultWorkStartMinutes,
		WorkEndMinutes:   DefaultWorkEndMinutes,
		BufferMinutes:    DefaultBufferMinutes,
		DefaultMinutes:   DefaultTaskMinutes,
	}
}

func (c Config) Normalized() Config {
	defaults := DefaultConfig()
	if c.WorkStartMinutes < 0 || c.WorkStartMinutes >= 24*60 {
		c.WorkStartMinutes = defaults.WorkStartMinutes
	}
	if c.WorkEndMinutes <= c.WorkStartMinutes || c.WorkEndMinutes > 24*60 {
		c.WorkEndMinutes = defaults.WorkEndMinutes
	}
	if c.BufferMinutes < 0 || c.BufferMinutes > c.WorkEndMinutes-c.WorkStartMinutes {
		c.BufferMinutes = defaults.BufferMinutes
	}
	if c.DefaultMinutes < 1 || c.DefaultMinutes > 24*60 {
		c.DefaultMinutes = defaults.DefaultMinutes
	}
	return c
}

// ParseClockMinutes parses a local HH:MM clock into minutes since midnight.
func ParseClockMinutes(raw string) (int, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("clock must use HH:MM")
	}
	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, fmt.Errorf("invalid hour")
	}
	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("invalid minute")
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("clock outside 00:00–23:59")
	}
	return hour*60 + minute, nil
}

type TaskInput struct {
	Task          ticktick.Task
	Overdue       bool
	Done          bool
	LoggedMinutes int
	LoggedPomos   int
}

type EstimateSource string

const (
	EstimateNativeFocus EstimateSource = "native focus"
	EstimateCalendar    EstimateSource = "calendar block"
	EstimateLogged      EstimateSource = "logged"
	EstimateDefault     EstimateSource = "default"
)

type TaskEstimate struct {
	Minutes  int
	Explicit bool
	Pomos    int
	Source   EstimateSource
}

// ScheduledMinutes returns the explicit time block represented by start/due.
func ScheduledMinutes(task ticktick.Task) (int, bool) {
	if task.IsAllDay || task.StartDate == "" || task.DueDate == "" {
		return 0, false
	}
	start, err := ticktick.ParseAPITime(task.StartDate)
	if err != nil {
		return 0, false
	}
	due, err := ticktick.ParseAPITime(task.DueDate)
	if err != nil || !due.After(start) {
		return 0, false
	}
	minutes := int(due.Sub(start).Minutes() + 0.5)
	if minutes < 1 {
		return 0, false
	}
	return minutes, true
}

func EstimateTask(task ticktick.Task, config Config) TaskEstimate {
	if seconds, pomos, ok := task.FocusEstimate(); ok {
		minutes := int((seconds + 59) / 60)
		if minutes == 0 {
			minutes = pomos * ticktick.StandardPomoMinutes
		}
		if pomos == 0 {
			pomos = (minutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes
		}
		return TaskEstimate{
			Minutes: minutes, Pomos: pomos, Explicit: true, Source: EstimateNativeFocus,
		}
	}
	if minutes, ok := ScheduledMinutes(task); ok {
		return TaskEstimate{
			Minutes:  minutes,
			Pomos:    (minutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes,
			Explicit: true,
			Source:   EstimateCalendar,
		}
	}
	config = config.Normalized()
	return TaskEstimate{
		Minutes: config.DefaultMinutes,
		Pomos:   (config.DefaultMinutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes,
		Source:  EstimateDefault,
	}
}

type Feasibility string

const (
	FeasibilityNone         Feasibility = ""
	FeasibilityClear        Feasibility = "clear"
	FeasibilityOnTrack      Feasibility = "on track"
	FeasibilityComfortable  Feasibility = FeasibilityOnTrack
	FeasibilityTight        Feasibility = "tight"
	FeasibilityOverCapacity Feasibility = "over capacity"
)

type Evidence string

const (
	EvidenceLow    Evidence = "low"
	EvidenceMedium Evidence = "medium"
	EvidenceHigh   Evidence = "high"
)

type DayPlan struct {
	Day              time.Time
	TaskCount        int
	OverdueCount     int
	NeededMinutes    int
	PlannedMinutes   int
	LoggedMinutes    int
	CreditedMinutes  int
	RemainingMinutes int
	AvailableMinutes int
	SlackMinutes     int
	PaceDeltaMinutes int
	PlannedPomos     int
	CompletedPomos   int
	ExplicitMinutes  int
	ExplicitTasks    int
	InferredTasks    int
	Feasibility      Feasibility
	Evidence         Evidence
	Historical       bool
	NextTaskTitle    string
	Guidance         string
}

type CoachingInput struct {
	Day                     time.Time
	Now                     time.Time
	Tasks                   []TaskInput
	UnassignedLoggedMinutes int
	UnassignedLoggedPomos   int
	ActiveMinutes           int
	ActiveTaskID            string
	Config                  Config
}

func localDay(day time.Time) time.Time {
	day = day.In(time.Local)
	return time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, time.Local)
}

func clockOnDay(day time.Time, minutes int) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), minutes/60, minutes%60, 0, 0, time.Local)
}

func AvailableMinutes(day, now time.Time, config Config) (int, bool) {
	config = config.Normalized()
	day = localDay(day)
	today := localDay(now)
	if day.Before(today) {
		return 0, true
	}
	start := clockOnDay(day, config.WorkStartMinutes)
	end := clockOnDay(day, config.WorkEndMinutes)
	if day.Equal(today) && now.After(start) {
		start = now
	}
	if !end.After(start) {
		return 0, false
	}
	available := int(end.Sub(start).Minutes()) - config.BufferMinutes
	if available < 0 {
		available = 0
	}
	return available, false
}

func evidenceFor(explicit, total int) Evidence {
	if total <= 0 {
		return EvidenceHigh
	}
	ratio := float64(explicit) / float64(total)
	switch {
	case ratio >= 0.80:
		return EvidenceHigh
	case ratio >= 0.50:
		return EvidenceMedium
	default:
		return EvidenceLow
	}
}

func feasibilityFor(needed, available int, historical bool) Feasibility {
	if historical {
		return FeasibilityNone
	}
	if needed <= 0 {
		return FeasibilityClear
	}
	if available <= 0 {
		return FeasibilityOverCapacity
	}
	ratio := float64(needed) / float64(available)
	switch {
	case ratio <= 0.75:
		return FeasibilityComfortable
	case ratio <= 1:
		return FeasibilityTight
	default:
		return FeasibilityOverCapacity
	}
}

func BuildDayPlan(day, now time.Time, tasks []TaskInput, config Config) DayPlan {
	return BuildCoachingPlan(CoachingInput{Day: day, Now: now, Tasks: tasks, Config: config})
}

func BuildCoachingPlan(input CoachingInput) DayPlan {
	day, now := input.Day, input.Now
	tasks := append([]TaskInput(nil), input.Tasks...)
	config := input.Config.Normalized()
	if input.ActiveMinutes > 0 {
		assigned := false
		for i := range tasks {
			if input.ActiveTaskID != "" &&
				(tasks[i].Task.ID == input.ActiveTaskID || tasks[i].Task.SeriesID() == input.ActiveTaskID) {
				tasks[i].LoggedMinutes += input.ActiveMinutes
				assigned = true
				break
			}
		}
		if !assigned {
			input.UnassignedLoggedMinutes += input.ActiveMinutes
		}
	}
	plan := DayPlan{Day: localDay(day)}
	for _, taskInput := range tasks {
		estimate := EstimateTask(taskInput.Task, config)
		plan.TaskCount++
		plan.PlannedMinutes += estimate.Minutes
		plan.PlannedPomos += estimate.Pomos
		if taskInput.Overdue {
			plan.OverdueCount++
		}
		if estimate.Explicit {
			plan.ExplicitTasks++
			plan.ExplicitMinutes += estimate.Minutes
		} else {
			plan.InferredTasks++
		}
		logged := max(taskInput.LoggedMinutes, 0)
		plan.LoggedMinutes += logged
		credited := min(logged, estimate.Minutes)
		completedPomos := min(max(taskInput.LoggedPomos, 0), estimate.Pomos)
		if taskInput.Done {
			credited = estimate.Minutes
			completedPomos = estimate.Pomos
		} else {
			remaining := estimate.Minutes - credited
			if remaining > 0 {
				plan.RemainingMinutes += remaining
				if plan.NextTaskTitle == "" {
					plan.NextTaskTitle = taskInput.Task.Title
					if plan.NextTaskTitle == "" {
						plan.NextTaskTitle = "next task"
					}
				}
			}
		}
		plan.CreditedMinutes += credited
		plan.CompletedPomos += completedPomos
	}
	plan.LoggedMinutes += max(input.UnassignedLoggedMinutes, 0)
	plan.CreditedMinutes += max(input.UnassignedLoggedMinutes, 0)
	plan.CompletedPomos += max(input.UnassignedLoggedPomos, 0)
	if plan.CompletedPomos > plan.PlannedPomos {
		plan.CompletedPomos = plan.PlannedPomos
	}
	plan.NeededMinutes = plan.RemainingMinutes
	plan.AvailableMinutes, plan.Historical = AvailableMinutes(day, now, config)
	plan.SlackMinutes = plan.AvailableMinutes - plan.RemainingMinutes
	plan.Evidence = evidenceFor(plan.ExplicitMinutes, plan.NeededMinutes)
	plan.Feasibility = feasibilityFor(plan.NeededMinutes, plan.AvailableMinutes, plan.Historical)
	plan.PaceDeltaMinutes = paceDelta(plan, now, config)
	plan.Guidance = coachingGuidance(plan)
	return plan
}

func paceDelta(plan DayPlan, now time.Time, config Config) int {
	if plan.Historical || !localDay(plan.Day).Equal(localDay(now)) || plan.PlannedMinutes == 0 {
		return 0
	}
	start := clockOnDay(plan.Day, config.WorkStartMinutes)
	end := clockOnDay(plan.Day, config.WorkEndMinutes)
	if !now.After(start) {
		return plan.CreditedMinutes
	}
	elapsed := now.Sub(start)
	if now.After(end) {
		elapsed = end.Sub(start)
	}
	total := end.Sub(start)
	if total <= 0 {
		return 0
	}
	expected := int(float64(plan.PlannedMinutes)*float64(elapsed)/float64(total) + 0.5)
	return plan.CreditedMinutes - expected
}

func coachingGuidance(plan DayPlan) string {
	if plan.Historical {
		return "Historical day"
	}
	if plan.RemainingMinutes <= 0 {
		return "Plan complete"
	}
	if plan.SlackMinutes < 0 {
		return "Defer " + FormatMinutes(-plan.SlackMinutes) + " to get on track"
	}
	if plan.PaceDeltaMinutes > 0 {
		return fmt.Sprintf("%s ahead · start %s next", FormatMinutes(plan.PaceDeltaMinutes), plan.NextTaskTitle)
	}
	if plan.PaceDeltaMinutes < 0 {
		return fmt.Sprintf("%s behind · start %s now", FormatMinutes(-plan.PaceDeltaMinutes), plan.NextTaskTitle)
	}
	if plan.NextTaskTitle != "" {
		return "Start " + plan.NextTaskTitle + " now"
	}
	return "No next action"
}

func FormatMinutes(minutes int) string {
	if minutes < 0 {
		minutes = 0
	}
	hours := minutes / 60
	remainder := minutes % 60
	switch {
	case hours > 0 && remainder > 0:
		return fmt.Sprintf("%dh%02dm", hours, remainder)
	case hours > 0:
		return fmt.Sprintf("%dh", hours)
	default:
		return fmt.Sprintf("%dm", remainder)
	}
}
