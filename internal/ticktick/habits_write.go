package ticktick

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HabitCheckin is one day's check-in for a habit.
type HabitCheckin struct {
	ID           string  `json:"id"`
	HabitID      string  `json:"habitId"`
	CheckinStamp flexInt `json:"checkinStamp"`
	Status       flexInt `json:"status"` // 0 unlabeled, 1 undone, 2 done
	Value        flexInt `json:"value"`
	Goal         flexInt `json:"goal"`
}

func checkinStamp(day time.Time) int {
	day = day.UTC()
	return day.Year()*10000 + int(day.Month())*100 + day.Day()
}

func dateOnlyStr(day time.Time) string {
	return day.Format("2006-01-02")
}

// HabitCheckinsForDay returns today's check-ins keyed by habit id (done only).
func (c *Client) HabitCheckinsForDay(habitIDs []string, day time.Time) (map[string]HabitCheckin, error) {
	out := map[string]HabitCheckin{}
	if len(habitIDs) == 0 {
		return out, nil
	}
	checkins, err := c.queryHabitCheckins(habitIDs, dateOnlyStr(day), dateOnlyStr(day))
	if err != nil {
		return nil, err
	}
	for _, ch := range checkins {
		if ch.Status.Int() == 2 {
			out[ch.HabitID] = ch
		}
	}
	return out, nil
}

func (c *Client) queryHabitCheckins(habitIDs []string, startDate, endDate string) ([]HabitCheckin, error) {
	payload := map[string]any{
		"habitIds":  habitIDs,
		"startDate": startDate,
		"endDate":   endDate,
	}
	b, _ := json.Marshal(payload)
	rb, err := c.do(http.MethodPost, "/api/v2/habitCheckins/query", b)
	if err != nil {
		return nil, err
	}
	var flat struct {
		HabitCheckins []HabitCheckin `json:"habitCheckins"`
	}
	if err := json.Unmarshal(rb, &flat); err == nil && len(flat.HabitCheckins) > 0 {
		return flat.HabitCheckins, nil
	}
	var nested struct {
		Checkins map[string][]HabitCheckin `json:"checkins"`
	}
	if err := json.Unmarshal(rb, &nested); err != nil {
		return nil, fmt.Errorf("decode habit checkins: %w", err)
	}
	var out []HabitCheckin
	for _, list := range nested.Checkins {
		out = append(out, list...)
	}
	return out, nil
}

// UpsertHabitCheckin marks a habit done or undone for the given day.
func (c *Client) UpsertHabitCheckin(habitID string, day time.Time, done bool, goal int) error {
	if habitID == "" {
		return fmt.Errorf("habit id required")
	}
	if goal < 1 {
		goal = 1
	}
	dayStr := dateOnlyStr(day)
	existing, err := c.queryHabitCheckins([]string{habitID}, dayStr, dayStr)
	if err != nil {
		return err
	}
	stamp := checkinStamp(day)
	status := 2
	value := goal
	if !done {
		status = 1
		value = 0
	}
	checkin := map[string]any{
		"habitId":      habitID,
		"checkinStamp": stamp,
		"checkinTime":  time.Now().UTC().Format(ticktickTimeLayout),
		"goal":         goal,
		"value":        value,
		"status":       status,
	}
	var payload map[string]any
	if len(existing) > 0 {
		checkin["id"] = existing[0].ID
		payload = map[string]any{"update": []any{checkin}}
	} else {
		checkin["id"] = generateID()
		payload = map[string]any{"add": []any{checkin}}
	}
	b, _ := json.Marshal(payload)
	_, err = c.do(http.MethodPost, "/api/v2/habitCheckins/batch", b)
	return err
}

// UpdateHabitName renames a habit.
func (c *Client) UpdateHabitName(id, name string) error {
	if id == "" || name == "" {
		return fmt.Errorf("habit id and name required")
	}
	payload := map[string]any{
		"add":    []any{},
		"update": []any{map[string]any{"id": id, "name": name}},
		"delete": []any{},
	}
	b, _ := json.Marshal(payload)
	_, err := c.do(http.MethodPost, "/api/v2/habits/batch", b)
	return err
}

// DeleteHabit removes a habit permanently.
func (c *Client) DeleteHabit(id string) error {
	if id == "" {
		return fmt.Errorf("habit id required")
	}
	payload := map[string]any{
		"add":    []any{},
		"update": []any{},
		"delete": []string{id},
	}
	b, _ := json.Marshal(payload)
	_, err := c.do(http.MethodPost, "/api/v2/habits/batch", b)
	return err
}
