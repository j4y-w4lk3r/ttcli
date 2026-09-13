package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func (m model) showTaskDetail() bool {
	if m.mode != modeNormal || m.paneFocus != paneTasks {
		return false
	}
	_, ok := m.selectedTask()
	return ok
}

func (m model) taskDetailLayoutPref() TaskDetailLayout {
	if m.uiSettings.TaskDetailLayout == TaskDetailSide {
		return TaskDetailSide
	}
	return TaskDetailBottom
}

func (m model) taskDetailUsesSide(l layout) bool {
	if !m.showTaskDetail() || m.taskDetailLayoutPref() != TaskDetailSide {
		return false
	}
	if l.narrow {
		return false
	}
	listW, _ := taskDetailSplitWidths(l.rightBoxW)
	return listW > 0
}

func (m model) taskDetailLineBudget(innerLines int) int {
	if !m.showTaskDetail() {
		return 0
	}
	if m.taskDetailLayoutPref() == TaskDetailSide {
		return m.taskDetailSideLineBudget(innerLines)
	}
	budget := innerLines / 2
	if budget < 8 {
		budget = 8
	}
	maxBudget := innerLines - 5
	if budget > maxBudget {
		budget = maxBudget
	}
	if budget < 4 {
		budget = 4
	}
	return budget
}

func (m model) taskDetailSideLineBudget(innerLines int) int {
	budget := innerLines - 2
	if budget < 6 {
		budget = 6
	}
	return budget
}

func (m model) taskListMaxRows(innerLines int) int {
	reserved := 0
	if m.showTaskDetail() {
		reserved = m.taskDetailLineBudget(innerLines)
	}
	return paneScrollRows(innerLines, reserved)
}

func taskDetailPanel(t ticktick.Task, focusFn func(ticktick.Task) (ticktick.TaskFocusSummary, bool), contentW, maxLines int) []string {
	return taskDetailPanelOpts(t, focusFn, contentW, maxLines, false)
}

func taskDetailPanelSide(t ticktick.Task, focusFn func(ticktick.Task) (ticktick.TaskFocusSummary, bool), contentW, maxLines int) []string {
	return taskDetailPanelOpts(t, focusFn, contentW, maxLines, true)
}

func taskDetailPanelOpts(t ticktick.Task, focusFn func(ticktick.Task) (ticktick.TaskFocusSummary, bool), contentW, maxLines int, sideLayout bool) []string {
	if maxLines < 2 {
		maxLines = 2
	}
	var body []string

	var meta []string
	if s, ok := focusFn(t); ok {
		meta = append(meta, renderTaskFocusDetail(s))
	}
	if p := t.PriorityLabel(); p != "-" {
		meta = append(meta, "priority "+p)
	}
	if d := formatDueDMY(t.DueDate); d != "" {
		meta = append(meta, "due "+d)
	}
	if s := formatDueDMY(t.StartDate); s != "" && s != formatDueDMY(t.DueDate) {
		meta = append(meta, "start "+s)
	}
	if len(t.Tags) > 0 {
		meta = append(meta, "tags "+strings.Join(t.Tags, ", "))
	}
	if t.Trashed() {
		meta = append(meta, "trashed")
	}
	if t.Done() {
		meta = append(meta, "completed")
	}
	if t.IsSubtask() {
		meta = append(meta, "subtask")
	}
	if len(meta) > 0 {
		body = append(body, hintStyle.Render("  "+strings.Join(meta, " · ")))
	}

	content := strings.TrimSpace(stripTaskHTML(t.Content))
	desc := strings.TrimSpace(stripTaskHTML(t.Desc))
	switch {
	case content != "":
		body = append(body, noteLabelStyle.Render("  Notes"))
		body = append(body, wrapStyledLines(content, contentW-2, noteStyle, "  ")...)
	case desc != "":
		body = append(body, noteLabelStyle.Render("  Description"))
		body = append(body, wrapStyledLines(desc, contentW-2, noteStyle, "  ")...)
	}

	if len(t.Items) > 0 {
		body = append(body, noteLabelStyle.Render("  Checklist"))
		for _, item := range t.Items {
			marker := iconTaskOpen
			style := noteStyle
			if item.Done() {
				marker = iconCheck
				style = taskDoneStyle
			}
			body = append(body, style.Render("  "+marker+" "+item.Title))
		}
	}

	if len(body) == 0 {
		body = append(body, hintStyle.Render("  (no notes)"))
	}

	out := make([]string, 0, len(body)+1)
	if !sideLayout {
		out = append(out, lipglossOverlayRule(contentW))
	}
	out = append(out, body...)
	if len(out) > maxLines {
		out = out[:maxLines-1]
		out = append(out, hintStyle.Render("  …"))
	}
	for i := range out {
		out[i] = truncateRenderedWidth(out[i], contentW)
	}
	return out
}

func lipglossOverlayRule(contentW int) string {
	return truncateRenderedWidth(
		sectionRuleStyle.Render(strings.Repeat("─", max(1, contentW-2))),
		contentW,
	)
}

func wrapStyledLines(text string, width int, style lipgloss.Style, prefix string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	plain := strings.TrimSpace(text)
	if plain == "" {
		return nil
	}
	prefixW := lipgloss.Width(prefix)
	lineW := width - prefixW
	if lineW < 8 {
		lineW = 8
	}
	words := strings.Fields(plain)
	if len(words) == 0 {
		return []string{truncateRenderedWidth(style.Render(prefix+plain), width)}
	}
	var lines []string
	var cur strings.Builder
	for _, w := range words {
		next := w
		if cur.Len() > 0 {
			next = cur.String() + " " + w
		}
		if lipgloss.Width(next) > lineW && cur.Len() > 0 {
			lines = append(lines, truncateRenderedWidth(style.Render(prefix+cur.String()), width))
			cur.Reset()
			cur.WriteString(w)
		} else {
			cur.Reset()
			cur.WriteString(next)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, truncateRenderedWidth(style.Render(prefix+cur.String()), width))
	}
	return lines
}

func stripTaskHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}
