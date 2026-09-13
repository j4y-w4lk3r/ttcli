package notify

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

const (
	// ntGrowMS matches nt escalate default — canvas transform grow duration.
	ntGrowMS = 2600
	// ntRefreshHZ 0 lets nt overlay auto-detect display refresh (120 Hz, etc.).
	ntRefreshHZ = 0
)

// buildNTEscalateArgs returns argv for `nt escalate`.
// skipInitial avoids nt's built-in critical dunst ping (ttcli sends its own styled notify first).
func buildNTEscalateArgs(title, body string, delay time.Duration, skipInitial bool) []string {
	if delay <= 0 {
		delay = EscalateAfter
	}
	args := []string{
		"escalate",
		"--delay", formatNTDelay(delay),
		"--grow-ms", strconv.Itoa(ntGrowMS),
	}
	if skipInitial {
		args = append(args, "--skip-initial")
	}
	if ntRefreshHZ > 0 {
		args = append(args, "--hz", strconv.Itoa(ntRefreshHZ))
	}
	args = append(args, title, body)
	return args
}

// startNTEscalate launches `nt escalate` (dunst → wait → 120 Hz-smooth fullscreen overlay).
func startNTEscalate(title, body string, delay time.Duration, skipInitial bool) (pid int, err error) {
	ntPath, err := exec.LookPath("nt")
	if err != nil {
		return 0, err
	}
	args := buildNTEscalateArgs(title, body, delay, skipInitial)

	cmd := exec.Command(ntPath, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	return cmd.Process.Pid, nil
}

func formatNTDelay(d time.Duration) string {
	secs := d.Seconds()
	if secs == float64(int(secs)) {
		return strconv.Itoa(int(secs))
	}
	return strconv.FormatFloat(secs, 'f', 1, 64)
}

// CancelOverlay kills any nt escalate overlay process, even when notify state is missing.
func CancelOverlay() {
	stopNTOverlay(0)
}

func stopNTOverlay(pid int) {
	if pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	if path, _ := exec.LookPath("pkill"); path != "" {
		for _, pattern := range []string{"nt-escalate", "nt escalate", "escalate-overlay"} {
			_ = exec.Command("pkill", "-f", pattern).Run()
		}
	}
}
