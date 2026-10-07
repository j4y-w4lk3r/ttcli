package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestAskAIOpensAPromptAndPreviewsBeforeSaving(t *testing.T) {
	m := fixtureModel(80, 24)
	m.view = viewTasks
	m.paneFocus = paneTasks
	m.tasks = []ticktick.Task{{ID: "task-1", Title: "wake up", Content: "lights"}}
	m.taskCursor = 0

	opened := pressKey(m, "i")
	if opened.mode != modeAskAI || opened.ask.taskID != "task-1" || opened.ask.originalNotes != "lights" {
		t.Fatalf("mode=%v id=%s notes=%q", opened.mode, opened.ask.taskID, opened.ask.originalNotes)
	}

	cancelled, _ := opened.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cancelled.(model).mode != modeNormal {
		t.Fatal("esc should leave the prompt")
	}

	preview, _ := opened.Update(aiRewriteMsg{taskID: "task-1", title: "Wake", notes: "lights on"})
	got := preview.(model)
	if got.mode != modeAskAIPreview || got.ask.proposedTitle != "Wake" || got.ask.proposedNotes != "lights on" {
		t.Fatalf("preview mode=%v title=%q notes=%q", got.mode, got.ask.proposedTitle, got.ask.proposedNotes)
	}
	view := stripANSI(got.renderAskAI(60, 20))
	if !strings.Contains(view, "wake up") || !strings.Contains(view, "Wake") || !strings.Contains(view, "lights on") {
		t.Fatalf("preview view:\n%s", view)
	}

	revised, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if revised.(model).mode != modeAskAI {
		t.Fatal("r should return to the instruction")
	}
}
