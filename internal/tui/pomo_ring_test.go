package tui

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestAggregateFocusByTaskStableColors(t *testing.T) {
	records := []ticktick.FocusRecord{
		{StartTime: "2026-08-31T10:00:00+00:00", EndTime: "2026-08-31T10:25:00+00:00", Tasks: []struct {
			TaskID      string `json:"taskId"`
			Title       string `json:"title"`
			ProjectName string `json:"projectName"`
			StartTime   string `json:"startTime"`
			EndTime     string `json:"endTime"`
		}{{Title: "Alpha"}}},
		{StartTime: "2026-08-31T11:00:00+00:00", EndTime: "2026-08-31T11:25:00+00:00", Tasks: []struct {
			TaskID      string `json:"taskId"`
			Title       string `json:"title"`
			ProjectName string `json:"projectName"`
			StartTime   string `json:"startTime"`
			EndTime     string `json:"endTime"`
		}{{Title: "Beta"}}},
		{StartTime: "2026-08-31T12:00:00+00:00", EndTime: "2026-08-31T12:25:00+00:00", Tasks: []struct {
			TaskID      string `json:"taskId"`
			Title       string `json:"title"`
			ProjectName string `json:"projectName"`
			StartTime   string `json:"startTime"`
			EndTime     string `json:"endTime"`
		}{{Title: "Gamma"}}},
	}
	a := aggregateFocusByTask(records)
	b := aggregateFocusByTask(records)
	if len(a) != len(b) {
		t.Fatalf("len mismatch")
	}
	colors := map[string]lipgloss.Color{}
	for _, s := range a {
		colors[s.Title] = s.Color
	}
	for _, s := range b {
		if colors[s.Title] != s.Color {
			t.Fatalf("unstable color for %q", s.Title)
		}
	}
}

func TestPomoTimelineContentWUsesFullPanel(t *testing.T) {
	if got := pomoTimelineContentW(80); got != 80 {
		t.Fatalf("contentW=%d want 80", got)
	}
	if got := pomoTimelineContentW(40); got != 40 {
		t.Fatalf("narrow panel should use full width, got %d", got)
	}
}
