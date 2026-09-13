package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

const (
	addPomoFieldTask = iota
	addPomoFieldDate
	addPomoFieldStart
	addPomoFieldDuration
	addPomoFieldPause
)

func (m model) openAddPomoForm() (model, tea.Cmd) {
	m.mode = modeAddPomoForm
	m.addPomoField = addPomoFieldTask
	m.addPomoEditPause = false
	m.focusPickerSwitch = false
	m.focusPickerMinutes = 25
	m.focusPickerCursor = 0
	m.focusPickerTasks = nil
	m.focusPickerProjectNames = nil
	m.focusPickerLoading = true
	m.focusPickerEditDuration = false
	m.focusPickerDurationBuf = ""
	m.focusPickerFilter = ""
	m.addPomoLogDate = dateOnly(m.pomoViewDate)
	now := time.Now()
	m.addPomoStartMinutes = now.Hour()*60 + now.Minute()
	m.addPomoStartUnset = dateKey(m.addPomoLogDate) == dateKey(now)
	m.addPomoPauseInput.SetValue("")
	m.blurAddPomoInputs()
	return m, loadFocusPickerCmd(m.client)
}

func (m *model) blurAddPomoInputs() {
	m.addPomoPauseInput.Blur()
}

func (m model) addPomoLogDateIsToday() bool {
	return dateKey(m.addPomoLogDate) == dateKey(time.Now())
}

func clampAddPomoStartMinutes(mins int) int {
	if mins < 0 {
		return 0
	}
	if mins >= 24*60 {
		return 24*60 - 1
	}
	return mins
}

func (m model) shiftAddPomoLogDate(delta int) model {
	next := dateOnly(m.addPomoLogDate.AddDate(0, 0, delta))
	today := dateOnly(time.Now())
	if next.After(today) {
		next = today
	}
	m.addPomoLogDate = next
	if !m.addPomoLogDateIsToday() {
		m.addPomoStartUnset = false
	}
	return m
}

func (m model) renderAddPomoOverlay() string {
	boxW, innerW, listH := focusPickerLayout(m.width, m.height)
	var lines []string
	lines = append(lines, helpInnerLine(centerStyledText(headerStyle.Render("Log focus session"), innerW), innerW))
	lines = append(lines, helpInnerLine(centerStyledText(hintStyle.Render("pick task, day, and when it started"), innerW), innerW))
	lines = append(lines, helpInnerLine(focusPickerDivider(innerW), innerW))

	fieldLabel := func(label string, active bool) string {
		st := hintStyle
		if active {
			st = headerStyle
		}
		return st.Render(label)
	}

	lines = append(lines, helpInnerLine(fieldLabel("Task", m.addPomoField == addPomoFieldTask), innerW))
	searchRows := 0
	if q := strings.TrimSpace(m.focusPickerFilter); q != "" {
		lines = append(lines, helpInnerLine(helpSearchStyle.Render("search: "+q), innerW))
		searchRows = 1
	}
	taskListH := listH - searchRows - 5
	if taskListH < 2 {
		taskListH = 2
	}
	if taskListH > 8 {
		taskListH = 8
	}

	visible := m.focusPickerVisibleTasks()
	if m.focusPickerLoading {
		lines = append(lines, helpInnerLine(hintStyle.Render("loading tasks…"), innerW))
	} else if len(visible) == 0 {
		if q := strings.TrimSpace(m.focusPickerFilter); q != "" {
			lines = append(lines, helpInnerLine(hintStyle.Render("Enter to use \""+q+"\" as new task"), innerW))
		} else {
			lines = append(lines, helpInnerLine(hintStyle.Render("(type a task name or pick from list)"), innerW))
		}
	} else {
		win := computeScrollWindow(m.focusPickerCursor, len(visible), taskListH)
		for row := 0; row < taskListH; row++ {
			idx := win.Start + row
			if idx < win.End && idx < len(visible) {
				t := visible[idx]
				name := m.focusPickerProjectNames[t.ProjectID]
				lines = append(lines, renderFocusPickerTaskLine(t, name, m.addPomoField == addPomoFieldTask && idx == m.focusPickerCursor, innerW, m.focusPickerFilter))
			} else {
				lines = append(lines, helpInnerLine("", innerW))
			}
		}
	}

	lines = append(lines, helpInnerLine(focusPickerDivider(innerW), innerW))
	lines = append(lines, helpInnerLine(m.renderAddPomoDateField(), innerW))
	lines = append(lines, helpInnerLine(m.renderAddPomoStartField(), innerW))
	if m.addPomoField == addPomoFieldStart {
		lines = append(lines, helpInnerLine(renderAddPomoStartPresetRow(innerW), innerW))
	}

	durLabel := formatFocusPickerDuration(m.focusPickerMinutes)
	if m.addPomoField == addPomoFieldDuration && m.focusPickerEditDuration {
		buf := m.focusPickerDurationBuf
		if buf == "" {
			buf = "▌"
		} else {
			buf += "▌"
		}
		durLabel = buf + "m"
	}
	lines = append(lines, helpInnerLine(fieldLabel("Focus", m.addPomoField == addPomoFieldDuration)+"  "+timerStyle.Render(durLabel), innerW))
	if m.addPomoField == addPomoFieldDuration {
		lines = append(lines, helpInnerLine(renderFocusPickerPresetRow(m.focusPickerMinutes, m.focusPickerEditDuration, innerW), innerW))
	}

	pauseVal := m.addPomoPauseInput.Value()
	if m.addPomoEditPause {
		pauseVal = pauseVal + "▌"
	}
	if pauseVal == "" {
		pauseVal = hintStyle.Render("optional")
	}
	lines = append(lines, helpInnerLine(fieldLabel("Pause", m.addPomoField == addPomoFieldPause)+"  "+inputStyle.Render(pauseVal), innerW))

	if h := m.addPomoFormHint(); h != "" {
		lines = append(lines, helpInnerLine("", innerW))
		lines = append(lines, helpInnerLine(h, innerW))
	}
	box := helpBoxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
	return centerBoxOnPlainScreen(box, m.width, m.height)
}

func (m model) renderAddPomoDateField() string {
	active := m.addPomoField == addPomoFieldDate
	label := m.addPomoLogDate.Format("Mon 2 Jan 2006")
	if m.addPomoLogDateIsToday() {
		label += "  " + calTodayStyle.Render("today")
	}
	chip := hintStyle.Render(label)
	if active {
		chip = timerBigStyle.Render(label)
	}
	prefix := fieldLabelStyle(active, "Date")
	return prefix + "  " + hintStyle.Render("◀") + " " + chip + " " + hintStyle.Render("▶")
}

func (m model) renderAddPomoStartField() string {
	active := m.addPomoField == addPomoFieldStart
	var chip string
	if m.addPomoStartUnset && m.addPomoLogDateIsToday() {
		chip = hintStyle.Render("ended now · from duration")
		if active {
			chip = timerBigStyle.Render("ended now · from duration")
		}
	} else {
		h, min := m.addPomoStartMinutes/60, m.addPomoStartMinutes%60
		label := fmt.Sprintf("%02d:%02d", h, min)
		chip = timerStyle.Render(label)
		if active {
			chip = timerBigStyle.Render(label)
		}
	}
	prefix := fieldLabelStyle(active, "Start")
	return prefix + "  " + hintStyle.Render("◀") + " " + chip + " " + hintStyle.Render("▶")
}

func fieldLabelStyle(active bool, label string) string {
	st := hintStyle
	if active {
		st = headerStyle
	}
	return st.Render(label)
}

func renderAddPomoStartPresetRow(innerW int) string {
	presets := []struct {
		key   string
		label string
		mins  int
	}{
		{"6", "08:00", 8 * 60},
		{"7", "12:00", 12 * 60},
		{"8", "14:00", 14 * 60},
		{"9", "17:00", 17 * 60},
	}
	var parts []string
	for _, p := range presets {
		parts = append(parts, hintStyle.Render(p.key+"="+p.label))
	}
	if innerW > 0 {
		return truncateInner(strings.Join(parts, "  "), innerW)
	}
	return strings.Join(parts, "  ")
}

func (m model) addPomoFormHint() string {
	switch m.addPomoField {
	case addPomoFieldDate:
		return m.keyHint("tab fields · [ ] h/l day · t today · enter save · esc cancel")
	case addPomoFieldStart:
		return m.keyHint("tab fields · [ ] ±5m · h/l ±1h · n now · 0 ended now · 6–9 presets · enter save")
	default:
		return m.keyHint("tab fields · j/k task · 1–5 focus · enter save · esc cancel")
	}
}

func (m model) updateAddPomoForm(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.blurAddPomoInputs()
		m.focusPickerFilter = ""
		return m, nil
	case "tab":
		m.addPomoField = (m.addPomoField + 1) % (addPomoFieldPause + 1)
		m.addPomoEditPause = false
		m.focusPickerEditDuration = false
		m.blurAddPomoInputs()
		return m, nil
	case "shift+tab":
		m.addPomoField--
		if m.addPomoField < 0 {
			m.addPomoField = addPomoFieldPause
		}
		m.addPomoEditPause = false
		m.focusPickerEditDuration = false
		m.blurAddPomoInputs()
		return m, nil
	case "enter":
		in, err := m.buildAddPomoLogInput()
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
		m.pomoViewDate = dateOnly(m.addPomoLogDate)
		m.mode = modeNormal
		m.blurAddPomoInputs()
		m.focusPickerFilter = ""
		return m, addPomodoroCmd(m.client, in)
	}

	switch m.addPomoField {
	case addPomoFieldTask:
		return m.updateAddPomoFormTask(msg)
	case addPomoFieldDate:
		return m.updateAddPomoFormDate(msg)
	case addPomoFieldStart:
		return m.updateAddPomoFormStart(msg)
	case addPomoFieldDuration:
		return m.updateAddPomoFormDuration(msg)
	case addPomoFieldPause:
		return m.updateAddPomoFormPause(msg)
	}
	return m, nil
}

func (m model) updateAddPomoFormTask(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+u":
		m.focusPickerFilter = ""
		m.focusPickerCursor = 0
		return m, nil
	case "backspace":
		if m.focusPickerFilter != "" {
			m.focusPickerFilter = trimLastRune(m.focusPickerFilter)
			m.focusPickerCursor = m.clampFocusPickerCursor()
		}
		return m, nil
	case "j", "down":
		visible := m.focusPickerVisibleTasks()
		if len(visible) > 0 {
			m.focusPickerCursor = min(m.focusPickerCursor+1, len(visible)-1)
		}
		return m, nil
	case "k", "up":
		if len(m.focusPickerVisibleTasks()) > 0 {
			m.focusPickerCursor = max(m.focusPickerCursor-1, 0)
		}
		return m, nil
	}
	if key, ok := msgKeyRune(msg); ok && key != "" {
		m.focusPickerFilter += key
		if len(m.focusPickerVisibleTasks()) > 0 {
			m.focusPickerCursor = 0
		}
	}
	return m, nil
}

func (m model) updateAddPomoFormDate(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "[", "h", "left":
		return m.shiftAddPomoLogDate(-1), nil
	case "]", "l", "right":
		return m.shiftAddPomoLogDate(1), nil
	case "t":
		m.addPomoLogDate = dateOnly(time.Now())
		if !m.addPomoLogDateIsToday() {
			m.addPomoStartUnset = false
		}
		return m, nil
	}
	return m, nil
}

func (m model) updateAddPomoFormStart(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "0":
		if m.addPomoLogDateIsToday() {
			m.addPomoStartUnset = !m.addPomoStartUnset
		}
		return m, nil
	case "n", "t":
		now := time.Now()
		m.addPomoStartMinutes = now.Hour()*60 + now.Minute()
		m.addPomoStartUnset = false
		return m, nil
	case "[":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = clampAddPomoStartMinutes(m.addPomoStartMinutes - 5)
	case "]":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = clampAddPomoStartMinutes(m.addPomoStartMinutes + 5)
	case "h", "left":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = clampAddPomoStartMinutes(m.addPomoStartMinutes - 60)
	case "l", "right":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = clampAddPomoStartMinutes(m.addPomoStartMinutes + 60)
	case "6":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = 8 * 60
	case "7":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = 12 * 60
	case "8":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = 14 * 60
	case "9":
		m.addPomoStartUnset = false
		m.addPomoStartMinutes = 17 * 60
	}
	return m, nil
}

func (m model) updateAddPomoFormDuration(msg tea.KeyMsg) (model, tea.Cmd) {
	if m.focusPickerEditDuration {
		switch msg.String() {
		case "esc":
			m.focusPickerEditDuration = false
			m.focusPickerDurationBuf = ""
			return m, nil
		case "enter":
			if v, ok := parseFocusDurationInput(m.focusPickerDurationBuf); ok {
				m.focusPickerMinutes = v
			}
			m.focusPickerEditDuration = false
			m.focusPickerDurationBuf = ""
			return m, nil
		case "backspace":
			if len(m.focusPickerDurationBuf) > 0 {
				m.focusPickerDurationBuf = m.focusPickerDurationBuf[:len(m.focusPickerDurationBuf)-1]
			}
			return m, nil
		default:
			if len(msg.Runes) == 1 && msg.Runes[0] >= '0' && msg.Runes[0] <= '9' {
				if len(m.focusPickerDurationBuf) < 3 {
					m.focusPickerDurationBuf += string(msg.Runes[0])
				}
			}
			return m, nil
		}
	}
	switch msg.String() {
	case "t":
		m.focusPickerEditDuration = true
		m.focusPickerDurationBuf = ""
		return m, nil
	case "[":
		m.focusPickerMinutes = clampFocusMinutes(m.focusPickerMinutes - 5)
	case "]":
		m.focusPickerMinutes = clampFocusMinutes(m.focusPickerMinutes + 5)
	case "1":
		m.focusPickerMinutes = 5
	case "2":
		m.focusPickerMinutes = 15
	case "3":
		m.focusPickerMinutes = 25
	case "4":
		m.focusPickerMinutes = 45
	case "5":
		m.focusPickerMinutes = 60
	}
	return m, nil
}

func (m model) updateAddPomoFormPause(msg tea.KeyMsg) (model, tea.Cmd) {
	switch msg.String() {
	case "t":
		m.addPomoEditPause = true
		m.addPomoPauseInput.Focus()
		return m, textinput.Blink
	}
	if !m.addPomoEditPause && len(msg.Runes) > 0 {
		m.addPomoEditPause = true
		m.addPomoPauseInput.Focus()
	}
	var cmd tea.Cmd
	m.addPomoPauseInput, cmd = m.addPomoPauseInput.Update(msg)
	return m, cmd
}

func (m model) buildAddPomoLogInput() (ticktick.LogPomodoroInput, error) {
	title, taskID, projectName, err := m.addPomoTaskSelection()
	if err != nil {
		return ticktick.LogPomodoroInput{}, err
	}
	elapsed := time.Duration(m.focusPickerMinutes) * time.Minute
	if elapsed < time.Minute {
		return ticktick.LogPomodoroInput{}, fmt.Errorf("focus duration must be at least 1 minute")
	}
	pause, err := parseAddPomoPauseMinutes(m.addPomoPauseInput.Value())
	if err != nil {
		return ticktick.LogPomodoroInput{}, err
	}
	start, err := m.parseAddPomoStart(elapsed, pause)
	if err != nil {
		return ticktick.LogPomodoroInput{}, err
	}
	return ticktick.LogPomodoroInput{
		TaskID:        taskID,
		TaskTitle:     title,
		ProjectName:   projectName,
		StartedAt:     start,
		Elapsed:       elapsed,
		PauseDuration: pause,
	}, nil
}

func (m model) addPomoTaskSelection() (title, taskID, projectName string, err error) {
	if t, ok := m.selectedFocusPickerTask(); ok {
		return t.Title, t.ID, m.focusPickerProjectNames[t.ProjectID], nil
	}
	custom := strings.TrimSpace(m.focusPickerFilter)
	if custom == "" {
		return "", "", "", fmt.Errorf("pick a task or type a new task name")
	}
	return custom, "", "", nil
}

func (m model) parseAddPomoStart(elapsed, pause time.Duration) (time.Time, error) {
	if m.addPomoStartUnset {
		if !m.addPomoLogDateIsToday() {
			return time.Time{}, fmt.Errorf("ended-now only works when logging today")
		}
		end := time.Now().UTC()
		return end.Add(-elapsed - pause), nil
	}
	h := m.addPomoStartMinutes / 60
	min := m.addPomoStartMinutes % 60
	start, err := parseClockOnDay(m.addPomoLogDate, fmt.Sprintf("%02d:%02d", h, min))
	if err != nil {
		return time.Time{}, err
	}
	return start.UTC(), nil
}

func parseClockOnDay(day time.Time, hhmm string) (time.Time, error) {
	parts := strings.Split(hhmm, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("start: use HH:MM")
	}
	hh, err1 := strconv.Atoi(parts[0])
	mm, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return time.Time{}, fmt.Errorf("start: use HH:MM")
	}
	loc := day.Location()
	return time.Date(day.Year(), day.Month(), day.Day(), hh, mm, 0, 0, loc), nil
}

func parseAddPomoPauseMinutes(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	mins, err := strconv.Atoi(raw)
	if err != nil || mins < 0 {
		return 0, fmt.Errorf("pause: use minutes")
	}
	return time.Duration(mins) * time.Minute, nil
}

func msgKeyRune(msg tea.KeyMsg) (string, bool) {
	if len(msg.Runes) != 1 {
		return "", false
	}
	r := msg.Runes[0]
	if r < 32 {
		return "", false
	}
	return string(r), true
}
