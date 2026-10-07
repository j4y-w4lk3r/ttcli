package tasktext

import "testing"

func TestNotesRoundTrip(t *testing.T) {
	plain := "remember the brand\n1L"
	if got := Strip(ToHTML(plain)); got != plain {
		t.Fatalf("round trip=%q", got)
	}
}
