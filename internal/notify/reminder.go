package notify

import (
	"os/exec"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
)

// TaskDue sends a task reminder through Pushover and the desktop notifier.
// It does not start the focus-session overlay.
func TaskDue(title, dueLabel string) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Task"
	}
	body := title
	if dueLabel != "" {
		body = title + " · due " + dueLabel
	}
	go notifyPushover("Reminder", body)
	if _, err := exec.LookPath("notify-send"); err == nil {
		_ = exec.Command("notify-send", "-a", "ttcli", "-u", "normal", "-t", "12000", "Reminder", body).Run()
	}
	sessionlog.Appendf("reminder_sent", "due=%q", dueLabel)
}
