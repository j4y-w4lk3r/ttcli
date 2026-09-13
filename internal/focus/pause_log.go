package focus

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func focusEventLogPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "focus-session.log"), nil
}

type focusLogEvent struct {
	ts   time.Time
	kind string
	task string
}

type taskWorkState struct {
	workStart time.Time
	tracking  bool
	inPause   bool
	openIdx   int
}

func normalizeLogTask(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "task=") {
		rest := strings.TrimPrefix(raw, "task=")
		if strings.HasPrefix(rest, `"`) {
			if end := strings.Index(rest[1:], `"`); end >= 0 {
				return rest[1 : end+1]
			}
		}
		if i := strings.Index(rest, " "); i >= 0 {
			rest = rest[:i]
		}
		return strings.Trim(rest, `"`)
	}
	return strings.Trim(raw, `"`)
}

func tasksMatch(a, b string) bool {
	return strings.EqualFold(normalizeLogTask(a), normalizeLogTask(b))
}

func (st *taskWorkState) finalizeWorkAfter(out *[]PauseSpell, task string, end time.Time) {
	if !st.tracking || st.inPause || st.workStart.IsZero() {
		return
	}
	for i := len(*out) - 1; i >= 0; i-- {
		spell := &(*out)[i]
		if !tasksMatch(spell.TaskTitle, task) || spell.End.IsZero() || spell.WorkAfter > 0 {
			continue
		}
		spell.WorkAfter = end.Sub(st.workStart)
		if spell.WorkAfter < 0 {
			spell.WorkAfter = 0
		}
		return
	}
}

// pauseSpellsFromLog reconstructs pause intervals and adjacent work slices from focus-session.log.
func pauseSpellsFromLog(day time.Time) ([]PauseSpell, error) {
	events, err := readFocusLogEvents(day)
	if err != nil {
		return nil, err
	}
	states := map[string]*taskWorkState{}
	var out []PauseSpell

	for _, ev := range events {
		task := normalizeLogTask(ev.task)
		if task == "" {
			continue
		}
		st := states[task]
		if st == nil {
			st = &taskWorkState{openIdx: -1}
			states[task] = st
		}

		switch ev.kind {
		case "session_start":
			st.finalizeWorkAfter(&out, task, ev.ts)
			st.workStart = ev.ts
			st.tracking = true
			st.inPause = false
			st.openIdx = -1
		case "session_pause":
			if !st.tracking || st.inPause || st.workStart.IsZero() {
				continue
			}
			workBefore := ev.ts.Sub(st.workStart)
			if workBefore < 0 {
				workBefore = 0
			}
			out = append(out, PauseSpell{
				TaskTitle:  task,
				Start:      ev.ts,
				WorkBefore: workBefore,
			})
			st.openIdx = len(out) - 1
			st.inPause = true
		case "session_resume":
			if !st.inPause || st.openIdx < 0 || st.openIdx >= len(out) {
				continue
			}
			out[st.openIdx].End = ev.ts
			st.workStart = ev.ts
			st.inPause = false
			st.openIdx = -1
		case "session_stop", "focus_alert_show":
			if !st.tracking {
				continue
			}
			st.finalizeWorkAfter(&out, task, ev.ts)
			if st.inPause && st.openIdx >= 0 && st.openIdx < len(out) && out[st.openIdx].End.IsZero() {
				out[st.openIdx].End = ev.ts
			}
			st.tracking = false
			st.inPause = false
			st.openIdx = -1
		}
	}
	return out, nil
}

func readFocusLogEvents(day time.Time) ([]focusLogEvent, error) {
	path, err := focusEventLogPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	day = day.Local()
	y, m, d := day.Date()
	var events []focusLogEvent

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			continue
		}
		lt := ts.Local()
		if lt.Year() != y || lt.Month() != m || lt.Day() != d {
			continue
		}
		ev := focusLogEvent{ts: ts.UTC(), kind: parts[1]}
		if len(parts) == 3 {
			ev.task = parts[2]
		}
		switch ev.kind {
		case "session_start", "session_pause", "session_resume", "session_stop", "focus_alert_show":
			events = append(events, ev)
		}
	}
	return events, sc.Err()
}
