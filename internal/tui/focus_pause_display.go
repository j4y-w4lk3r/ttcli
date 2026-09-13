package tui

import (
	"strings"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func liveSessionSuffix(sess *focus.Session) string {
	if sess == nil || !sess.Active() {
		return ""
	}
	pauseLabel := formatFocusPauseLabel(sess.PauseTotal())
	switch {
	case sess.State == focus.StateAwaitingDismiss && sess.InOvertimeGrace():
		return hintStyle.Render(formatClock(sess.OvertimeGraceRemaining()) + " until unclaimed")
	case sess.State == focus.StateAwaitingDismiss:
		return unclaimedLabelStyle.Render("+" + formatClock(sess.OvertimeElapsed()) + " extra")
	case sess.State == focus.StatePaused:
		if pauseLabel != "" {
			return hintStyle.Render(pauseLabel)
		}
		return hintStyle.Render("paused")
	default:
		rem := timerStyle.Render(formatClock(sess.Remaining()))
		if pauseLabel == "" {
			return rem
		}
		return rem + hintStyle.Render(" · "+pauseLabel)
	}
}

func formatFocusPauseLabel(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return formatClock(d) + " paused"
}

func appendFocusPauseDetail(parts []string, sess *focus.Session) []string {
	if p := sess.PauseTotal(); p > 0 {
		return append(parts, formatFocusPauseLabel(p))
	}
	return parts
}

func loadActiveFocusSession() *focus.Session {
	sess, err := focus.Load()
	if err != nil || !sess.Active() {
		return nil
	}
	return sess
}

func activeFocusFooterLine(sess *focus.Session) string {
	if sess == nil || !sess.Active() {
		return ""
	}
	parts := []string{"focus", sess.State, formatClock(sess.Remaining()) + " left"}
	parts = appendFocusPauseDetail(parts, sess)
	if sess.TaskTitle != "" {
		parts = append(parts, sess.TaskTitle)
	}
	return strings.Join(parts, " · ")
}
