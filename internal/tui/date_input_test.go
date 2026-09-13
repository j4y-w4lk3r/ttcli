package tui

import (
	"testing"
	"time"
)

func TestFormatDateInput(t *testing.T) {
	if got := formatDateInput("01092026"); got != "01/09/2026" {
		t.Fatalf("got %q", got)
	}
	if got := formatDateInput("01/09/2026"); got != "01/09/2026" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatDateInputPreserveCursor(t *testing.T) {
	formatted, cur := formatDateInputPreserveCursor("31/10/2026", 10)
	if formatted != "31/10/2026" || cur != 10 {
		t.Fatalf("end edit got %q cursor=%d", formatted, cur)
	}
	formatted, cur = formatDateInputPreserveCursor("31/10/202", 9)
	if formatted != "31/10/202" || cur != 9 {
		t.Fatalf("partial got %q cursor=%d", formatted, cur)
	}
	formatted, cur = formatDateInputPreserveCursor("01112026", 8)
	if formatted != "01/11/2026" || cur != 10 {
		t.Fatalf("typed digits got %q cursor=%d", formatted, cur)
	}
}

func TestParseDueDateInput(t *testing.T) {
	d, err := parseDueDateInput("01/09/2026")
	if err != nil {
		t.Fatal(err)
	}
	if d.Day() != 1 || d.Month() != time.September || d.Year() != 2026 {
		t.Fatalf("parsed=%v", d)
	}
	_, err = parseDueDateInput("2026-09-01")
	if err != nil {
		t.Fatal("legacy ISO should work:", err)
	}
}
