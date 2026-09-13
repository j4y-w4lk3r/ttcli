package ticktick

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	StandardPomoMinutes = 25
	UnclaimedTitlePrefix = "Unclaimed · "
)

// RecordDuration returns the logged length of a focus record.
func RecordDuration(r FocusRecord) time.Duration {
	st, ok1 := parseRecordTime(r.StartTime)
	et, ok2 := parseRecordTime(r.EndTime)
	if !ok1 || !ok2 || !et.After(st) {
		return StandardPomoMinutes * time.Minute
	}
	return et.Sub(st)
}

func parseRecordTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := ParseAPITime(s)
	if err != nil && len(s) >= 16 {
		t, err = time.Parse("2006-01-02T15:04", s[:16])
	}
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// IsUnclaimedTitle reports logged overtime slices (post-session extra time).
func IsUnclaimedTitle(title string) bool {
	return strings.HasPrefix(title, UnclaimedTitlePrefix)
}

// IsFullPomoRecord counts toward the daily pomodoro goal: a standard-length
// planned session, never unclaimed overtime.
func IsFullPomoRecord(r FocusRecord) bool {
	if IsUnclaimedTitle(r.TaskTitle()) {
		return false
	}
	mins := int(RecordDuration(r).Minutes() + 0.5)
	return mins >= StandardPomoMinutes-1
}

// CountFullPomos returns how many records qualify as full pomodoro sessions.
func CountFullPomos(records []FocusRecord) int {
	n := 0
	for _, r := range records {
		if IsFullPomoRecord(r) {
			n++
		}
	}
	return n
}

// TaskFocusSummary is accumulated focus time linked to a task.
type TaskFocusSummary struct {
	FullSessions   int
	LoggedSessions int
	TotalSeconds   int64
}

func (s TaskFocusSummary) Visible() bool {
	return s.LoggedSessions > 0 || s.TotalSeconds > 0
}

// TaskFocusIndex maps focus history by task id and normalized title.
type TaskFocusIndex struct {
	ByID    map[string]TaskFocusSummary
	ByTitle map[string]TaskFocusSummary
}

func normalizeFocusTaskTitle(title string) string {
	return strings.ToLower(strings.TrimSpace(title))
}

// NormalizeFocusTaskTitle is exported for UI lookups.
func NormalizeFocusTaskTitle(title string) string {
	return normalizeFocusTaskTitle(title)
}

func (idx *TaskFocusIndex) add(id, title string, secs int64, full bool) {
	if id != "" {
		s := idx.ByID[id]
		s.LoggedSessions++
		s.TotalSeconds += secs
		if full {
			s.FullSessions++
		}
		idx.ByID[id] = s
	}
	key := normalizeFocusTaskTitle(title)
	if key == "" {
		return
	}
	s := idx.ByTitle[key]
	s.LoggedSessions++
	s.TotalSeconds += secs
	if full {
		s.FullSessions++
	}
	idx.ByTitle[key] = s
}

// AggregateTaskFocus builds per-task focus totals from pomodoro records.
func AggregateTaskFocus(records []FocusRecord) TaskFocusIndex {
	idx := TaskFocusIndex{
		ByID:    make(map[string]TaskFocusSummary),
		ByTitle: make(map[string]TaskFocusSummary),
	}
	for _, r := range records {
		if IsUnclaimedTitle(r.TaskTitle()) {
			continue
		}
		secs := int64(RecordDuration(r).Seconds() + 0.5)
		if secs < 1 {
			secs = 1
		}
		idx.add(r.TaskID(), r.TaskTitle(), secs, IsFullPomoRecord(r))
	}
	return idx
}

// FocusHistoryDays returns how many days of focus history to load for tasks.
func FocusHistoryDays() int {
	const defaultDays = 365
	if g := os.Getenv("TTCLI_FOCUS_HISTORY_DAYS"); g != "" {
		if n, err := strconv.Atoi(g); err == nil && n > 0 {
			return n
		}
	}
	return defaultDays
}

func localDateOnly(t time.Time) time.Time {
	if t.IsZero() {
		t = time.Now()
	}
	t = t.In(time.Local)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.Local)
}

// LocalDayBounds returns inclusive start/end for a local calendar day.
func LocalDayBounds(day time.Time) (start, end time.Time) {
	day = localDateOnly(day)
	start = day
	end = time.Date(day.Year(), day.Month(), day.Day(), 23, 59, 59, 999000000, time.Local)
	return start, end
}

// FocusRecordFilter selects logged pomodoros by local start time and task title.
type FocusRecordFilter struct {
	Day              time.Time
	TaskSubstring    string
	FromClock        string // HH:MM local, inclusive
	ToClock          string // HH:MM local, inclusive
	IncludeUnclaimed bool
}

// FilterFocusRecords returns records matching the filter (local day + clock window + title).
func FilterFocusRecords(records []FocusRecord, f FocusRecordFilter) ([]FocusRecord, error) {
	day := localDateOnly(f.Day)
	from, err := parseClockOnDay(day, f.FromClock)
	if err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	to, err := parseClockOnDay(day, f.ToClock)
	if err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	if to.Before(from) {
		return nil, fmt.Errorf("to time %s is before from %s", f.ToClock, f.FromClock)
	}
	needle := strings.ToLower(strings.TrimSpace(f.TaskSubstring))
	var out []FocusRecord
	for _, r := range records {
		title := r.TaskTitle()
		if !f.IncludeUnclaimed && IsUnclaimedTitle(title) {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(title), needle) {
			continue
		}
		st, ok := parseRecordTime(r.StartTime)
		if !ok {
			continue
		}
		loc := st.In(time.Local)
		if loc.Before(from) || loc.After(to) {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func parseClockOnDay(day time.Time, clock string) (time.Time, error) {
	clock = strings.TrimSpace(clock)
	if clock == "" {
		return time.Time{}, fmt.Errorf("empty clock")
	}
	parts := strings.Split(clock, ":")
	if len(parts) < 2 {
		return time.Time{}, fmt.Errorf("use HH:MM")
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}, fmt.Errorf("use HH:MM")
	}
	day = localDateOnly(day)
	return time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 59, 0, time.Local), nil
}

// FocusRecordsOnDay keeps records whose start time falls on the local calendar day.
func FocusRecordsOnDay(records []FocusRecord, day time.Time) []FocusRecord {
	day = localDateOnly(day)
	key := day.Format("2006-01-02")
	out := records[:0]
	for _, r := range records {
		st, ok := parseRecordTime(r.StartTime)
		if !ok {
			continue
		}
		if st.In(time.Local).Format("2006-01-02") == key {
			out = append(out, r)
		}
	}
	return out
}
