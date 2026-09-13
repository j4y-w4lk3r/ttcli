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
	addTaskFieldReminder
	addTaskFieldPriority
	addTaskFieldCount
)

var (
	addTaskFieldLabels = []string{"Title", "Notes", "Due date", "Time", "Duration", "Reminder", "Priority"}
	addTaskFieldHints  = []string{
		"required",
		"optional · Enter for new line",
		"DD/MM/YYYY · empty = none",
		"HH:MM · empty = all day",
		"minutes · empty = none",
		"none · 0 · 5 · 15 · 30 · 60",
		"none · low · med · high",
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
	m.addTaskNotesInput.SetValue(stripTaskHTML(t.Content))
	m.addTaskNotesInput.CursorEnd()
	m.addTaskDueInput.SetValue(formatDueDMY(t.DueDate))
	if !t.IsAllDay && hasDueTime(t.DueDate) {
		m.addTaskTimeInput.SetValue(dueTaskClock(t))
	}
	if dur := taskDurationMinutes(t); dur != "" {
		m.addTaskDurationInput.SetValue(dur)
	}
	m.addTaskPriorityIdx = priorityOptionIndex(t.Priority.Int())
	m.addTaskReminderIdx = reminderOptionIndex(t)
	m.addTaskField = addTaskFieldTitle
	m.focusAddTaskField()
}

func (m *model) resetTaskFormFields() {
	m.addInput.Blur()
	m.addTaskField = addTaskFieldTitle
	m.addTaskReminderIdx = 1
	m.addTaskPriorityIdx = 0
	m.taskTitleInput.SetValue("")
	m.taskTitleInput.Placeholder = "task title…"
	m.addTaskNotesInput.Reset()
	m.addTaskDueInput.SetValue("")
	m.addTaskDueInput.Placeholder = "DD/MM/YYYY"
	m.addTaskTimeInput.SetValue("")
	m.addTaskDurationInput.SetValue("")
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
	}
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

	for i := 0; i < addTaskFieldCount; i++ {
		label := addTaskFieldLabels[i]
		hint := addTaskFieldHints[i]
		active := i == m.addTaskField
		labelSt := hintStyle
		if active {
			labelSt = listSelStyle
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
		case addTaskFieldReminder:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskReminderOpts[m.addTaskReminderIdx].label, active), contentW))
		case addTaskFieldPriority:
			lines = append(lines, truncateInner(m.renderAddTaskChoice(addTaskPriorityOpts[m.addTaskPriorityIdx].label, active), contentW))
		}
		if i < addTaskFieldCount-1 {
			lines = append(lines, "")
		}
	}
	if h := m.keyHint("tab next · shift+tab prev · [/] cycle · enter next/save · esc cancel · notes: enter = newline"); h != "" {
		lines = append(lines, "", truncateInner(h, contentW))
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

func (m model) updateAddTaskForm(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.editTaskID = ""
		m.blurTaskFormInputs()
		return m, nil
	case "tab":
		m.addTaskField = (m.addTaskField + 1) % addTaskFieldCount
		m.focusAddTaskField()
		return m, textinput.Blink
	case "shift+tab":
		m.addTaskField--
		if m.addTaskField < 0 {
			m.addTaskField = addTaskFieldCount - 1
		}
		m.focusAddTaskField()
		return m, textinput.Blink
	case "enter":
		if m.addTaskField == addTaskFieldNotes {
			var cmd tea.Cmd
			m.addTaskNotesInput, cmd = m.addTaskNotesInput.Update(msg)
			return m, cmd
		}
		if m.addTaskField < addTaskFieldCount-1 {
			m.addTaskField++
			m.focusAddTaskField()
			return m, textinput.Blink
		}
		sched, clearDue, err := m.parseTaskFormSchedule()
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		title := strings.TrimSpace(m.taskTitleInput.Value())
		notes := strings.TrimSpace(m.addTaskNotesInput.Value())
		priority := addTaskPriorityOpts[m.addTaskPriorityIdx].value
		if m.mode == modeEditTask {
			if m.editTaskID == "" {
				m.errMsg = "no task selected"
				return m, nil
			}
			return m, updateTaskCmd(m.client, m.editTaskID, m.projectID, title, notes, priority, sched, clearDue)
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
		return m, createTaskCmd(m.client, createIn)
	case "[", "left":
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
		Due:         d,
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

func updateTaskCmd(c *ticktick.Client, taskID, projectID, title, content string, priority int, sched *ticktick.TaskSchedule, clearDue bool) tea.Cmd {
	return func() tea.Msg {
		in := ticktick.TaskUpdateInput{
			Title:    title,
			Content:  content,
			Priority: priority,
			Schedule: sched,
			ClearDue: clearDue,
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
