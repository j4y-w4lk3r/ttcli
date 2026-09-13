// Package notify sends desktop notifications via notify-send (libnotify).
package notify

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/sessionlog"
)

const notifyID = "424242"

// State tracks an active focus-end notification.
type State struct {
	Active     bool      `json:"active"`
	SentAt     time.Time `json:"sentAt"`
	Escalated  bool      `json:"escalated"`
	TaskTitle  string    `json:"taskTitle,omitempty"`
	OverlayPID int       `json:"overlayPid,omitempty"`
}

var ErrNoActive = errors.New("no active focus notification")

func statePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "focus-notify.json"), nil
}

// LoadState reads notification state; missing file means inactive.
func LoadState() (State, error) {
	path, err := statePath()
	if err != nil {
		return State{}, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, nil
		}
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, err
	}
	return st, nil
}

func saveState(st State) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if !st.Active {
		_ = os.Remove(path)
		return nil
	}
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// FocusDone starts an escalating nt notification (dunst → growing fullscreen overlay).
func FocusDone(taskTitle string, escalated bool) error {
	if escalated {
		return nil
	}
	c := BuildContent(VariantOrange, taskTitle)
	sessionlog.Appendf("notify_send_attempt", "task=%q via=notify-send+nt-escalate delay=%s", taskTitle, EscalateAfter)
	if err := saveState(State{
		Active:    true,
		SentAt:    time.Now(),
		TaskTitle: taskTitle,
	}); err != nil {
		return err
	}

	// nt escalate sends its own critical dialog-warning dunst ping unless skipped.
	softSent := SendVariant(VariantOrange, taskTitle, false) == nil
	if !softSent {
		sessionlog.Appendf("notify_send_fail", "task=%q notify-send missing or failed — nt will show its default alert", taskTitle)
	} else {
		sessionlog.Appendf("notify_send_ok", "task=%q via=notify-send", taskTitle)
	}

	pid, err := startNTEscalate(c.Title, c.Body, EscalateAfter, softSent)
	if err != nil {
		sessionlog.Appendf("notify_send_fail", "task=%q nt missing: %v", taskTitle, err)
		if !softSent {
			if err2 := SendVariant(VariantOrange, taskTitle, false); err2 != nil {
				return err2
			}
			return saveState(State{
				Active:    true,
				SentAt:    time.Now(),
				TaskTitle: taskTitle,
			})
		}
		return err
	}
	sessionlog.Appendf("notify_send_ok", "task=%q via=nt-escalate pid=%d skip_initial=%v", taskTitle, pid, softSent)
	return saveState(State{
		Active:     true,
		SentAt:     time.Now(),
		TaskTitle:  taskTitle,
		OverlayPID: pid,
	})
}

// Dismiss clears the active notification and cancels any pending nt escalate overlay.
func Dismiss() error {
	stopNTOverlay(0)
	st, err := LoadState()
	if err != nil {
		sessionlog.Appendf("notify_dismiss_fail", "load err=%v", err)
		return err
	}
	if !st.Active {
		sessionlog.Append("notify_dismiss_skip", "no active notification state")
		return ErrNoActive
	}
	sessionlog.Appendf("notify_dismiss", "task=%q escalated=%v age=%s pid=%d", st.TaskTitle, st.Escalated, time.Since(st.SentAt), st.OverlayPID)
	if path, _ := exec.LookPath("notify-send"); path != "" {
		_ = exec.Command("notify-send",
			"-a", "ttcli",
			"-t", "1",
			"-h", "int:1:"+notifyID,
			"-h", "int:transient:1",
			"", "",
		).Run()
	}
	if path, _ := exec.LookPath("makoctl"); path != "" {
		_ = exec.Command("makoctl", "dismiss", "-a", "ttcli").Run()
	}
	if path, _ := exec.LookPath("dunstctl"); path != "" {
		_ = exec.Command("dunstctl", "close-all").Run()
	}
	st.Active = false
	return saveState(st)
}

// ShouldEscalate reports whether an active notification should be upgraded.
func ShouldEscalate(st State, after time.Duration) bool {
	if !st.Active || st.Escalated {
		return false
	}
	return time.Since(st.SentAt) >= after
}

// ParseVariant resolves a CLI variant name or returns empty if unknown.
func ParseVariant(name string) (Variant, bool) {
	for _, v := range Variants() {
		if string(v.ID) == name {
			return v.ID, true
		}
	}
	return "", false
}
