package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const (
	minTaskTitleColW     = 12
	maxTaskTitleColW     = 42
	taskTitleColFraction = 55 // percent of pane space left for titles
	taskSuffixGap        = 2
)

// taskRowLayout holds column metrics for one visible task block so due dates
// and focus stats align without sizing the title column to the longest title
// in the entire project.
type taskRowLayout struct {
	TitleColW  int // title text width at the deepest prefix in the window
	SuffixCol  int // visual column where focus+due block starts
	FocusColW  int
	MaxPrefixW int
}

func taskPrefixWidth(depth int) int {
	return lipgloss.Width(taskTreePrefix(depth)) + 2 // marker + space
}

func computeTaskRowLayout(rows []taskListRow, contentW, focusColW int) taskRowLayout {
	if contentW < 20 {
		contentW = 20
	}
	maxPrefixW := taskPrefixWidth(0)
	maxTitleW := 0
	for _, r := range rows {
		if pw := taskPrefixWidth(r.Depth); pw > maxPrefixW {
			maxPrefixW = pw
		}
		if tw := lipgloss.Width(displayText(r.Task.Title)); tw > maxTitleW {
			maxTitleW = tw
		}
	}

	suffixW := dueColWidth
	if focusColW > 0 {
		suffixW = focusColW + taskSuffixGap + dueColWidth
	}

	avail := contentW - maxPrefixW - suffixW - taskSuffixGap
	if avail < minTaskTitleColW {
		avail = minTaskTitleColW
	}

	cap := avail * taskTitleColFraction / 100
	if cap < minTaskTitleColW {
		cap = minTaskTitleColW
	}
	if cap > maxTaskTitleColW {
		cap = maxTaskTitleColW
	}
	if cap > avail {
		cap = avail
	}

	titleColW := maxTitleW
	if titleColW < minTaskTitleColW {
		titleColW = minTaskTitleColW
	}
	if titleColW > cap {
		titleColW = cap
	}
	if titleColW > avail {
		titleColW = avail
	}

	return taskRowLayout{
		TitleColW:  titleColW,
		SuffixCol:  maxPrefixW + titleColW + taskSuffixGap,
		FocusColW:  focusColW,
		MaxPrefixW: maxPrefixW,
	}
}

func formatTaskRow(prefix, styledTitle, focusPart, due string, layout taskRowLayout, contentW int) string {
	leftTarget := layout.SuffixCol - taskSuffixGap
	if leftTarget < lipgloss.Width(prefix)+4 {
		leftTarget = lipgloss.Width(prefix) + 4
	}

	titleSlot := leftTarget - lipgloss.Width(prefix)
	if titleSlot < 4 {
		titleSlot = 4
	}
	title := styledTitle
	if lipgloss.Width(title) > titleSlot {
		title = truncateRenderedWidth(title, titleSlot)
	}

	left := padToWidth(prefix+title, leftTarget)
	suffix := focusPart + due
	line := left + strings.Repeat(" ", taskSuffixGap) + suffix
	return truncateRenderedWidth(line, contentW)
}
