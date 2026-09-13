package focus_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestPauseSpellsFromLogPairsPauseResume(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	logDir := filepath.Join(dir, ".cache", "ttcli")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logBody := "" +
		"2026-09-11T10:44:56.031258028+02:00\tsession_start\ttask=\"desk + monitor\" duration=25m0s state=running\n" +
		"2026-09-11T10:52:37.586324617+02:00\tsession_pause\tdesk + monitor\n" +
		"2026-09-11T11:48:03.520410612+02:00\tsession_resume\tdesk + monitor\n" +
		"2026-09-11T12:05:21.967439144+02:00\tfocus_alert_show\ttask=\"desk + monitor\"\n"
	if err := os.WriteFile(filepath.Join(logDir, "focus-session.log"), []byte(logBody), 0o644); err != nil {
		t.Fatal(err)
	}

	day := time.Date(2026, 9, 11, 12, 0, 0, 0, time.FixedZone("CEST", 2*3600))
	spells, err := focus.PauseSpellsForDay(day)
	if err != nil {
		t.Fatal(err)
	}
	if len(spells) != 1 {
		t.Fatalf("spells=%d want 1", len(spells))
	}
	spell := spells[0]
	if spell.TaskTitle != "desk + monitor" {
		t.Fatalf("task=%q", spell.TaskTitle)
	}
	d := spell.Duration(time.Now())
	if d < 54*time.Minute || d > 56*time.Minute {
		t.Fatalf("pause duration=%s want ~55m", d)
	}
	if spell.WorkBefore < 7*time.Minute || spell.WorkBefore > 9*time.Minute {
		t.Fatalf("work before=%s want ~8m", spell.WorkBefore)
	}
	if spell.WorkAfter < 16*time.Minute || spell.WorkAfter > 18*time.Minute {
		t.Fatalf("work after=%s want ~17m", spell.WorkAfter)
	}
}
