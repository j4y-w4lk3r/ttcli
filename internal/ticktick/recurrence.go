package ticktick

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
)

const (
	RepeatFromDue        = 0
	RepeatFromCompletion = 1
)

type TaskRecurrence struct {
	Rule       string
	RepeatFrom int
}

var recurrenceDayPattern = regexp.MustCompile(`^([+-]?[1-5])?(MO|TU|WE|TH|FR|SA|SU)$`)

func NormalizeRecurrenceRule(raw string) (string, error) {
	raw = strings.ToUpper(strings.TrimSpace(raw))
	if raw == "" {
		return "", nil
	}
	if len(raw) > 512 || strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("repeat rule is too long or contains a newline")
	}
	switch {
	case strings.HasPrefix(raw, "RRULE:"):
		if err := validateRRule(strings.TrimPrefix(raw, "RRULE:")); err != nil {
			return "", err
		}
	case strings.HasPrefix(raw, "ERULE:"):
		if err := validateERule(strings.TrimPrefix(raw, "ERULE:")); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("repeat rule must start with RRULE: or ERULE:")
	}
	return raw, nil
}

func recurrenceFields(body string) (map[string]string, error) {
	fields := make(map[string]string)
	for _, component := range strings.Split(body, ";") {
		key, value, ok := strings.Cut(component, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || key == "" || value == "" {
			return nil, fmt.Errorf("invalid repeat component %q", component)
		}
		if _, exists := fields[key]; exists {
			return nil, fmt.Errorf("duplicate repeat component %s", key)
		}
		fields[key] = value
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("repeat rule has no components")
	}
	return fields, nil
}

func validateRRule(body string) error {
	fields, err := recurrenceFields(body)
	if err != nil {
		return err
	}
	switch fields["FREQ"] {
	case "DAILY", "WEEKLY", "MONTHLY", "YEARLY":
	case "":
		return fmt.Errorf("RRULE requires FREQ")
	default:
		return fmt.Errorf("unsupported RRULE frequency %q", fields["FREQ"])
	}
	for _, key := range []string{"INTERVAL", "COUNT"} {
		if value := fields[key]; value != "" {
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 {
				return fmt.Errorf("%s must be a positive integer", key)
			}
		}
	}
	if days := fields["BYDAY"]; days != "" {
		for _, day := range strings.Split(days, ",") {
			if !recurrenceDayPattern.MatchString(day) {
				return fmt.Errorf("invalid BYDAY value %q", day)
			}
		}
	}
	if weekStart := fields["WKST"]; weekStart != "" && !recurrenceDayPattern.MatchString(weekStart) {
		return fmt.Errorf("invalid WKST value %q", weekStart)
	}
	return nil
}

func validateERule(body string) error {
	fields, err := recurrenceFields(body)
	if err != nil {
		return err
	}
	if fields["NAME"] == "" {
		return fmt.Errorf("ERULE requires NAME")
	}
	if dates := fields["BYDATE"]; dates != "" {
		for _, raw := range strings.Split(dates, ",") {
			if _, err := time.Parse("20060102", raw); err != nil {
				return fmt.Errorf("invalid BYDATE value %q", raw)
			}
		}
	}
	return nil
}

func NewTaskRecurrence(rule string, repeatFrom int) (*TaskRecurrence, error) {
	rule, err := NormalizeRecurrenceRule(rule)
	if err != nil {
		return nil, err
	}
	if rule == "" {
		return nil, nil
	}
	if repeatFrom != RepeatFromDue && repeatFrom != RepeatFromCompletion {
		return nil, fmt.Errorf("repeat basis must be due date or completion date")
	}
	return &TaskRecurrence{Rule: rule, RepeatFrom: repeatFrom}, nil
}

func RecurrencePresetRule(preset string, anchor time.Time) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "daily":
		return "RRULE:FREQ=DAILY;INTERVAL=1"
	case "weekdays":
		return "RRULE:FREQ=WEEKLY;INTERVAL=1;WKST=MO;BYDAY=MO,TU,WE,TH,FR"
	case "weekly":
		days := []string{"SU", "MO", "TU", "WE", "TH", "FR", "SA"}
		return "RRULE:FREQ=WEEKLY;INTERVAL=1;BYDAY=" + days[int(anchor.Weekday())]
	default:
		return ""
	}
}

func RecurrencePreset(rule string) string {
	normalized, err := NormalizeRecurrenceRule(rule)
	if err != nil || normalized == "" {
		return "none"
	}
	if !strings.HasPrefix(normalized, "RRULE:") {
		return "custom"
	}
	fields, err := recurrenceFields(strings.TrimPrefix(normalized, "RRULE:"))
	if err != nil || (fields["INTERVAL"] != "" && fields["INTERVAL"] != "1") {
		return "custom"
	}
	if fields["FREQ"] == "DAILY" && recurrenceOnlyHas(fields, "FREQ", "INTERVAL") {
		return "daily"
	}
	if fields["FREQ"] == "WEEKLY" &&
		recurrenceOnlyHas(fields, "FREQ", "INTERVAL", "WKST", "BYDAY") &&
		fields["BYDAY"] == "MO,TU,WE,TH,FR" {
		return "weekdays"
	}
	if fields["FREQ"] == "WEEKLY" &&
		recurrenceOnlyHas(fields, "FREQ", "INTERVAL", "WKST", "BYDAY") &&
		len(strings.Split(fields["BYDAY"], ",")) == 1 {
		return "weekly"
	}
	return "custom"
}

func recurrenceOnlyHas(fields map[string]string, allowed ...string) bool {
	allowedSet := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = true
	}
	for key := range fields {
		if !allowedSet[key] {
			return false
		}
	}
	return true
}

// ExpandTaskOccurrences returns occurrence-shaped task copies for an inclusive
// local-day range. Unsupported TickTick extensions are preserved on the task
// and fall back to its current occurrence.
func ExpandTaskOccurrences(task Task, startDay, endDay time.Time) ([]Task, bool) {
	if !task.Repeating() {
		return []Task{task}, true
	}
	loc := startDay.Location()
	if loc == nil {
		loc = time.Local
	}
	startDay = dayInLocation(startDay, loc)
	endExclusive := dayInLocation(endDay, loc).AddDate(0, 0, 1)

	currentDue, dueOK := parseTaskDateTime(task.DueDate, loc)
	if !dueOK {
		return []Task{task}, false
	}
	duration := time.Duration(0)
	if start, ok := parseTaskDateTime(task.StartDate, loc); ok && !task.IsAllDay && currentDue.After(start) {
		duration = currentDue.Sub(start)
	}
	anchor := currentDue
	if first, ok := parseTaskDateTime(task.RepeatFirstDate, loc); ok {
		anchor = first
	}

	var dates []time.Time
	rule := strings.TrimSpace(strings.ToUpper(task.RepeatFlag))
	supported := true
	switch {
	case strings.HasPrefix(rule, "RRULE:"):
		option, err := rrule.StrToROption(strings.TrimPrefix(rule, "RRULE:"))
		if err != nil {
			supported = false
			break
		}
		option.Dtstart = anchor
		repeater, err := rrule.NewRRule(*option)
		if err != nil {
			supported = false
			break
		}
		dates = repeater.Between(startDay, endExclusive.Add(-time.Second), true)
	case strings.HasPrefix(rule, "ERULE:"):
		fields, err := recurrenceFields(strings.TrimPrefix(rule, "ERULE:"))
		if err != nil || fields["BYDATE"] == "" {
			supported = false
			break
		}
		for _, raw := range strings.Split(fields["BYDATE"], ",") {
			date, err := time.ParseInLocation("20060102", raw, loc)
			if err != nil {
				continue
			}
			date = time.Date(date.Year(), date.Month(), date.Day(), anchor.Hour(), anchor.Minute(), anchor.Second(), anchor.Nanosecond(), loc)
			if !date.Before(startDay) && date.Before(endExclusive) {
				dates = append(dates, date)
			}
		}
	default:
		supported = false
	}

	if currentDue.Before(endExclusive) && !currentDue.Before(startDay) {
		dates = append(dates, currentDue)
	}
	if !supported {
		if currentDue.Before(startDay) || !currentDue.Before(endExclusive) {
			return nil, false
		}
		return []Task{task}, false
	}
	seen := make(map[string]bool)
	out := make([]Task, 0, len(dates))
	for _, due := range dates {
		key := due.In(loc).Format("2006-01-02")
		if seen[key] {
			continue
		}
		seen[key] = true
		occurrence := task
		occurrence.DueDate = formatTaskDateTime(due, task.IsAllDay)
		if task.IsAllDay {
			occurrence.StartDate = occurrence.DueDate
		} else if duration > 0 {
			occurrence.StartDate = formatTaskDateTime(due.Add(-duration), false)
		} else {
			occurrence.StartDate = occurrence.DueDate
		}
		out = append(out, occurrence)
	}
	return out, true
}

func dayInLocation(value time.Time, loc *time.Location) time.Time {
	value = value.In(loc)
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, loc)
}

func parseTaskDateTime(value string, loc *time.Location) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := ParseAPITime(value); err == nil {
		return parsed.In(loc), true
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, loc); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func formatTaskDateTime(value time.Time, allDay bool) string {
	if allDay {
		return value.Format("2006-01-02")
	}
	return value.Format(ticktickTimeLayout)
}
