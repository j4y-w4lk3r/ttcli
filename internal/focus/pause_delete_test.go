package focus_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func TestDeletePauseSpellRemovesLogLines(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	logDir := filepath.Join(dir, ".cache", "ttcli")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "" +
		"2026-09-11T10:44:56.031258028+02:00\tsession_start\ttask=\"desk + monitor\" duration=25m0s state=running\n" +
		"2026-09-11T10:52:37.586324617+02:00\tsession_pause\tdesk + monitor\n" +
		"2026-09-11T11:48:03.520410612+02:00\tsession_resume\tdesk + monitor\n" +
		"2026-09-11T12:05:21.967439144+02:00\tfocus_alert_show\ttask=\"desk + monitor\"\n"
	path := filepath.Join(logDir, "focus-session.log")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	spell := focus.PauseSpell{
		TaskTitle: "desk + monitor",
		Start:     mustUTC(t, "2026-09-11T10:52:37.586324617+02:00"),
		End:       mustUTC(t, "2026-09-11T11:48:03.520410612+02:00"),
	}
	if err := focus.DeletePauseSpell(spell); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	if strings.Contains(text, "session_pause") || strings.Contains(text, "session_resume") {
		t.Fatalf("pause lines remain: %q", text)
	}
	if !strings.Contains(text, "session_start") {
		t.Fatal("other log lines should remain")
	}
}

func mustUTC(t *testing.T, s string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t.Fatal(err)
	}
	return ts.UTC()
}
