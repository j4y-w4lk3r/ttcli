package tasktext

import "testing"

func TestNotesRoundTrip(t *testing.T) {
	plain := "remember the brand\n1L"
	if got := ToHTML(plain); got != plain {
		t.Fatalf("stored=%q", got)
	}
	if got := Strip(ToHTML(plain)); got != plain {
		t.Fatalf("round trip=%q", got)
	}
	if got := Strip("remember the brand<br/>1L"); got != plain {
		t.Fatalf("legacy br=%q", got)
	}
}
