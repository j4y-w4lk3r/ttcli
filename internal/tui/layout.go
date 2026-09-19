package tui

import (
	"strings"
)

type layout struct {
	termW      int // terminal columns
	termH      int // terminal rows
	leftW      int // list pane inner content width (leftBoxW - 4)
	rightW     int // tasks pane inner content width (rightBoxW - 4)
	leftBoxW   int // bordered pane width in terminal columns
	rightBoxW  int
	fullW      int // full-width pane inner content (termW - 4)
	bodyLines  int
	innerLines int
	itemRows   int
	narrow     bool
}

const (
	headerRows = 2
	footerRows = 1
)

func (m model) layout() layout {
	w, h := m.width, m.height
	if w < 1 {
		w = 80
	}
	if h < 1 {
		h = 24
	}

	// header + body + footer = termH (fixed row budget)
	bodyLines := h - headerRows - footerRows
	if bodyLines < 4 {
		bodyLines = 4
	}

	innerLines := bodyLines - 2
	if innerLines < 2 {
		innerLines = 2
	}

	itemRows := innerLines - 3
	if itemRows < 1 {
		itemRows = 1
	}

	narrow := w < 76

	// Fixed column widths: left box + gap + right box must equal w exactly.
	maxLeftW := 32
	leftPaneCols := w / 3
	if m.view == viewPomodoro {
		// Ring + legend need room for task title and unclaimed duration on the right.
		maxLeftW = 48
		leftPaneCols = w * 2 / 5
	}
	leftW := leftPaneCols - 4
	if leftW < 18 {
		leftW = 18
	}
	if leftW > maxLeftW {
		leftW = maxLeftW
	}
	leftBoxW := leftW + 2
	rightBoxW := w - leftBoxW - 1
	if rightBoxW < 30 {
		rightBoxW = 30
		leftBoxW = w - rightBoxW - 1
		leftW = leftBoxW - 4
	}

	if narrow {
		leftBoxW = w
		rightBoxW = w
	}

	return layout{
		termW:      w,
		termH:      h,
		leftW:      leftBoxW - 4,
		rightW:     rightBoxW - 4,
		leftBoxW:   leftBoxW,
		rightBoxW:  rightBoxW,
		fullW:      w - 4,
		bodyLines:  bodyLines,
		innerLines: innerLines,
		itemRows:   itemRows,
		narrow:     narrow,
	}
}

func fitLines(content string, n int) string {
	if n < 1 {
		n = 1
	}
	lines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func lineCount(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func renderPane(content string, l layout, boxW int, focused bool) string {
	if boxW < 4 {
		boxW = 4
	}
	// Border (2) + horizontal padding (2) on paneBorder.
	contentW := boxW - 4
	inner := clipInnerContent(fitLines(content, l.innerLines), contentW)
	style := paneBorder
	if focused {
		style = paneFocusBorder
	}
	// lipgloss Width is the inner measure; bordered output is boxW (= Width+2).
	// Never combine Width with MaxWidth on a border — MaxWidth clips off ╮ │ ╯.
	rendered := style.
		Width(boxW - 2).
		MaxHeight(l.bodyLines).
		Render(inner)
	return fitPaneLines(rendered, l.bodyLines, boxW)
}

func fitPaneLines(s string, n, boxW int) string {
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = truncateRenderedWidth(padToWidth(line, boxW), boxW)
	}
	return strings.Join(lines, "\n")
}

// fitView forces every row to the terminal size so the emulator never soft-wraps.
func fitView(s string, width, height int) string {
	if width < 1 {
		width = 80
	}
	if height < 1 {
		height = 24
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = padToWidth(line, width)
	}
	return strings.Join(lines, "\n")
}

// clipInnerContent ensures every line fits the pane content area so lipgloss does
// not wrap rows inside the border (which steals lines from MaxHeight and breaks the frame).
func clipInnerContent(s string, maxW int) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = padToWidth(truncateRenderedWidth(line, maxW), maxW)
	}
	return strings.Join(lines, "\n")
}

func padBlockToSize(s string, height, width int) string {
	if height < 1 {
		height = 1
	}
	if width < 1 {
		width = 1
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if s == "" {
		lines = nil
	}
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	for i, line := range lines {
		lines[i] = padToWidth(truncateRenderedWidth(line, width), width)
	}
	return strings.Join(lines, "\n")
}

func (l layout) maxScrollRows() int {
	return paneScrollRows(l.innerLines, 0)
}

func (l layout) withBoxW(boxW int) layout {
	l2 := l
	l2.rightBoxW = boxW
	if boxW < 4 {
		boxW = 4
	}
	l2.rightW = boxW - 4
	return l2
}

const taskDetailMinRightBoxW = 72

func taskDetailSplitWidths(rightBoxW int) (listBoxW, detailBoxW int) {
	if rightBoxW < taskDetailMinRightBoxW {
		return 0, 0
	}
	detailBoxW = rightBoxW * 2 / 5
	if detailBoxW < 28 {
		detailBoxW = 28
	}
	if detailBoxW > 52 {
		detailBoxW = 52
	}
	listBoxW = rightBoxW - detailBoxW - 1
	if listBoxW < 32 {
		return 0, 0
	}
	return listBoxW, detailBoxW
}
