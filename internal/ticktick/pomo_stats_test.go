package ticktick_test

import (
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestFocusRecordsOnDayLocal(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	day := time.Date(2026, 9, 1, 0, 0, 0, 0, time.Local)
	recs := []ticktick.FocusRecord{
		{StartTime: "2026-08-31T12:14:00.000+0200", EndTime: "2026-08-31T12:16:00.000+0200"},
		{StartTime: "2026-09-01T00:30:00.000+0200", EndTime: "2026-09-01T00:55:00.000+0200"},
		{StartTime: "2026-09-01T12:14:00.000+0200", EndTime: "2026-09-01T12:16:00.000+0200"},
		{StartTime: "2026-09-01T23:30:00.000+0200", EndTime: "2026-09-01T23:55:00.000+0200"},
	}
	got := ticktick.FocusRecordsOnDay(recs, day)
	if len(got) != 3 {
		t.Fatalf("want 3 records on Sep 1 local, got %d", len(got))
	}
}

func TestLocalDayBoundsUsesLocalMidnight(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	day := time.Date(2026, 9, 1, 15, 0, 0, 0, time.Local)
	start, end := ticktick.LocalDayBounds(day)
	if start.Hour() != 0 || start.Day() != 1 || start.Month() != time.September {
		t.Fatalf("start=%v", start)
	}
	if end.Hour() != 23 || end.Minute() != 59 || end.Day() != 1 {
		t.Fatalf("end=%v", end)
	}
	// Sep 1 00:00 Warsaw is Aug 31 22:00 UTC (CEST ended, assume +1 in Nov - Sep 1 2026 is CEST +2)
	wantStartUTC := time.Date(2026, 8, 31, 22, 0, 0, 0, time.UTC)
	if !start.UTC().Equal(wantStartUTC) {
		t.Fatalf("start UTC=%v want %v", start.UTC(), wantStartUTC)
	}
}

func TestFilterFocusRecordsByTaskAndClock(t *testing.T) {
	t.Setenv("TZ", "Europe/Warsaw")
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, time.Local)
	recs := []ticktick.FocusRecord{
		{ID: "a", StartTime: "2026-09-10T11:13:00.000+0200", EndTime: "2026-09-10T11:38:00.000+0200"},
		{ID: "b", StartTime: "2026-09-10T12:09:00.000+0200", EndTime: "2026-09-10T12:34:00.000+0200"},
		{ID: "c", StartTime: "2026-09-10T12:40:00.000+0200", EndTime: "2026-09-10T12:41:00.000+0200"},
		{ID: "d", StartTime: "2026-09-10T13:05:00.000+0200", EndTime: "2026-09-10T13:30:00.000+0200"},
	}
	recs[0].SetTaskTitle("breakfast")
	recs[1].SetTaskTitle("breakfast")
	recs[2].SetTaskTitle(ticktick.UnclaimedTitlePrefix + "breakfast")
	recs[3].SetTaskTitle("breakfast")
	got, err := ticktick.FilterFocusRecords(recs, ticktick.FocusRecordFilter{
		Day:           day,
		TaskSubstring: "breakfast",
		FromClock:     "12:08",
		ToClock:       "13:00",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "b" {
		t.Fatalf("got %d records ids=%v want [b]", len(got), recordIDs(got))
	}
}

func recordIDs(recs []ticktick.FocusRecord) []string {
	out := make([]string, len(recs))
	for i, r := range recs {
		out[i] = r.ID
	}
	return out
}

func TestIsFullPomoRecord(t *testing.T) {
	full := ticktick.FocusRecord{
		StartTime: "2026-09-01T11:30:00.000+0200",
		EndTime:   "2026-09-01T11:55:00.000+0200",
		Tasks: []struct {
			TaskID      string `json:"taskId"`
			Title       string `json:"title"`
			ProjectName string `json:"projectName"`
			StartTime   string `json:"startTime"`
			EndTime     string `json:"endTime"`
		}{{Title: "Task"}},
	}
	if !ticktick.IsFullPomoRecord(full) {
		t.Fatal("expected 25m session to count as full")
	}
	short := full
	short.EndTime = "2026-09-01T11:35:00.000+0200"
	if ticktick.IsFullPomoRecord(short) {
		t.Fatal("expected short session not to count as full")
	}
	unclaimed := full
	unclaimed.Tasks[0].Title = ticktick.UnclaimedTitlePrefix + "Task"
	if ticktick.IsFullPomoRecord(unclaimed) {
		t.Fatal("unclaimed must not count as full")
	}
}

func TestRecordDurationFallback(t *testing.T) {
	r := ticktick.FocusRecord{StartTime: "bad", EndTime: "bad"}
	if ticktick.RecordDuration(r) != time.Duration(ticktick.StandardPomoMinutes)*time.Minute {
		t.Fatalf("unexpected fallback duration: %s", ticktick.RecordDuration(r))
	}
}

func TestAggregateTaskFocusByID(t *testing.T) {
	recs := []ticktick.FocusRecord{
		{
			StartTime: "2026-09-01T11:30:00.000+0200",
			EndTime:   "2026-09-01T11:55:00.000+0200",
			Tasks: []struct {
				TaskID      string `json:"taskId"`
				Title       string `json:"title"`
				ProjectName string `json:"projectName"`
				StartTime   string `json:"startTime"`
				EndTime     string `json:"endTime"`
			}{{TaskID: "t1", Title: "typ (apple keyboard)"}},
		},
		{
			StartTime: "2026-09-01T12:47:00.000+0200",
			EndTime:   "2026-09-01T13:12:00.000+0200",
			Tasks: []struct {
				TaskID      string `json:"taskId"`
				Title       string `json:"title"`
				ProjectName string `json:"projectName"`
				StartTime   string `json:"startTime"`
				EndTime     string `json:"endTime"`
			}{{TaskID: "t1", Title: "typ (apple keyboard)"}},
		},
		{
			StartTime: "2026-09-01T13:12:00.000+0200",
			EndTime:   "2026-09-01T13:17:00.000+0200",
			Tasks: []struct {
				TaskID      string `json:"taskId"`
				Title       string `json:"title"`
				ProjectName string `json:"projectName"`
				StartTime   string `json:"startTime"`
				EndTime     string `json:"endTime"`
			}{{TaskID: "t1", Title: ticktick.UnclaimedTitlePrefix + "typ (apple keyboard)"}},
		},
	}
	idx := ticktick.AggregateTaskFocus(recs)
	s := idx.ByID["t1"]
	if s.FullSessions != 2 {
		t.Fatalf("full=%d want 2", s.FullSessions)
	}
	if s.LoggedSessions != 2 {
		t.Fatalf("logged=%d want 2 (unclaimed excluded)", s.LoggedSessions)
	}
	if s.TotalSeconds < 3000 {
		t.Fatalf("seconds=%d too low", s.TotalSeconds)
	}
}

func TestAggregateTaskFocusByTitleFallback(t *testing.T) {
	recs := []ticktick.FocusRecord{
		{
			StartTime: "2026-09-01T11:30:00.000+0200",
			EndTime:   "2026-09-01T11:55:00.000+0200",
			Tasks: []struct {
				TaskID      string `json:"taskId"`
				Title       string `json:"title"`
				ProjectName string `json:"projectName"`
				StartTime   string `json:"startTime"`
				EndTime     string `json:"endTime"`
			}{{Title: "Manual entry"}},
		},
	}
	idx := ticktick.AggregateTaskFocus(recs)
	if idx.ByTitle["manual entry"].FullSessions != 1 {
		t.Fatal("expected title fallback aggregation")
	}
}
