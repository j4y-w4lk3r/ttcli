package tui

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// clipboardWrite copies text to the system clipboard. Tests replace it.
var clipboardWrite = writeClipboard

func copyTaskTitleCmd(title string) tea.Cmd {
	return func() tea.Msg {
		return taskTitleCopiedMsg{title: title, err: clipboardWrite(title)}
	}
}

type taskTitleCopiedMsg struct {
	title string
	err   error
}

func copiedTitleToast(title string) string {
	const max = 48
	runes := []rune(strings.TrimSpace(title))
	if len(runes) > max {
		return string(runes[:max-1]) + "…"
	}
	return string(runes)
}

// writeClipboard prefers the desktop clipboard (wl-copy on Wayland, xclip or
// xsel on X11) and falls back to OSC 52 so the title still lands in the
// terminal clipboard when those tools are missing.
func writeClipboard(text string) error {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if err := pipeTo("wl-copy", text); err == nil {
			return nil
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if err := pipeTo("xclip", text, "-selection", "clipboard"); err == nil {
			return nil
		}
		if err := pipeTo("xsel", text, "--clipboard", "--input"); err == nil {
			return nil
		}
	}
	if err := writeOSC52(text); err == nil {
		return nil
	}
	return fmt.Errorf("clipboard: install wl-copy or xclip, or use a terminal that supports OSC 52")
}

func pipeTo(name, text string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}
	cmd := exec.Command(path, args...)
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

func writeOSC52(text string) error {
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, osc52Sequence(text, os.Getenv("TMUX") != ""))
	return err
}

func osc52Sequence(text string, tmux bool) string {
	payload := base64.StdEncoding.EncodeToString([]byte(text))
	seq := "\033]52;c;" + payload + "\a"
	if tmux {
		return "\033Ptmux;\033" + seq + "\033\\"
	}
	return seq
}
