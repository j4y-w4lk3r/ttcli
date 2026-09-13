package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

// joinHorizPanes places two bordered panes side-by-side, forcing each column to a
// fixed visual width so the terminal never soft-wraps a row (which breaks TUI layout).
func joinHorizPanes(left, right string, height, leftCols, rightCols, totalW int) string {
	ll := splitLinesPad(left, height)
	rl := splitLinesPad(right, height)
	if totalW < 1 {
		totalW = leftCols + 1 + rightCols
	}
	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		line := terminalRow(ll[i], leftCols) + " " + terminalRow(rl[i], rightCols)
		b.WriteString(truncateRenderedWidth(terminalRow(line, totalW), totalW))
	}
	return b.String()
}

// fillBarRow extends a styled row to exactly w columns using the same bar background.
func fillBarRow(s string, w int, bar lipgloss.Style) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if w < 1 {
		return s
	}
	cur := lipgloss.Width(s)
	switch {
	case cur == w:
		return s
	case cur > w:
		return truncateRenderedWidth(s, w)
	default:
		return s + bar.Render(strings.Repeat(" ", w-cur))
	}
}

// padToWidth pads a rendered lipgloss/terminal row to an exact column count.
// Uses lipgloss.Width — ansi.StringWidth undercounts styled borders and causes soft-wrap.
func padToWidth(s string, w int) string {
	if w < 1 {
		return ""
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	cur := lipgloss.Width(s)
	switch {
	case cur == w:
		if runewidth.StringWidth(stripANSI(s)) <= w {
			return s
		}
		return truncateRenderedWidth(s, w)
	case cur > w:
		return truncateRenderedWidth(s, w)
	default:
		return s + strings.Repeat(" ", w-cur)
	}
}

// truncateRenderedWidth trims a rendered row to w columns, preserving ANSI state.
func truncateRenderedWidth(s string, w int) string {
	if w < 1 {
		return ""
	}
	out := s
	if lipgloss.Width(out) > w {
		out = ansi.Truncate(s, w, "")
		if lipgloss.Width(out) > w {
			for lipgloss.Width(out) > w && len(out) > 0 {
				out = out[:len(out)-1]
			}
		}
	}
	// lipgloss can undercount some emoji/VS sequences vs the terminal — verify plain width.
	for runewidth.StringWidth(stripANSI(out)) > w && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out
}

// truncateWidth shortens a styled string to max visual columns.
func truncateWidth(s string, maxW int) string {
	if maxW < 1 {
		return ""
	}
	return ansi.Truncate(s, maxW, "")
}

func splitLinesPad(s string, n int) []string {
	s = strings.TrimSuffix(s, "\n")
	var lines []string
	if s != "" {
		lines = strings.Split(s, "\n")
	}
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines
}

// displayText normalizes user-facing strings for single-row terminal cells.
func displayText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\uFE0F', '\uFE0E', '\uFEFF', '\u200B', '\u200C', '\u2060':
			// Variation selectors / zero-width chars break column alignment (e.g. "Haircut\uFE0F").
			continue
		default:
			b.WriteRune(r)
		}
	}
	return strings.TrimSpace(b.String())
}

// paneLine forces one terminal row — embedded control chars break pane layout.
func paneLine(s string) string {
	return displayText(s)
}

// fillInner builds exactly innerLines of pane content: 1 title, 1 hint, item rows.
func fillInner(title string, hint string, itemLines []string, innerLines int) string {
	lines := make([]string, innerLines)
	lines[0] = paneLine(title)
	if hint != "" {
		lines[1] = paneLine(hint)
	}
	start := 2
	maxItems := paneScrollRows(innerLines, 0)
	for j, item := range itemLines {
		if j >= maxItems || start+j >= innerLines {
			break
		}
		lines[start+j] = paneLine(item)
	}
	return strings.Join(lines, "\n")
}

// fillInnerWithFooter builds pane rows with a fixed-size footer block (e.g. task detail).
func fillInnerWithFooter(title, hint string, itemLines, footer []string, innerLines, footerLines int) string {
	lines := make([]string, innerLines)
	lines[0] = paneLine(title)
	if hint != "" {
		lines[1] = paneLine(hint)
	}
	const itemStart = 2 // always match fillInner / paneScrollRows hint slot
	if footerLines < 0 {
		footerLines = 0
	}
	maxItems := innerLines - itemStart - footerLines
	if maxItems < 0 {
		maxItems = 0
	}
	if maxItems > maxVisibleRows {
		maxItems = maxVisibleRows
	}
	for j, item := range itemLines {
		if j >= maxItems || itemStart+j >= innerLines {
			break
		}
		lines[itemStart+j] = paneLine(item)
	}
	start := innerLines - footerLines
	for j := 0; j < footerLines; j++ {
		idx := start + j
		if idx >= innerLines {
			break
		}
		line := ""
		if j < len(footer) {
			line = footer[j]
		}
		lines[idx] = paneLine(line)
	}
	return strings.Join(lines, "\n")
}

func padFooterLines(footer []string, lines int) []string {
	if lines < 1 {
		return nil
	}
	out := make([]string, lines)
	for i := 0; i < lines; i++ {
		if i < len(footer) {
			out[i] = footer[i]
		}
	}
	return out
}

// truncateInner shortens styled content to fit; never pads (padding breaks lipgloss borders).
func truncateInner(s string, maxW int) string {
	if maxW < 1 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	return ansi.Truncate(s, maxW, "…")
}

// clipLine truncates a styled string to max visual columns (single terminal row).
func clipLine(s string, maxW int) string {
	return truncateInner(s, maxW)
}

func lastRune(s string) rune {
	var r rune
	for _, c := range s {
		r = c
	}
	return r
}
