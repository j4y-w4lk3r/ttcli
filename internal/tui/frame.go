package tui

import "strings"

// terminalRow forces one physical terminal row: no embedded newlines, exact width.
func terminalRow(s string, w int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if w < 1 {
		return s
	}
	return padToWidth(s, w)
}

// composeView assembles exactly termH rows of termW columns each.
func composeView(header, body, footer string, termW, termH, bodyLines int) string {
	if termH < 1 {
		termH = 1
	}
	if bodyLines < 0 {
		bodyLines = 0
	}
	rows := make([]string, termH)

	headerParts := splitLinesPad(header, headerRows)
	for i := 0; i < headerRows && i < termH; i++ {
		rows[i] = fillBarRow(headerParts[i], termW, headerBarStyle)
	}

	bodyParts := splitLinesPad(body, bodyLines)
	for i := 0; i < bodyLines; i++ {
		idx := headerRows + i
		if idx >= termH-footerRows {
			break
		}
		rows[idx] = truncateRenderedWidth(terminalRow(bodyParts[i], termW), termW)
	}

	if footerRows > 0 && termH > 0 {
		footerParts := splitLinesPad(footer, footerRows)
		rows[termH-1] = fillBarRow(footerParts[0], termW, statusBarStyle)
	}

	return strings.Join(rows, "\n")
}
