package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Habit is a TickTick habit tracker entry.
type Habit struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Color         string  `json:"color"`
	IconRes       string  `json:"iconRes"`
	TotalCheckIns flexInt `json:"totalCheckIns"`
	Goal          flexInt `json:"goal"`
	Status        flexInt `json:"status"`
	Encouragement string  `json:"encouragement"`
}

// GoalValue returns the daily goal, defaulting to 1.
func (h Habit) GoalValue() int {
	if g := h.Goal.Int(); g >= 1 {
		return g
	}
	return 1
}

// ListHabits returns active habits from the private API.
func (c *Client) ListHabits() ([]Habit, error) {
	var habits []Habit
	if err := c.getJSON("/api/v2/habits", &habits); err != nil {
		return nil, err
	}
	return habits, nil
}

// LogPomodoroInput describes a completed focus session to sync to TickTick.
type LogPomodoroInput struct {
	TaskID        string
	TaskTitle     string
	ProjectName   string
	StartedAt     time.Time
	Elapsed       time.Duration
	PauseDuration time.Duration
}

// LogPomodoro records a completed Pomodoro via POST /api/v2/batch/pomodoro.
func (c *Client) LogPomodoro(in LogPomodoroInput) (string, error) {
	if in.Elapsed <= 0 {
		return "", fmt.Errorf("elapsed duration must be positive")
	}
	start := in.StartedAt.UTC()
	end := start.Add(in.Elapsed + in.PauseDuration)
	startS := start.Format(ticktickTimeLayout)
	endS := end.Format(ticktickTimeLayout)
	id := generateID()
	durationMs := in.Elapsed.Milliseconds()
	pauseMs := in.PauseDuration.Milliseconds()

	record := map[string]any{
		"id":            id,
		"status":        1,
		"startTime":     startS,
		"endTime":       endS,
		"pauseDuration": pauseMs,
		"duration":      durationMs,
		"relationType":  []int{2},
	}
	if in.TaskID != "" {
		record["tasks"] = []any{map[string]any{
			"taskId":      in.TaskID,
			"title":       in.TaskTitle,
			"projectName": in.ProjectName,
			"startTime":   startS,
			"endTime":     endS,
		}}
	}

	payload := map[string]any{"add": []any{record}}
	b, _ := json.Marshal(payload)
	rb, err := c.do(http.MethodPost, "/api/v2/batch/pomodoro", b)
	if err != nil {
		return "", err
	}
	got, err := canonicalID(rb, id)
	if err != nil {
		return got, err
	}
	if got == "" {
		return id, nil
	}
	return got, nil
}
