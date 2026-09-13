package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/notify"
	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
)

var (
	focusAlertTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorPeach)

	focusAlertTaskStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorText)

	focusAlertBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPeach).
				Padding(1, 2)

	focusAlertMegaBoxStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(colorPeach).
				Padding(2, 4)
)

// PreviewFocusAlert renders the focus-complete overlay for CLI preview/testing.
func PreviewFocusAlert(width, height int, escalated bool, taskTitle string) string {
	m := model{
		width:                width,
		height:               height,
		focusNotifyEscalated: escalated,
		focusAlertTitle:      taskTitle,
	}
	return m.renderFocusAlertOverlay()
}

func (m model) renderFocusAlertOverlay() string {
	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}

	escalated := m.focusNotifyEscalated
	innerW := w - 12
	if escalated {
		innerW = w - 6
	}
	if innerW < 40 {
		innerW = w - 4
	}
	if innerW < 20 {
		innerW = 20
	}

	task := strings.TrimSpace(m.focusAlertTitle)
	if task == "" {
		task = "(untitled focus)"
	}
	maxTaskW := innerW - 4
	if escalated {
		maxTaskW = innerW - 2
	}
	if lipgloss.Width(task) > maxTaskW {
		task = truncateRunes(task, max(1, maxTaskW-1)) + "…"
	}

	headline := iconCheck + "  Pomodoro complete"
	timeLabel := "Time is up"
	timeStyle := timerBigStyle
	if escalated {
		headline = iconCheck + "  Pomodoro complete"
		timeLabel = "TIME IS UP"
		timeStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPeach)
	}

	var lines []string
	lines = append(lines, helpInnerLine("", innerW))
	lines = append(lines, helpInnerLine(focusAlertTitleStyle.Render(headline), innerW))
	lines = append(lines, helpInnerLine("", innerW))
	lines = append(lines, helpInnerLine(centerStyledText(timeStyle.Render(timeLabel), innerW), innerW))
	lines = append(lines, helpInnerLine("", innerW))
	taskLine := focusAlertTaskStyle.Render(task)
	if escalated {
		taskLine = lipgloss.NewStyle().Bold(true).Foreground(colorMauve).Render(task)
	}
	lines = append(lines, helpInnerLine(centerStyledText(taskLine, innerW), innerW))
	lines = append(lines, helpInnerLine("", innerW))
	if escalated {
		lines = append(lines, helpInnerLine(hintStyle.Render("Still waiting — dismiss to continue"), innerW))
		lines = append(lines, helpInnerLine("", innerW))
	} else if sess, err := focus.Load(); err == nil && sess.State == focus.StateAwaitingDismiss && sess.InOvertimeGrace() {
		lines = append(lines, helpInnerLine(hintStyle.Render("Unclaimed time starts in "+formatClock(sess.OvertimeGraceRemaining())), innerW))
		lines = append(lines, helpInnerLine(hintStyle.Render("Dismiss now (Enter/D) or stop & log (S) to skip unclaimed"), innerW))
		lines = append(lines, helpInnerLine("", innerW))
	}
	if m.focusAlertChord {
		lines = append(lines, helpInnerLine(helpSelStyle.Render("n _   t dismiss · r repeat same session"), innerW))
	} else if h := m.keyHint("Enter/D dismiss · R repeat · S stop & log · P pause"); h != "" {
		lines = append(lines, helpInnerLine(h, innerW))
	}
	if h := m.keyHint("R logs this session and starts the same task + duration again"); h != "" {
		lines = append(lines, helpInnerLine(h, innerW))
	}
	lines = append(lines, helpInnerLine("", innerW))

	boxStyle := focusAlertBoxStyle
	boxPad := 4
	if escalated {
		boxStyle = focusAlertMegaBoxStyle
		boxPad = 8
	}
	boxW := innerW + boxPad
	box := boxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
	return centerBoxOnDimScreen(box, w, h, escalated)
}

func (m model) updateFocusAlert(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.focusAlertChord {
			m.focusAlertChord = false
			switch msg.String() {
			case "t", "T":
				return m.dismissFocusAlert()
			case "r", "R":
				return m.repeatFocusSession()
			case "esc":
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "n", "N":
			m.focusAlertChord = true
			return m, nil
		case "enter", "d", "D", " ", "esc":
			return m.dismissFocusAlert()
		case "s", "S":
			m.showFocusAlert = false
			m.focusAlertDismissed = true
			m.focusAlertChord = false
			return m, tea.Batch(focusStopCmd(m.client), focusDismissNotifyCmd())
		case "p", "P":
			m.showFocusAlert = false
			m.focusAlertChord = false
			return m, focusPauseCmd()
		case "r", "R":
			return m.repeatFocusSession()
		}
		return m, nil
	case tickMsg:
		return m.handleFocusTick()
	}
	return m, nil
}

func (m model) dismissFocusAlert() (model, tea.Cmd) {
	m.showFocusAlert = false
	m.focusAlertDismissed = true
	m.focusAlertChord = false
	return m, tea.Batch(focusFinalizeDismissCmd(m.client), focusDismissNotifyCmd(), loadPomoCmd(m.client, m.pomoViewDate))
}

func (m model) repeatFocusSession() (model, tea.Cmd) {
	m.showFocusAlert = false
	m.focusAlertDismissed = true
	m.focusAlertChord = false
	return m, tea.Batch(focusRepeatCmd(m.client), focusDismissNotifyCmd(), loadPomoCmd(m.client, m.pomoViewDate))
}

func (m model) syncFocusSession(sess *focus.Session) model {
	if sess == nil || !sess.Active() {
		if !m.focusTrackedStart.IsZero() {
			m.focusTrackedStart = time.Time{}
		}
		return m
	}
	if m.focusTrackedStart.IsZero() || !sess.StartedAt.Equal(m.focusTrackedStart) {
		m.focusTrackedStart = sess.StartedAt
		m.focusNotifySent = false
		m.focusNotifyEscalated = false
		m.showFocusAlert = false
		m.focusAlertDismissed = false
		m.focusAlertTitle = ""
		m.focusAlertChord = false
		m.focusAlertSince = time.Time{}
	}
	if sess.PlannedLogged {
		m.focusPlannedLogged = true
	}
	return m
}

func (m model) handleFocusTick() (model, tea.Cmd) {
	var cmds []tea.Cmd
	cmds = append(cmds, tickCmd())
	sess, err := focus.Load()
	if err != nil {
		sessionlog.Appendf("focus_tick", "load err=%v", err)
		return m, tea.Batch(cmds...)
	}
	m = m.syncFocusSession(sess)
	if !sess.Finished() {
		return m, tea.Batch(cmds...)
	}
	var batch []tea.Cmd
	batch = append(batch, cmds...)

	if !m.focusPlannedLogged {
		batch = append(batch, focusCompleteSessionCmd(m.client))
	}
	if !m.focusAlertDismissed && !m.showFocusAlert {
		m.showFocusAlert = true
		m.focusAlertTitle = sess.TaskTitle
		m.focusAlertSince = time.Now()
		sessionlog.Appendf("focus_alert_show", "task=%q", sess.TaskTitle)
	}
	if m.focusAlertDismissed {
		return m, tea.Batch(batch...)
	}
	if !m.focusNotifySent {
		m.focusNotifySent = true
		sessionlog.Appendf("focus_notify_cmd", "task=%q via=nt-escalate", sess.TaskTitle)
		batch = append(batch, focusNotifyCmd(sess.TaskTitle, false))
	} else if !m.focusNotifyEscalated {
		if !m.focusAlertSince.IsZero() && time.Since(m.focusAlertSince) >= notify.EscalateAfter {
			m.focusNotifyEscalated = true
			sessionlog.Appendf("focus_alert_escalate", "task=%q", sess.TaskTitle)
		}
	}
	return m, tea.Batch(batch...)
}
