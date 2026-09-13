package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	focusPickerMinMinutes = 1
	focusPickerMaxMinutes = 180
	focusPickerBoxMaxW    = 64
)

var focusPickerPresets = []int{5, 15, 25, 45, 60}

func filterFocusPickerTasks(tasks []ticktick.Task) []ticktick.Task {
	out := tasks[:0]
	for _, t := range tasks {
		if t.IsSubtask() || t.Done() {
			continue
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func filterFocusPickerByQuery(tasks []ticktick.Task, query string, projectNames map[string]string) []ticktick.Task {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return tasks
	}
	tokens := strings.Fields(q)
	var out []ticktick.Task
	for _, t := range tasks {
		hay := strings.ToLower(t.Title)
		if name := projectNames[t.ProjectID]; name != "" {
			hay += " " + strings.ToLower(name)
		}
		ok := true
		for _, tok := range tokens {
			if !strings.Contains(hay, tok) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, t)
		}
	}
	return out
}

func (m model) focusPickerVisibleTasks() []ticktick.Task {
	visible := filterFocusPickerByQuery(m.focusPickerTasks, m.focusPickerFilter, m.focusPickerProjectNames)
	if m.focusPickerSwitch {
		visible = excludeFocusPickerTask(visible, m.focusPickerActiveTaskID())
	}
	return visible
}

func excludeFocusPickerTask(tasks []ticktick.Task, taskID string) []ticktick.Task {
	if taskID == "" {
		return tasks
	}
	out := tasks[:0]
	for _, t := range tasks {
		if t.ID != taskID {
			out = append(out, t)
		}
	}
	return out
}

func (m model) focusPickerActiveTaskID() string {
	sess, err := focus.Load()
	if err != nil || !sess.Active() {
		return ""
	}
	return sess.TaskID
}

func (m model) focusPickerInitialCursor(tasks []ticktick.Task) int {
	visible := filterFocusPickerByQuery(tasks, m.focusPickerFilter, m.focusPickerProjectNames)
	if len(visible) == 0 {
		return 0
	}
	if t, ok := m.selectedTask(); ok {
		for i, task := range visible {
			if task.ID == t.ID {
				return i
			}
		}
	}
	if r, ok := m.selectedPomoRecord(); ok && len(r.Tasks) > 0 {
		tid := r.Tasks[0].TaskID
		for i, task := range visible {
			if task.ID == tid {
				return i
			}
		}
	}
	return 0
}

func (m model) selectedFocusPickerTask() (ticktick.Task, bool) {
	visible := m.focusPickerVisibleTasks()
	if m.focusPickerCursor < 0 || m.focusPickerCursor >= len(visible) {
		return ticktick.Task{}, false
	}
	return visible[m.focusPickerCursor], true
}

func focusPickerLayout(termW, termH int) (boxW, innerW, listH int) {
	boxW = termW - 4
	if boxW > focusPickerBoxMaxW {
		boxW = focusPickerBoxMaxW
	}
	if boxW < 36 {
		boxW = termW - 2
	}
	if boxW < 20 {
		boxW = 20
	}
	innerW = boxW - 4
	if innerW < 16 {
		innerW = 16
	}
	listH = termH - 14
	if listH > 12 {
		listH = 12
	}
	if listH < 4 {
		listH = 4
	}
	return boxW, innerW, listH
}

func (m model) renderFocusPickerOverlay() string {
	boxW, innerW, listH := focusPickerLayout(m.width, m.height)

	var lines []string
	if m.focusPickerSwitch {
		lines = append(lines, helpInnerLine(centerStyledText(timerBigStyle.Render("Switch task"), innerW), innerW))
		lines = append(lines, helpInnerLine(centerStyledText(hintStyle.Render("timer keeps running"), innerW), innerW))
	} else {
		for _, ln := range m.renderFocusPickerDurationBlock(innerW) {
			lines = append(lines, ln)
		}
	}
	lines = append(lines, helpInnerLine(focusPickerDivider(innerW), innerW))

	searchRows := 0
	if q := strings.TrimSpace(m.focusPickerFilter); q != "" {
		lines = append(lines, helpInnerLine(helpSearchStyle.Render("search: "+q), innerW))
		searchRows = 1
	}
	taskListH := listH - searchRows
	if taskListH < 1 {
		taskListH = 1
	}

	visible := m.focusPickerVisibleTasks()
	if m.focusPickerLoading {
		lines = append(lines, helpInnerLine(hintStyle.Render("loading tasks…"), innerW))
		for i := 1; i < taskListH; i++ {
			lines = append(lines, helpInnerLine("", innerW))
		}
	} else if len(m.focusPickerTasks) == 0 {
		lines = append(lines, helpInnerLine(hintStyle.Render("(no open tasks)"), innerW))
		for i := 1; i < taskListH; i++ {
			lines = append(lines, helpInnerLine("", innerW))
		}
	} else if len(visible) == 0 {
		lines = append(lines, helpInnerLine(hintStyle.Render("(no matching tasks)"), innerW))
		for i := 1; i < taskListH; i++ {
			lines = append(lines, helpInnerLine("", innerW))
		}
	} else {
		win := computeScrollWindow(m.focusPickerCursor, len(visible), taskListH)
		for row := 0; row < taskListH; row++ {
			idx := win.Start + row
			if idx < win.End && idx < len(visible) {
				t := visible[idx]
				name := m.focusPickerProjectNames[t.ProjectID]
				lines = append(lines, renderFocusPickerTaskLine(t, name, idx == m.focusPickerCursor, innerW, m.focusPickerFilter))
			} else {
				lines = append(lines, helpInnerLine("", innerW))
			}
		}
	}

	hint := "type filter · j/k task · t type minutes · [ ] ±5m · enter start · esc cancel"
	if m.focusPickerSwitch {
		hint = "type filter · j/k task · enter switch · esc cancel"
	}
	if h := m.keyHint(hint); h != "" {
		lines = append(lines, helpInnerLine("", innerW))
		lines = append(lines, helpInnerLine(h, innerW))
	}

	box := helpBoxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
	return centerBoxOnPlainScreen(box, m.width, m.height)
}

func focusPickerDivider(innerW int) string {
	ruleLen := innerW
	if ruleLen > 48 {
		ruleLen = 48
	}
	if ruleLen < 8 {
		ruleLen = 8
	}
	pad := max(0, (innerW-ruleLen)/2)
	return strings.Repeat(" ", pad) + sectionRuleStyle.Render(strings.Repeat("─", ruleLen))
}

func (m model) renderFocusPickerDurationBlock(innerW int) []string {
	display := m.focusPickerDurationDisplay()
	centered := centerStyledText(display, innerW)
	lines := []string{helpInnerLine(centered, innerW)}
	lines = append(lines, helpInnerLine(renderFocusPickerPresetRow(m.focusPickerMinutes, m.focusPickerEditDuration, innerW), innerW))
	return lines
}

func (m model) focusPickerDurationDisplay() string {
	if m.focusPickerEditDuration {
		buf := m.focusPickerDurationBuf
		if buf == "" {
			return timerBigStyle.Render("▏m")
		}
		return timerBigStyle.Render(buf + "▏m")
	}
	return timerBigStyle.Render(formatFocusPickerDuration(m.focusPickerMinutes))
}

func centerStyledText(s string, width int) string {
	pad := max(0, (width-lipgloss.Width(s))/2)
	return strings.Repeat(" ", pad) + s
}

func renderFocusPickerPresetRow(selected int, editing bool, innerW int) string {
	var chips []string
	for _, preset := range focusPickerPresets {
		chips = append(chips, focusPickerPresetChip(preset, preset == selected && !editing))
	}
	row := strings.Join(chips, " ")
	if lipgloss.Width(row) > innerW {
		row = truncateRenderedWidth(row, innerW)
	}
	return centerStyledText(row, innerW)
}

func focusPickerPresetChip(mins int, active bool) string {
	label := fmt.Sprintf("%dm", mins)
	if active {
		return lipgloss.NewStyle().
			Foreground(colorMauve).
			Bold(true).
			Padding(0, 1).
			Render(label)
	}
	return lipgloss.NewStyle().
		Foreground(colorSubtext).
		Padding(0, 1).
		Render(label)
}

func renderFocusPickerTaskLine(task ticktick.Task, projectName string, selected bool, innerW int, query string) string {
	marker := "  "
	taskIcon := iconTaskOpen
	titleStyle := helpIdleStyle
	if selected {
		marker = "▸ "
		taskIcon = iconTaskSel
		titleStyle = focusPickerSelStyle
	}

	title := task.Title
	if title == "" {
		title = "(untitled)"
	}

	projPlain := ""
	if projectName != "" {
		projPlain = " · " + projectName
	}
	projW := lipgloss.Width(projPlain)
	prefixW := lipgloss.Width(marker+taskIcon) + 1
	titleW := innerW - prefixW - projW
	if titleW < 6 {
		titleW = 6
	}
	titlePlain := title
	if lipgloss.Width(titlePlain) > titleW {
		titlePlain = truncateRunes(titlePlain, max(1, titleW-1)) + "…"
	}
	titleRendered := highlightHelpMatch(titlePlain, query, titleStyle)

	line := helpMarkerStyle.Render(marker+taskIcon) +
		helpMarkerStyle.Render(" ") +
		titleRendered +
		highlightHelpMatch(projPlain, query, hintStyle)
	return fillBarRow(line, innerW, rowPadStyle)
}

func formatFocusPickerDuration(mins int) string {
	if mins >= 60 {
		return formatFocusTotal(mins * 60)
	}
	return fmt.Sprintf("%dm", mins)
}

func clampFocusMinutes(mins int) int {
	if mins < focusPickerMinMinutes {
		return focusPickerMinMinutes
	}
	if mins > focusPickerMaxMinutes {
		return focusPickerMaxMinutes
	}
	return mins
}

func parseFocusDurationInput(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return clampFocusMinutes(v), true
}
