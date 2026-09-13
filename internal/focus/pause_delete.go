package focus

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const pauseLogTimeSlack = 2 * time.Second

// DeletePauseSpell removes a pause/resume pair from the local focus event log.
func DeletePauseSpell(spell PauseSpell) error {
	if spell.Start.IsZero() || spell.End.IsZero() {
		return fmt.Errorf("cannot delete an in-progress pause")
	}
	path, err := focusEventLogPath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("pause log not found")
		}
		return err
	}
	lines := strings.Split(string(b), "\n")
	var kept []string
	removed := 0
	for _, line := range lines {
		if line == "" {
			continue
		}
		if dropPauseLogLine(line, spell) {
			removed++
			continue
		}
		kept = append(kept, line)
	}
	if removed == 0 {
		return fmt.Errorf("pause entry not found in log")
	}
	out := strings.Join(kept, "\n")
	if len(kept) > 0 {
		out += "\n"
	}
	if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
		return err
	}
	return removePauseSpellFromStore(spell)
}

func dropPauseLogLine(line string, spell PauseSpell) bool {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) < 2 {
		return false
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return false
	}
	task := ""
	if len(parts) == 3 {
		task = normalizeLogTask(parts[2])
	}
	if !tasksMatch(task, spell.TaskTitle) {
		return false
	}
	switch parts[1] {
	case "session_pause":
		d := ts.UTC().Sub(spell.Start)
		return d >= -pauseLogTimeSlack && d <= pauseLogTimeSlack
	case "session_resume":
		d := ts.UTC().Sub(spell.End)
		return d >= -pauseLogTimeSlack && d <= pauseLogTimeSlack
	default:
		return false
	}
}

func removePauseSpellFromStore(spell PauseSpell) error {
	path, err := pauseStorePath()
	if err != nil {
		return err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var store pauseStoreFile
	if err := json.Unmarshal(b, &store); err != nil {
		return nil
	}
	filtered := store.Spells[:0]
	for _, s := range store.Spells {
		if s.Start.Equal(spell.Start) && s.End.Equal(spell.End) && tasksMatch(s.TaskTitle, spell.TaskTitle) {
			continue
		}
		filtered = append(filtered, s)
	}
	if len(filtered) == len(store.Spells) {
		return nil
	}
	store.Spells = filtered
	nb, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, nb, 0o644)
}
