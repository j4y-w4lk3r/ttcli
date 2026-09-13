package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestDisplayTextStripsVariationSelector(t *testing.T) {
	if got := displayText("Haircut\uFE0F"); got != "Haircut" {
		t.Fatalf("got %q", got)
	}
	if got := displayText("Culligan Water \U0001F4A7"); !strings.Contains(got, "💧") {
		t.Fatalf("emoji preserved: %q", got)
	}
}

func TestHaircutInFullInboxPane(t *testing.T) {
	tasks := make([]ticktick.Task, 0, 30)
	for i := 0; i < 22; i++ {
		tasks = append(tasks, ticktick.Task{
			ID: fmtID(i), Title: "filler", DueDate: "2026-07-01",
		})
	}
	tasks = append(tasks, ticktick.Task{
		ID: "hair", Title: "Haircut\uFE0F", DueDate: "2026-08-01T07:30:00",
	})
	tasks = append(tasks, ticktick.Task{
		ID: "dis", Title: "add the Disney + to the fm", DueDate: "2026-08-05",
	})

	m := fixtureModel(200, 50)
	m.projectName = "Inbox"
	m.tasks = tasks
	m.paneFocus = paneTasks
	m.taskCursor = 22

	l := m.layout()
	rightPane := renderPane(m.renderTasks(l), l, l.rightBoxW, true)
	plainRows := strings.Split(rightPane, "\n")
	for i, row := range plainRows {
		if lipgloss.Width(row) != l.rightBoxW {
			t.Fatalf("pane row %d width=%d want %d: %q", i, lipgloss.Width(row), l.rightBoxW, previewLine(row, 80))
		}
	}

	view := m.View()
	r := analyzeView(view, m.width, m.height, "tasks")
	if !r.OK() {
		t.Fatalf("frame broken:\n%s", r)
	}
	// Haircut row must not be followed by a blank inner line before Disney in the pane body.
	body := stripANSI(view)
	if strings.Contains(body, "Haircut") && strings.Contains(body, "Disney") {
		if strings.Contains(body, "Haircut\uFE0F") {
			t.Fatal("variation selector leaked into view")
		}
	}
}

func fmtID(i int) string {
	return "t" + string(rune('a'+i%26))
}

func TestHaircutTitleWithNewline(t *testing.T) {
	for _, title := range []string{
		"Haircut\uFE0F\n",
		"Haircut\uFE0F\r",
		"Haircut\uFE0F\r\n",
		"Haircut \U0001F487",
		"Haircut \U0001F487\uFE0F",
	} {
		m := fixtureModel(200, 50)
		m.tasks = []ticktick.Task{{ID: "h", Title: title, DueDate: "2026-08-01T07:30:00"}}
		m.projectName = "Inbox"
		m.paneFocus = paneTasks
		view := m.View()
		r := analyzeView(view, m.width, m.height, "tasks")
		if !r.OK() {
			t.Fatalf("title %q broke frame:\n%s", title, r)
		}
	}
}
