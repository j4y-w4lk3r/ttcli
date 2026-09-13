// Package sessionlog appends ttcli events (focus, notifications, task moves) to a log file.
package sessionlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var mu sync.Mutex

func logPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "focus-session.log"), nil
}

// Append writes a tab-separated event line to ~/.cache/ttcli/focus-session.log.
func Append(event, detail string) {
	mu.Lock()
	defer mu.Unlock()
	path, err := logPath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	ts := time.Now().Format(time.RFC3339Nano)
	if detail != "" {
		_, _ = fmt.Fprintf(f, "%s\t%s\t%s\n", ts, event, detail)
	} else {
		_, _ = fmt.Fprintf(f, "%s\t%s\n", ts, event)
	}
}

// Appendf formats detail and appends the event.
func Appendf(event, format string, args ...any) {
	Append(event, fmt.Sprintf(format, args...))
}
