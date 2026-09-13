package tui

import (
	"fmt"
	"strings"
	"time"
)

const addTaskDateLayout = "02/01/2006"

func formatDateInput(raw string) string {
	var digits []rune
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) > 8 {
		digits = digits[:8]
	}
	var b strings.Builder
	for i, r := range digits {
		if i == 2 || i == 4 {
			b.WriteByte('/')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// formatDateInputPreserveCursor auto-inserts slashes without losing edit position.
func formatDateInputPreserveCursor(raw string, cursor int) (string, int) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(raw) {
		cursor = len(raw)
	}
	digitsBefore := 0
	for i := 0; i < cursor && i < len(raw); i++ {
		if raw[i] >= '0' && raw[i] <= '9' {
			digitsBefore++
		}
	}
	formatted := formatDateInput(raw)
	if digitsBefore <= 0 {
		return formatted, 0
	}
	totalDigits := 0
	for i := 0; i < len(formatted); i++ {
		if formatted[i] >= '0' && formatted[i] <= '9' {
			totalDigits++
		}
	}
	if digitsBefore >= totalDigits {
		return formatted, len(formatted)
	}
	seen := 0
	for i := 0; i < len(formatted); i++ {
		if formatted[i] >= '0' && formatted[i] <= '9' {
			seen++
			if seen == digitsBefore {
				return formatted, i + 1
			}
		}
	}
	return formatted, len(formatted)
}

func parseDueDateInput(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, nil
	}
	if d, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return dateOnly(d), nil
	}
	s = formatDateInput(s)
	layouts := []string{
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
	}
	var lastErr error
	for _, layout := range layouts {
		d, err := time.ParseInLocation(layout, s, time.Local)
		if err == nil {
			return dateOnly(d), nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return time.Time{}, fmt.Errorf("due date: use DD/MM/YYYY")
	}
	return time.Time{}, fmt.Errorf("due date: use DD/MM/YYYY")
}
