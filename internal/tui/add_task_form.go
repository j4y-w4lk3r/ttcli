package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	addTaskFieldTitle = iota
	addTaskFieldNotes
	addTaskFieldDue
	addTaskFieldTime
	addTaskFieldDuration
	addTaskFieldFocus
	addTaskFieldRepeat
	addTaskFieldRepeatFrom
	addTaskFieldRepeatRule
	addTaskFieldReminder
	addTaskFieldPriority
	addTaskFieldCount
)

var (
	addTaskFieldLabels = []string{"Title", "Notes", "Due date", "Start time", "Duration", "Planned focus", "Repeat", "Repeat from", "Custom rule", "Reminder", "Priority"}
	addTaskFieldHints  = []string{
		"required",
		"optional · Enter for new line",
		"DD/MM/YYYY · empty = none",
		"HH:MM · empty = all day",
		"calendar block minutes · empty = none",
		"75m or 3p · native TickTick estimate",
		"none · daily · weekdays · weekly · custom",
		"due date · completion date",
		"RRULE:… or ERULE:…",
		"none · 0 · 5 · 15 · 30 · 60",
		"none · low · med · high",
	}
	addTaskRepeatOpts     = []string{"none", "daily", "weekdays", "weekly", "custom"}
	addTaskRepeatFromOpts = []struct {
		label string
		value int
	}{
		{"due date", ticktick.RepeatFromDue},
		{"completion date", ticktick.RepeatFromCompletion},
	}
	addTaskReminderOpts = []struct {
		label string
		mins  int // -1 = none, 0 = at due
	}{
		{"none", -1},
		{"at due", 0},
		{"5m before", 5},
		{"15m before", 15},
		{"30m before", 30},
		{"1h before", 60},
	}
	addTaskPriorityOpts = []struct {
		label string
		value int
	}{
		{"none", 0},
		{"low", 1},
		{"med", 3},
		{"high", 5},
	}
)

func newAddTaskFieldInput(placeholder string) textinput.Model {
	in := textinput.New()
	in.Placeholder = placeholder
	in.CharLimit = 120
	in.Prompt = "  "
	in.PromptStyle = inputPromptStyle
	in.TextStyle = inputStyle
	return in
}

func newAddTaskNotesInput() textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "notes…"
	ta.CharLimit = 4000
	ta.SetWidth(40)
	ta.SetHeight(4)
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = inputStyle
	ta.BlurredStyle.Base = inputStyle
	ta.Cursor.Style = inputStyle
	return ta
}

func (m *model) blurTaskFormInputs() {
	m.taskTitleInput.Blur()
	m.addTaskNotesInput.Blur()
	m.addTaskDueInput.Blur()
	m.addTaskTimeInput.Blur()
	m.addTaskDurationInput.Blur()
	m.addTaskFocusInput.Blur()
	m.addTaskRepeatInput.Blur()
}

func taskTitleInputView(in textinput.Model, contentW int) string {
	promptW := lipgloss.Width(in.Prompt)
	in.Width = max(contentW-promptW, 1)
	view := strings.ReplaceAll(in.View(), "\n", " ")
	return truncateInner(view, contentW)
}

func (m *model) openAddTaskForm() {
	m.mode = modeAddTask
	m.editTaskID = ""
	m.resetTaskFormFields()
}

func (m *model) openEditTaskForm(t ticktick.Task) {
	m.mode = modeEditTask
	m.editTaskID = t.ID
	m.resetTaskFormFields()
	m.taskTitleInput.SetValue(t.Title)
	m.taskTitleInput.Placeholder = ""
	m.taskTitleInput.CursorEnd()
	notes := t.Content
	if strings.TrimSpace(notes) == "" {
		notes = t.Desc
	}
	m.addTaskNotesInput.SetValue(stripTaskHTML(notes))
	m.addTaskNotesInput.CursorEnd()
	m.addTaskDueInput.SetValue(formatDueDMY(t.DueDate))
	if !t.IsAllDay && hasDueTime(t.StartDate) {
		if start, ok := parseDueTime(t.StartDate); ok {
			m.addTaskTimeInput.SetValue(start.Format("15:04"))
		}
	}
	if dur := taskDurationMinutes(t); dur != "" {
		m.addTaskDurationInput.SetValue(dur)
	}
	m.addTaskPriorityIdx = priorityOptionIndex(t.Priority.Int())
	m.addTaskReminderIdx = reminderOptionIndex(t)
	if seconds, pomos, ok := t.FocusEstimate(); ok {
		if seconds > 0 {
			m.addTaskFocusInput.SetValue(strconv.FormatInt((seconds+59)/60, 10) + "m")
		} else {
			m.addTaskFocusInput.SetValue(strconv.Itoa(pomos) + "p")
		}
	}
	m.addTaskRepeatIdx = repeatOptionIndex(t.RepeatFlag)
	m.addTaskRepeatFromIdx = repeatFromOptionIndex(t.RepeatFrom.Int())
	if addTaskRepeatOpts[m.addTaskRepeatIdx] == "custom" {
		m.addTaskRepeatInput.SetValue(t.RepeatFlag)
	}
	m.addTaskField = addTaskFieldTitle
	m.focusAddTaskField()
}

func (m *model) resetTaskFormFields() {
	m.addInput.Blur()
	m.addTaskField = addTaskFieldTitle
	m.addTaskReminderIdx = 1
	m.addTaskPriorityIdx = 0
	m.addTaskRepeatIdx = 0
	m.addTaskRepeatFromIdx = 0
	m.taskTitleInput.SetValue("")
	m.taskTitleInput.Placeholder = "task title…"
	m.addTaskNotesInput.Reset()
	m.addTaskDueInput.SetValue("")
	m.addTaskDueInput.Placeholder = "DD/MM/YYYY"
	m.addTaskTimeInput.SetValue("")
	m.addTaskDurationInput.SetValue("")
	m.addTaskFocusInput.SetValue("")
	m.addTaskRepeatInput.SetValue("")
	m.focusAddTaskField()
}

func priorityOptionIndex(v int) int {
	best := 0
	for i, opt := range addTaskPriorityOpts {
		if opt.value == v {
			return i
		}
		if opt.value <= v {
			best = i
		}
	}
	return best
}

func repeatOptionIndex(rule string) int {
	preset := ticktick.RecurrencePreset(rule)
	for i, option := range addTaskRepeatOpts {
		if option == preset {
			return i
		}
	}
	return 0
}

func repeatFromOptionIndex(value int) int {
	for i, option := range addTaskRepeatFromOpts {
		if option.value == value {
			return i
		}
	}
	return 0
}

func reminderOptionIndex(t ticktick.Task) int {
	if t.Reminder == "" {
		return 0
	}
	before, ok := ticktick.ParseReminderBefore(t.Reminder)
	if !ok {
		return 1
	}
	for i, opt := range addTaskReminderOpts {
		if opt.mins < 0 {
			continue
		}
		want := time.Duration(opt.mins) * time.Minute
		if before == want {
			return i
		}
	}
	return 1
}

func taskDurationMinutes(t ticktick.Task) string {
	if t.IsAllDay || !hasDueTime(t.DueDate) {
		return ""
	}
	start, ok1 := parseDueTime(t.StartDate)
	due, ok2 := parseDueTime(t.DueDate)
	if !ok1 || !ok2 {
		return ""
	}
	d := due.Sub(start).Round(time.Minute)
	if d <= 0 {
		return ""
	}
	return strconv.Itoa(int(d.Minutes()))
}

func (m *model) focusAddTaskField() {
	m.taskTitleInput.Blur()
	m.addTaskNotesInput.Blur()
	m.addTaskDueInput.Blur()
	m.addTaskTimeInput.Blur()
	m.addTaskDurationInput.Blur()
	m.addTaskFocusInput.Blur()
	m.addTaskRepeatInput.Blur()
	switch m.addTaskField {
	case addTaskFieldTitle:
		m.taskTitleInput.Focus()
	case addTaskFieldNotes:
		m.addTaskNotesInput.Focus()
	case addTaskFieldDue:
		m.addTaskDueInput.Focus()
		m.addTaskDueInput.CursorEnd()
	case addTaskFieldTime:
		m.addTaskTimeInput.Focus()
	case addTaskFieldDuration:
		m.addTaskDurationInput.Focus()
	case addTaskFieldFocus:
		m.addTaskFocusInput.Focus()
	case addTaskFieldRepeatRule:
		m.addTaskRepeatInput.Focus()
	}
}

func (m model) taskFormFieldVisible(field int) bool {
	return field != addTaskFieldRepeatRule || addTaskRepeatOpts[m.addTaskRepeatIdx] == "custom"
}

func (m model) nextTaskFormField(field, delta int) int {
	if delta == 0 {
		return field
	}
	for i := 0; i < addTaskFieldCount; i++ {
		field = (field + delta + addTaskFieldCount) % addTaskFieldCount
		if m.taskFormFieldVisible(field) {
			return field
		}
	}
	return field
}

func taskNotesInputView(ta textarea.Model, contentW int) []string {
	w := max(contentW-2, 20)
	ta.SetWidth(w)
	view := strings.TrimSuffix(ta.View(), "\n")
	if view == "" {
		return []string{""}
	}
	var lines []string
	for _, line := range strings.Split(view, "\n") {
		lines = append(lines, truncateInner(line, contentW))
	}
	return lines
}

func (m model) renderAddTaskForm(width, innerLines int) string {
	contentW := width
	if contentW < 20 {
		contentW = 20
	}
	header := iconAdd + " New task"
	if m.mode == modeEditTask {
		header = iconEdit + " Edit task"
	}
	title := truncateInner(headerStyle.Render(header), contentW)
	var lines []string
	lines = append(lines, title, "")
	selectedLine := 0

	for i := 0; i < addTaskFieldCount; i++ {
		if !m.taskFormFieldVisible(i) {
			continue
		}
		label := addTaskFieldLabels[i]
		hint := addTaskFieldHints[i]
		active := i == m.addTaskField
		labelSt := hintStyle
		if active {
			labelSt = listSelStyle
			selectedLine = len(lines)
		}
		lines = append(lines, truncateInner(labelSt.Render(fmt.Sprintf("%s  %s", label, hintStyle.Render("("+hint+")"))), contentW))
		switch i {
		case addTaskFieldTitle:
			lines = append(lines, taskTitleInputView(m.taskTitleInput, contentW))
		case addTaskFieldNotes:
			lines = append(lines, taskNotesInputView(m.addTaskNotesInput, contentW)...)
		case addTaskFieldDue:
			lines = append(lines, truncateInner(m.addTaskDueInput.View(), contentW))
		case addTaskFieldTime:
			lines = append(lines, truncateInner(m.addTaskTimeInput.View(), contentW))
		case addTaskFieldDuration:
			lines = append(lines, truncateInner(m.addTaskDurationInput.View(), contentW))
		case addTaskFieldFocus:
			lines = append(lines, truncateInner(m.addTaskFocusInput.View(), contentW))
		case addTaskFieldRepeat:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskRepeatOpts[m.addTaskRepeatIdx], active), contentW))
		case addTaskFieldRepeatFrom:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskRepeatFromOpts[m.addTaskRepeatFromIdx].label, active), contentW))
		case addTaskFieldRepeatRule:
			lines = append(lines, truncateInner(m.addTaskRepeatInput.View(), contentW))
		case addTaskFieldReminder:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskReminderOpts[m.addTaskReminderIdx].label, active), contentW))
		case addTaskFieldPriority:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskPriorityOpts[m.addTaskPriorityIdx].label, active), contentW))
		}
		if i < addTaskFieldCount-1 {
			lines = append(lines, "")
		}
	}
	if preview := m.taskFormPreview(); preview != "" {
		lines = append(lines, "", truncateInner(sectionHeader("Plan preview", contentW), contentW))
		lines = append(lines, truncateInner(hintStyle.Render(preview), contentW))
	}
	if h := m.keyHint("tab next · shift+tab prev · [/] cycle · enter next/save · esc cancel · notes: enter = newline"); h != "" {
		lines = append(lines, "", truncateInner(h, contentW))
	}
	if len(lines) > innerLines {
		window := computeScrollWindow(selectedLine, len(lines), innerLines)
		visible := append([]string(nil), lines[window.Start:window.End]...)
		if window.Start > 0 && len(visible) > 0 {
			visible[0] = truncateInner(hintStyle.Render("↑ more fields"), contentW)
		}
		if window.End < len(lines) && len(visible) > 0 {
			visible[len(visible)-1] = truncateInner(hintStyle.Render("↓ more fields"), contentW)
		}
		return fitLines(strings.Join(visible, "\n"), innerLines)
	}
	return fitLines(strings.Join(lines, "\n"), innerLines)
}

func (m model) renderAddTaskChoice(value string, active bool) string {
	st := listIdleStyle
	if active {
		st = taskSelStyle
	}
	return "  " + st.Render(value)
}

func (m model) taskFormPreview() string {
	var parts []string
	dateValue := strings.TrimSpace(m.addTaskDueInput.Value())
	timeValue := strings.TrimSpace(m.addTaskTimeInput.Value())
	durationValue := strings.TrimSpace(m.addTaskDurationInput.Value())
	if dateValue != "" {
		schedule := dateValue
		if timeValue != "" {
			schedule += " " + timeValue
			if minutes, err := strconv.Atoi(durationValue); err == nil && minutes > 0 {
				if start, err := time.Parse("15:04", timeValue); err == nil {
					schedule += "–" + start.Add(time.Duration(minutes)*time.Minute).Format("15:04")
				}
			}
		} else {
			schedule += " all day"
		}
		parts = append(parts, schedule)
	}
	if plan, err := m.parseTaskFocusPlan(); err == nil && plan != nil && !plan.Clear {
		parts = append(parts, fmt.Sprintf("%dm focus · %d pomos", plan.Minutes, plan.Pomos))
	}
	if repeat := addTaskRepeatOpts[m.addTaskRepeatIdx]; repeat != "none" {
		parts = append(parts, repeat+" from "+addTaskRepeatFromOpts[m.addTaskRepeatFromIdx].label)
	}
	return strings.Join(parts, " · ")
}

func (m model) updateAddTaskForm(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		return m, nil
	case "tab":
		m.addTaskField = m.nextTaskFormField(m.addTaskField, 1)
		m.focusAddTaskField()
		return m, textinput.Blink
	case "shift+tab":
		m.addTaskField = m.nextTaskFormField(m.addTaskField, -1)
		m.focusAddTaskField()
		return m, textinput.Blink
	case "enter":
		if m.addTaskField == addTaskFieldNotes {
			var cmd tea.Cmd
			m.addTaskNotesInput, cmd = m.addTaskNotesInput.Update(msg)
			return m, cmd
		}
		if m.addTaskField < addTaskFieldCount-1 {
			m.addTaskField = m.nextTaskFormField(m.addTaskField, 1)
			m.focusAddTaskField()
			return m, textinput.Blink
		}
		sched, clearDue, err := m.parseTaskFormSchedule()
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		focusPlan, err := m.parseTaskFocusPlan()
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		recurrence, clearRecurrence, err := m.parseTaskRecurrence(sched)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		title := strings.TrimSpace(m.taskTitleInput.Value())
		notes := taskNotesToHTML(m.addTaskNotesInput.Value())
		priority := addTaskPriorityOpts[m.addTaskPriorityIdx].value
		if m.mode == modeEditTask {
			if m.editTaskID == "" {
				m.errMsg = "no task selected"
				return m, nil
			}
			return m, updateTaskCmd(
				m.client, m.editTaskID, m.projectID, title, notes, priority,
				sched, clearDue, recurrence, clearRecurrence, focusPlan,
			)
		}
		if m.projectID == "" {
			m.errMsg = "select a list first"
			return m, nil
		}
		createIn := ticktick.TaskCreateInput{
			Title:     title,
			Content:   notes,
			ProjectID: m.projectID,
			Priority:  priority,
		}
		if !clearDue && sched != nil {
			createIn.Schedule = sched
		}
		createIn.Recurrence = recurrence
		createIn.FocusPlan = focusPlan
		return m, createTaskCmd(m.client, createIn)
	case "[", "left":
		if m.addTaskField == addTaskFieldRepeat {
			m.addTaskRepeatIdx--
			if m.addTaskRepeatIdx < 0 {
				m.addTaskRepeatIdx = len(addTaskRepeatOpts) - 1
			}
			return m, nil
		}
		if m.addTaskField == addTaskFieldRepeatFrom {
			m.addTaskRepeatFromIdx--
			if m.addTaskRepeatFromIdx < 0 {
				m.addTaskRepeatFromIdx = len(addTaskRepeatFromOpts) - 1
			}
			return m, nil
		}
		if m.addTaskField == addTaskFieldReminder {
			m.addTaskReminderIdx--
			if m.addTaskReminderIdx < 0 {
				m.addTaskReminderIdx = len(addTaskReminderOpts) - 1
			}
			return m, nil
		}
		if m.addTaskField == addTaskFieldPriority {
			m.addTaskPriorityIdx--
			if m.addTaskPriorityIdx < 0 {
				m.addTaskPriorityIdx = len(addTaskPriorityOpts) - 1
			}
			return m, nil
		}
	case "]", "right":
		if m.addTaskField == addTaskFieldRepeat {
			m.addTaskRepeatIdx = (m.addTaskRepeatIdx + 1) % len(addTaskRepeatOpts)
			return m, nil
		}
		if m.addTaskField == addTaskFieldRepeatFrom {
			m.addTaskRepeatFromIdx = (m.addTaskRepeatFromIdx + 1) % len(addTaskRepeatFromOpts)
			return m, nil
		}
		if m.addTaskField == addTaskFieldReminder {
			m.addTaskReminderIdx = (m.addTaskReminderIdx + 1) % len(addTaskReminderOpts)
			return m, nil
		}
		if m.addTaskField == addTaskFieldPriority {
			m.addTaskPriorityIdx = (m.addTaskPriorityIdx + 1) % len(addTaskPriorityOpts)
			return m, nil
		}
	}

	var cmd tea.Cmd
	switch m.addTaskField {
	case addTaskFieldTitle:
		m.taskTitleInput, cmd = m.taskTitleInput.Update(msg)
	case addTaskFieldNotes:
		m.addTaskNotesInput, cmd = m.addTaskNotesInput.Update(msg)
	case addTaskFieldDue:
		m.addTaskDueInput, cmd = m.addTaskDueInput.Update(msg)
		val := m.addTaskDueInput.Value()
		cur := m.addTaskDueInput.Position()
		formatted, newCur := formatDateInputPreserveCursor(val, cur)
		if formatted != val {
			m.addTaskDueInput.SetValue(formatted)
		}
		m.addTaskDueInput.SetCursor(newCur)
	case addTaskFieldTime:
		m.addTaskTimeInput, cmd = m.addTaskTimeInput.Update(msg)
	case addTaskFieldDuration:
		m.addTaskDurationInput, cmd = m.addTaskDurationInput.Update(msg)
	case addTaskFieldFocus:
		m.addTaskFocusInput, cmd = m.addTaskFocusInput.Update(msg)
	case addTaskFieldRepeatRule:
		m.addTaskRepeatInput, cmd = m.addTaskRepeatInput.Update(msg)
	}
	return m, cmd
}

func (m model) parseTaskFormSchedule() (*ticktick.TaskSchedule, bool, error) {
	title := strings.TrimSpace(m.taskTitleInput.Value())
	if title == "" {
		return nil, false, fmt.Errorf("title is required")
	}
	dueStr := strings.TrimSpace(m.addTaskDueInput.Value())
	if dueStr == "" {
		return nil, true, nil
	}
	d, err := parseDueDateInput(dueStr)
	if err != nil {
		return nil, false, err
	}
	timeStr := strings.TrimSpace(m.addTaskTimeInput.Value())
	allDay := timeStr == ""
	if !allDay {
		parts := strings.Split(timeStr, ":")
		if len(parts) != 2 {
			return nil, false, fmt.Errorf("time: use HH:MM")
		}
		hh, err1 := strconv.Atoi(parts[0])
		mm, err2 := strconv.Atoi(parts[1])
		if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
			return nil, false, fmt.Errorf("time: use HH:MM")
		}
		d = d.Add(time.Duration(hh)*time.Hour + time.Duration(mm)*time.Minute)
	}
	sched := &ticktick.TaskSchedule{
		Start:       d,
		HasDue:      true,
		AllDay:      allDay,
		HasReminder: addTaskReminderOpts[m.addTaskReminderIdx].mins >= 0,
	}
	if sched.HasReminder {
		mins := addTaskReminderOpts[m.addTaskReminderIdx].mins
		sched.ReminderBefore = time.Duration(mins) * time.Minute
	}
	durStr := strings.TrimSpace(m.addTaskDurationInput.Value())
	if durStr != "" && !allDay {
		mins, err := strconv.Atoi(durStr)
		if err != nil || mins < 1 {
			return nil, false, fmt.Errorf("duration: use minutes")
		}
		sched.Duration = time.Duration(mins) * time.Minute
	}
	return sched, false, nil
}

func (m model) parseTaskFocusPlan() (*ticktick.TaskFocusPlan, error) {
	raw := strings.ToLower(strings.TrimSpace(m.addTaskFocusInput.Value()))
	if raw == "" {
		if m.mode == modeEditTask {
			return &ticktick.TaskFocusPlan{Clear: true}, nil
		}
		return nil, nil
	}
	unit := byte('m')
	if last := raw[len(raw)-1]; last == 'm' || last == 'p' {
		unit = last
		raw = strings.TrimSpace(raw[:len(raw)-1])
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return nil, fmt.Errorf("planned focus: use minutes or pomos, e.g. 75m or 3p")
	}
	minutes, pomos := value, 0
	if unit == 'p' {
		pomos = value
		minutes = value * ticktick.StandardPomoMinutes
	} else {
		pomos = (minutes + ticktick.StandardPomoMinutes - 1) / ticktick.StandardPomoMinutes
	}
	if pomos > 60 {
		return nil, fmt.Errorf("planned focus: maximum is 60 pomos")
	}
	return &ticktick.TaskFocusPlan{Minutes: minutes, Pomos: pomos}, nil
}

func (m model) parseTaskRecurrence(schedule *ticktick.TaskSchedule) (*ticktick.TaskRecurrence, bool, error) {
	preset := addTaskRepeatOpts[m.addTaskRepeatIdx]
	if preset == "none" {
		return nil, m.mode == modeEditTask, nil
	}
	if schedule == nil || !schedule.HasDue {
		return nil, false, fmt.Errorf("repeat requires a due date")
	}
	rule := ""
	if preset == "custom" {
		rule = m.addTaskRepeatInput.Value()
	} else {
		rule = ticktick.RecurrencePresetRule(preset, schedule.Start)
	}
	recurrence, err := ticktick.NewTaskRecurrence(rule, addTaskRepeatFromOpts[m.addTaskRepeatFromIdx].value)
	if err != nil {
		return nil, false, fmt.Errorf("repeat: %w", err)
	}
	return recurrence, false, nil
}

func updateTaskCmd(
	c *ticktick.Client,
	taskID, projectID, title, content string,
	priority int,
	sched *ticktick.TaskSchedule,
	clearDue bool,
	recurrence *ticktick.TaskRecurrence,
	clearRecurrence bool,
	focusPlan *ticktick.TaskFocusPlan,
) tea.Cmd {
	return func() tea.Msg {
		in := ticktick.TaskUpdateInput{
			Title:           title,
			Content:         content,
			Priority:        priority,
			Schedule:        sched,
			ClearDue:        clearDue,
			Recurrence:      recurrence,
			ClearRecurrence: clearRecurrence,
			FocusPlan:       focusPlan,
		}
		err := c.UpdateTask(taskID, projectID, in)
		return taskUpdatedMsg{title: title, err: err}
	}
}

func createTaskCmd(c *ticktick.Client, in ticktick.TaskCreateInput) tea.Cmd {
	return func() tea.Msg {
		_, err := c.CreateTask(in)
		return taskAddedMsg{title: in.Title, err: err}
	}
}
