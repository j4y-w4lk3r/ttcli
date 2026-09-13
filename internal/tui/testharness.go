package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

// viewFrameReport captures dimensional analysis of a rendered View().
type viewFrameReport struct {
	TermW, TermH int
	Lines        []string
	WidthErrors  []string
	HeightError  string
	BorderErrors []string
	HeaderErrors []string
}

func (r viewFrameReport) OK() bool {
	return len(r.WidthErrors) == 0 && r.HeightError == "" &&
		len(r.BorderErrors) == 0 && len(r.HeaderErrors) == 0
}

func (r viewFrameReport) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "terminal %dx%d, %d lines\n", r.TermW, r.TermH, len(r.Lines))
	for _, e := range r.WidthErrors {
		fmt.Fprintf(&b, "  WIDTH: %s\n", e)
	}
	if r.HeightError != "" {
		fmt.Fprintf(&b, "  HEIGHT: %s\n", r.HeightError)
	}
	for _, e := range r.BorderErrors {
		fmt.Fprintf(&b, "  BORDER: %s\n", e)
	}
	for _, e := range r.HeaderErrors {
		fmt.Fprintf(&b, "  HEADER: %s\n", e)
	}
	return b.String()
}

// analyzeView checks every row fits the terminal and basic frame invariants hold.
func analyzeView(view string, termW, termH int, viewName string) viewFrameReport {
	r := viewFrameReport{TermW: termW, TermH: termH}
	view = strings.TrimSuffix(view, "\n")
	if view == "" {
		r.Lines = []string{""}
	} else {
		r.Lines = strings.Split(view, "\n")
	}

	if len(r.Lines) != termH {
		r.HeightError = fmt.Sprintf("got %d lines, want %d", len(r.Lines), termH)
	}

	for i, line := range r.Lines {
		w := lipgloss.Width(line)
		if w != termW {
			r.WidthErrors = append(r.WidthErrors,
				fmt.Sprintf("line %d: lipgloss.Width=%d want %d preview=%q", i, w, termW, previewLine(line, 60)))
		}
		if strings.Contains(line, "\n") {
			r.WidthErrors = append(r.WidthErrors, fmt.Sprintf("line %d: contains embedded newline", i))
		}
	}

	if len(r.Lines) >= 2 {
		plain0 := stripANSI(r.Lines[0])
		if !strings.Contains(plain0, "TickTick") {
			r.HeaderErrors = append(r.HeaderErrors, "title row missing brand")
		}
		if !strings.Contains(plain0, "/") && !strings.Contains(stripANSI(r.Lines[0]), fmt.Sprint(dailyPomoGoal)) {
			// pomo badge should be on title row (line 0)
			r.HeaderErrors = append(r.HeaderErrors, "title row missing pomo badge")
		}
		if !strings.Contains(stripANSI(r.Lines[1]), "Tasks") {
			r.HeaderErrors = append(r.HeaderErrors, "nav row missing Tasks tab")
		}
		if strings.Contains(r.Lines[1], "╭") || strings.Contains(r.Lines[1], "╮") {
			r.HeaderErrors = append(r.HeaderErrors, "nav row contains box border (merged with body?)")
		}
	}

	if viewName == "tasks" && len(r.Lines) > headerRows+footerRows {
		bodyStart := headerRows
		bodyEnd := len(r.Lines) - footerRows
		for i := bodyStart; i < bodyEnd; i++ {
			line := r.Lines[i]
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "╭") && lastRune(line) != '╮' {
				r.BorderErrors = append(r.BorderErrors, fmt.Sprintf("body line %d: top border missing ╮", i))
			}
			if strings.HasPrefix(line, "│") && lastRune(line) != '│' {
				r.BorderErrors = append(r.BorderErrors, fmt.Sprintf("body line %d: side border missing right │", i))
			}
			if strings.HasPrefix(line, "╰") && lastRune(line) != '╯' {
				r.BorderErrors = append(r.BorderErrors, fmt.Sprintf("body line %d: bottom border missing ╯", i))
			}
		}
	}

	return r
}

func previewLine(s string, max int) string {
	plain := stripANSI(s)
	if len(plain) <= max {
		return plain
	}
	return plain[:max] + "…"
}

func stripANSI(s string) string {
	var b strings.Builder
	esc := false
	for i := 0; i < len(s); i++ {
		if esc {
			if s[i] == 'm' {
				esc = false
			}
			continue
		}
		if s[i] == '\x1b' {
			esc = true
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// fixtureModel builds a model pre-loaded with realistic tree/tasks/calendar data.
func fixtureModel(termW, termH int) model {
	groups := []ticktick.ProjectGroup{
		{ID: "g1", Name: "X", SortOrder: 1},
		{ID: "g2", Name: "Y", SortOrder: 2},
	}
	projects := []ticktick.Project{
		{ID: "p0", Name: "List0", GroupID: "NONE", Kind: "TASK"},
		{ID: "p1", Name: "Tech", GroupID: "g1", Kind: "TASK"},
		{ID: "p2", Name: "PXC", GroupID: "g1", Kind: "TASK"},
		{ID: "p3", Name: "Company", GroupID: "g2", Kind: "TASK"},
		{ID: "p4", Name: "Personal Care", GroupID: "g2", Kind: "TASK"},
	}
	tree := ticktick.ProjectTree(groups, projects)
	listRows := buildListRows(tree)

	tasks := make([]ticktick.Task, 0, 20)
	for i := 0; i < 20; i++ {
		tasks = append(tasks, ticktick.Task{
			ID:        fmt.Sprintf("t%d", i),
			ProjectID: "p0",
			Title:     fmt.Sprintf("task item number %d with a longer title", i),
			DueDate:   "2026-07-03",
			Status:    0,
		})
	}

	calTasks := make([]ticktick.Task, 0, 50)
	for i := 0; i < 50; i++ {
		calTasks = append(calTasks, ticktick.Task{
			ID:      fmt.Sprintf("c%d", i),
			Title:   fmt.Sprintf("calendar due task %d", i),
			DueDate: "2026-08-30",
			Status:  0,
		})
	}

	m := newModel(nil)
	m.width = termW
	m.height = termH
	m.loading = false
	m.view = viewTasks
	m.tree = tree
	m.listRows = listRows
	m.listCursor = 0
	m.projectID = "p0"
	m.projectName = "List0"
	m.tasks = tasks
	m.taskCursor = 0
	m.calTasks = calTasks
	m.todayPomos = 3
	return m
}

func applyWindowSize(m model, w, h int) model {
	m.width = w
	m.height = h
	out, cmd := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	_ = cmd
	return out.(model)
}

func switchToView(m model, v appView) model {
	out, _ := m.switchView(v)
	switch v {
	case viewCalendar:
		out.loading = false
		out.calTasks = m.calTasks
	case viewPomodoro:
		out.loading = false
	case viewHabits:
		out.loading = false
	case viewTasks:
		out.loading = false
	}
	return out
}

func pressKey(m model, s string) model {
	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	_ = cmd
	return out.(model)
}

func viewName(v appView) string {
	switch v {
	case viewCalendar:
		return "calendar"
	case viewPomodoro:
		return "pomo"
	case viewHabits:
		return "habits"
	default:
		return "tasks"
	}
}

func assertViewOK(t *testing.T, m model, label string) {
	t.Helper()
	view := m.View()
	if view == "loading…" {
		t.Fatalf("%s: view still loading", label)
	}
	r := analyzeView(view, m.width, m.height, viewName(m.view))
	if !r.OK() {
		t.Fatalf("%s:\n%s", label, r)
	}
}

func dumpView(t *testing.T, m model, label string) {
	t.Helper()
	view := m.View()
	r := analyzeView(view, m.width, m.height, viewName(m.view))
	t.Logf("=== %s (%dx%d) ===\n%s", label, m.width, m.height, r)
	for i, line := range r.Lines {
		t.Logf("%3d|%3d| %s", i, lipgloss.Width(line), previewLine(line, 100))
	}
}
