package notify

import (
	"os/exec"
	"time"
)

// BuildNotifySendArgs returns notify-send argv for a variant (without executing).
func BuildNotifySendArgs(v Variant, taskTitle string) []string {
	c := BuildContent(v, taskTitle)
	args := []string{
		"-a", "ttcli",
		"-i", focusDoneIconArg(),
		"-t", "0",
		"-u", c.Urgency,
		"-h", "int:1:" + notifyID,
	}
	args = AppendHints(args, v)
	args = append(args, c.Title, c.Body)
	return args
}

// SendVariant sends a notification using the given design variant.
// When persist is true, notification state is saved (production focus-end flow).
func SendVariant(v Variant, taskTitle string, persist bool) error {
	if _, err := exec.LookPath("notify-send"); err != nil {
		return err
	}
	args := BuildNotifySendArgs(v, taskTitle)
	if err := exec.Command("notify-send", args...).Run(); err != nil {
		return err
	}
	if !persist {
		return nil
	}
	escalated := v == VariantEscalated
	return saveState(State{
		Active:    true,
		SentAt:    time.Now(),
		Escalated: escalated,
		TaskTitle: taskTitle,
	})
}
