package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ai"
	"github.com/j4y-w4lk3r/ttcli/internal/secrets"
	"github.com/j4y-w4lk3r/ttcli/internal/tasktext"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type askState struct {
	input         textinput.Model
	taskID        string
	originalTitle string
	originalNotes string
	proposedTitle string
	proposedNotes string
	busy          bool
}

type aiRewriteMsg struct {
	taskID string
	title  string
	notes  string
	err    error
}

type askAppliedMsg struct {
	title string
	err   error
}

func newAskInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = "what should change?"
	input.CharLimit = 400
	input.Prompt = "i "
	input.PromptStyle = inputPromptStyle
	input.TextStyle = inputStyle
	return input
}

func (m model) beginAskAI() (model, tea.Cmd) {
	if m.paneFocus != paneTasks {
		return m, nil
	}
	if m.effectiveTaskScope() == TaskScopeArchive {
		m.errMsg = "archive records stay as snapshots"
		return m, nil
	}
	task, ok := m.selectedTask()
	if !ok {
		m.errMsg = "select a task"
		return m, nil
	}
	if task.Trashed() {
		m.errMsg = "task is in trash"
		return m, nil
	}
	m.mode = modeAskAI
	m.ask.taskID = task.ID
	m.ask.originalTitle = task.Title
	m.ask.originalNotes = tasktext.Strip(task.Content)
	m.ask.proposedTitle = ""
	m.ask.proposedNotes = ""
	m.ask.busy = false
	m.ask.input.SetValue("")
	m.ask.input.Focus()
	return m, textinput.Blink
}

func (m model) updateAskAI(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.mode == modeAskAIPreview {
		return m.updateAskPreview(msg)
	}
	switch msg.String() {
	case "esc":
		return m.clearAskAI(), nil
	case "enter":
		if m.ask.busy {
			return m, nil
		}
		instruction := strings.TrimSpace(m.ask.input.Value())
		if instruction == "" {
			m.errMsg = "say what should change"
			return m, nil
		}
		m.ask.busy = true
		m.toast = "asking…"
		return m, askRewriteCmd(m.ask.taskID, m.ask.originalTitle, m.ask.originalNotes, instruction)
	}
	var cmd tea.Cmd
	m.ask.input, cmd = m.ask.input.Update(msg)
	return m, cmd
}

func (m model) updateAskPreview(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return m.clearAskAI(), nil
	case "r":
		m.mode = modeAskAI
		m.ask.busy = false
		m.ask.input.Focus()
		return m, textinput.Blink
	case "enter":
		if m.client == nil {
			m.errMsg = "not connected"
			return m, nil
		}
		m.toast = "saving…"
		return m, askApplyCmd(m.client, m.ask.taskID, m.ask.proposedTitle, m.ask.proposedNotes)
	}
	return m, nil
}

func (m model) clearAskAI() model {
	m.mode = modeNormal
	m.ask.busy = false
	m.ask.input.Blur()
	m.ask.input.SetValue("")
	return m
}

func askRewriteCmd(taskID, title, notes, instruction string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := ai.Load(secrets.Default().GetLogin)
		if err != nil {
			return aiRewriteMsg{taskID: taskID, err: err}
		}
		nextTitle, nextNotes, err := ai.Rewrite(context.Background(), cfg, title, notes, instruction)
		return aiRewriteMsg{taskID: taskID, title: nextTitle, notes: nextNotes, err: err}
	}
}

func askApplyCmd(c *ticktick.Client, taskID, title, notes string) tea.Cmd {
	return func() tea.Msg {
		html := tasktext.ToHTML(notes)
		err := c.EditTask(taskID, ticktick.TaskEdit{Title: &title, Content: &html})
		return askAppliedMsg{title: title, err: err}
	}
}

func (m model) renderAskAI(contentW, innerLines int) string {
	title := headerStyle.Render(iconEdit + "  Rewrite " + m.ask.originalTitle)
	hint := "enter ask · esc cancel"
	var lines []string
	switch {
	case m.mode == modeAskAIPreview:
		hint = "enter save · r revise · esc cancel"
		lines = append(lines, hintStyle.Render("title"))
		lines = append(lines, "  "+m.ask.originalTitle)
		lines = append(lines, "  → "+m.ask.proposedTitle)
		lines = append(lines, "")
		lines = append(lines, hintStyle.Render("notes"))
		lines = append(lines, previewNoteLines(m.ask.originalNotes)...)
		lines = append(lines, "  →")
		lines = append(lines, previewNoteLines(m.ask.proposedNotes)...)
	case m.ask.busy:
		hint = "esc cancel"
		lines = append(lines, m.ask.input.View())
		lines = append(lines, hintStyle.Render("asking…"))
	default:
		lines = append(lines, m.ask.input.View())
	}
	if contentW > 0 {
		for i, line := range lines {
			lines[i] = truncateInner(line, contentW)
		}
		title = truncateInner(title, contentW)
	}
	return fillInner(title, truncateInner(hintStyle.Render(hint), contentW), lines, innerLines)
}

func previewNoteLines(notes string) []string {
	notes = strings.TrimSpace(notes)
	if notes == "" {
		return []string{"  (no notes)"}
	}
	parts := strings.Split(notes, "\n")
	if len(parts) > 8 {
		parts = append(parts[:8], "…")
	}
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		out = append(out, "  "+part)
	}
	return out
}

func (m model) handleAskMsg(msg tea.Msg) (model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case aiRewriteMsg:
		if m.mode != modeAskAI || msg.taskID != m.ask.taskID {
			return m, nil, true
		}
		m.ask.busy = false
		m.toast = ""
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil, true
		}
		m.ask.proposedTitle = msg.title
		m.ask.proposedNotes = msg.notes
		m.mode = modeAskAIPreview
		m.ask.input.Blur()
		return m, nil, true
	case askAppliedMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			m.toast = ""
			return m, nil, true
		}
		m = m.clearAskAI()
		if m.repo != nil {
			m.repo.InvalidateTasks(m.projectID)
		}
		m.toast = fmt.Sprintf("%s updated %q", iconCheck, msg.title)
		return m, m.reloadCurrentTasks(false), true
	default:
		return m, nil, false
	}
}
