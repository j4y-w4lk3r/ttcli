package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestFilterFocusPickerByQuery(t *testing.T) {
	tasks := []ticktick.Task{
		{ID: "1", Title: "Write report", ProjectID: "p1"},
		{ID: "2", Title: "Buy milk", ProjectID: "p2"},
		{ID: "3", Title: "Report bugs", ProjectID: "p1"},
	}
	names := map[string]string{"p1": "Work", "p2": "Home"}

	got := filterFocusPickerByQuery(tasks, "report", names)
	if len(got) != 2 {
		t.Fatalf("want 2 matches, got %d", len(got))
	}

	got = filterFocusPickerByQuery(tasks, "write work", names)
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("multi-token filter: got %+v", got)
	}

	got = filterFocusPickerByQuery(tasks, "home", names)
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("list name filter: got %+v", got)
	}
}

func TestFocusPickerTypeFilter(t *testing.T) {
	m := fixtureModel(80, 24)
	m.mode = modeFocusPicker
	m.focusPickerMinutes = 25
	m.focusPickerTasks = []ticktick.Task{
		{ID: "1", Title: "Alpha task"},
		{ID: "2", Title: "Beta task"},
		{ID: "3", Title: "Alpine"},
	}

	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	visible := m.focusPickerVisibleTasks()
	if len(visible) != 2 {
		t.Fatalf("filter alp: got %d tasks", len(visible))
	}
	if m.focusPickerCursor != 0 {
		t.Fatalf("cursor should reset to 0, got %d", m.focusPickerCursor)
	}

	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if m.focusPickerFilter != "alpt" {
		t.Fatalf("t should append while filtering, got %q", m.focusPickerFilter)
	}

	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyCtrlU})
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !m.focusPickerEditDuration {
		t.Fatal("t with empty filter should open duration edit")
	}
}

func TestFilterFocusPickerTasks(t *testing.T) {
	tasks := filterFocusPickerTasks([]ticktick.Task{
		{ID: "1", Title: "B", Status: 0},
		{ID: "2", Title: "A", Status: 2},
		{ID: "3", Title: "Child", ParentID: "1"},
	})
	if len(tasks) != 1 {
		t.Fatalf("want 1 open top-level task, got %d", len(tasks))
	}
	if tasks[0].Title != "B" {
		t.Fatalf("got %q", tasks[0].Title)
	}
}

func TestClampFocusMinutes(t *testing.T) {
	if clampFocusMinutes(0) != 1 {
		t.Fatal("min clamp")
	}
	if clampFocusMinutes(999) != 180 {
		t.Fatal("max clamp")
	}
}

func TestParseFocusDurationInput(t *testing.T) {
	v, ok := parseFocusDurationInput("45")
	if !ok || v != 45 {
		t.Fatalf("got %d ok=%v", v, ok)
	}
	v, ok = parseFocusDurationInput("")
	if ok {
		t.Fatal("empty should fail")
	}
	v, ok = parseFocusDurationInput("999")
	if !ok || v != 180 {
		t.Fatalf("clamp got %d", v)
	}
}

func TestFocusPickerDurationTyping(t *testing.T) {
	m := fixtureModel(80, 24)
	m.mode = modeFocusPicker
	m.focusPickerMinutes = 25
	m.focusPickerTasks = []ticktick.Task{{ID: "1", Title: "Task"}}

	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	if !m.focusPickerEditDuration {
		t.Fatal("expected duration edit mode")
	}
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'5'}})
	m, _ = m.updateFocusPicker(tea.KeyMsg{Type: tea.KeyEnter})
	if m.focusPickerMinutes != 45 {
		t.Fatalf("minutes=%d want 45", m.focusPickerMinutes)
	}
	if m.focusPickerEditDuration {
		t.Fatal("should exit edit mode")
	}
}

func TestFocusPickerDurationDisplayEdit(t *testing.T) {
	m := fixtureModel(80, 24)
	m.focusPickerEditDuration = true
	m.focusPickerDurationBuf = "2"
	plain := strings.TrimSpace(stripANSI(m.focusPickerDurationDisplay()))
	if plain != "2▏m" {
		t.Fatalf("got %q want 2▏m", plain)
	}
	if strings.Contains(strings.TrimPrefix(plain, "2"), "  ") {
		t.Fatalf("unexpected gap in duration display: %q", plain)
	}
}

func TestFocusPickerNoTitleHeader(t *testing.T) {
	m := fixtureModel(80, 24)
	m.mode = modeFocusPicker
	m.focusPickerMinutes = 25
	m.focusPickerTasks = []ticktick.Task{{ID: "1", Title: "Task"}}
	out := stripANSI(m.renderFocusPickerOverlay())
	if strings.Contains(out, "Start focus") {
		t.Fatal("picker should not show Start focus header")
	}
	if strings.Contains(out, "pick a task") {
		t.Fatal("picker should not show pick a task hint")
	}
	if !strings.Contains(out, "25m") {
		t.Fatal("picker should show duration prominently")
	}
}

func TestFocusPickerPresetRow(t *testing.T) {
	row := stripANSI(renderFocusPickerPresetRow(25, false, 60))
	for _, p := range focusPickerPresets {
		if !strings.Contains(row, fmt.Sprintf("%dm", p)) {
			t.Fatalf("missing preset %dm in %q", p, row)
		}
	}
}

func TestFocusPickerFixedFrameSize(t *testing.T) {
	longTitle := strings.Repeat("Very long task name ", 15)
	m := fixtureModel(100, 30)
	m.mode = modeFocusPicker
	m.focusPickerTasks = []ticktick.Task{
		{ID: "1", Title: "Short"},
		{ID: "2", Title: longTitle, ProjectID: "p1"},
	}
	m.focusPickerProjectNames = map[string]string{"p1": "A very long project name that should truncate"}

	render := func(cursor int) []string {
		m.focusPickerCursor = cursor
		return strings.Split(m.renderFocusPickerOverlay(), "\n")
	}

	short := render(0)
	long := render(1)
	if len(short) != len(long) {
		t.Fatalf("row count %d vs %d", len(short), len(long))
	}
	for i := range short {
		w0 := lipgloss.Width(short[i])
		w1 := lipgloss.Width(long[i])
		if w0 != w1 {
			t.Fatalf("row %d width %d vs %d", i, w0, w1)
		}
		if w0 != 100 {
			t.Fatalf("row %d width=%d want 100", i, w0)
		}
	}

	// Every task slot row should be single-line (no wrap).
	_, innerW, listH := focusPickerLayout(100, 30)
	for row := 0; row < listH; row++ {
		line := renderFocusPickerTaskLine(m.focusPickerTasks[1], "PXC", true, innerW, "")
		if strings.Contains(stripANSI(line), "\n") {
			t.Fatal("task line wrapped")
		}
		if lipgloss.Width(line) > innerW {
			t.Fatalf("task line too wide: %d > %d", lipgloss.Width(line), innerW)
		}
	}
}

func TestFocusPickerOverlayUniformBackground(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(100, 30)
	m.mode = modeFocusPicker
	m.focusPickerMinutes = 25
	m.focusPickerTasks = []ticktick.Task{
		{ID: "1", Title: "Alpha", ProjectID: "p1"},
		{ID: "2", Title: "Beta", ProjectID: "p1"},
	}
	m.focusPickerProjectNames = map[string]string{"p1": "Inbox"}
	out := m.renderFocusPickerOverlay()
	lines := strings.Split(out, "\n")
	if len(lines) != 30 {
		t.Fatalf("rows=%d want 30", len(lines))
	}
	for i, line := range lines {
		if lipgloss.Width(line) != 100 {
			t.Fatalf("row %d width=%d want 100", i, lipgloss.Width(line))
		}
		plain := stripANSI(line)
		label := fmt.Sprintf("picker row %d", i)
		if strings.Contains(plain, "▒") || strings.Contains(plain, "░") {
			t.Fatalf("%s: block dim scrim should not be used", label)
		}
		assertNoHardcodedFillBackgrounds(t, line, nil, label)
	}
}
