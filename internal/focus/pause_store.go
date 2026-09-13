package focus

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type pauseStoreFile struct {
	Spells []PauseSpell `json:"spells"`
}

func pauseStorePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "ttcli", "pause-spells.json"), nil
}

// PersistPauseSpells appends pause intervals for a logged pomodoro segment.
func PersistPauseSpells(spells []PauseSpell) error {
	if len(spells) == 0 {
		return nil
	}
	path, err := pauseStorePath()
	if err != nil {
		return err
	}
	var store pauseStoreFile
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &store)
	}
	store.Spells = append(store.Spells, spells...)
	sort.Slice(store.Spells, func(i, j int) bool {
		return store.Spells[i].Start.Before(store.Spells[j].Start)
	})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// PauseSpellsForDay returns pause intervals for the local calendar day.
func PauseSpellsForDay(day time.Time) ([]PauseSpell, error) {
	day = day.Local()
	var out []PauseSpell

	path, err := pauseStorePath()
	if err == nil {
		if b, err := os.ReadFile(path); err == nil {
			var store pauseStoreFile
			if json.Unmarshal(b, &store) == nil {
				for _, spell := range store.Spells {
					if sameLocalDay(spell.Start, day) {
						out = append(out, spell)
					}
				}
			}
		}
	}

	fromLog, err := pauseSpellsFromLog(day)
	if err != nil {
		return out, err
	}
	out = mergePauseSpells(out, fromLog)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Start.Before(out[j].Start)
	})
	return out, nil
}

func sameLocalDay(t, day time.Time) bool {
	lt := t.Local()
	ld := day.Local()
	return lt.Year() == ld.Year() && lt.Month() == ld.Month() && lt.Day() == ld.Day()
}

func pauseSpellKey(spell PauseSpell) string {
	return spell.TaskTitle + "|" + spell.Start.Format(time.RFC3339Nano) + "|" + spell.End.Format(time.RFC3339Nano)
}

func coalescePauseSpell(existing, incoming PauseSpell) PauseSpell {
	if incoming.WorkBefore > existing.WorkBefore {
		existing.WorkBefore = incoming.WorkBefore
	}
	if incoming.WorkAfter > existing.WorkAfter {
		existing.WorkAfter = incoming.WorkAfter
	}
	if existing.End.IsZero() && !incoming.End.IsZero() {
		existing.End = incoming.End
	}
	return existing
}

func mergePauseSpells(a, b []PauseSpell) []PauseSpell {
	byKey := map[string]PauseSpell{}
	order := make([]string, 0, len(a)+len(b))
	for _, list := range [][]PauseSpell{a, b} {
		for _, spell := range list {
			key := pauseSpellKey(spell)
			if existing, ok := byKey[key]; ok {
				byKey[key] = coalescePauseSpell(existing, spell)
				continue
			}
			byKey[key] = spell
			order = append(order, key)
		}
	}
	out := make([]PauseSpell, 0, len(order))
	for _, key := range order {
		out = append(out, byKey[key])
	}
	return out
}

// MergePauseSpells combines stored and live pause intervals without duplicates.
func MergePauseSpells(stored []PauseSpell, live []PauseSpell) []PauseSpell {
	return mergePauseSpells(stored, live)
}

// PauseSpellsForTimeline returns pause intervals for the day, enriched with live session work slices.
func PauseSpellsForTimeline(day time.Time, live *Session, now time.Time) ([]PauseSpell, error) {
	spells, err := PauseSpellsForDay(day)
	if err != nil {
		return nil, err
	}
	if live != nil && live.Active() {
		spells = MergePauseSpells(spells, live.ActivePauseSpells(now))
		spells = EnrichPauseWork(spells, live, now)
	}
	return spells, nil
}
