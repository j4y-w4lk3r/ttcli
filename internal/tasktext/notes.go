package tasktext

import "strings"

// Strip turns TickTick note HTML into plain text.
func Strip(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// ToHTML keeps plain-text notes with real line breaks. TickTick shows a
// literal <br/> when that tag is stored in the note, and notes written in
// the TickTick app already use newline characters. Strip still turns older
// <br/> notes into lines so the next save writes them back as newlines.
func ToHTML(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}
