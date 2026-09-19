package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

type focusSlice struct {
	Title string
	Secs  int
	Color lipgloss.Color
	Kind  pomoSessionKind
}

func aggregatePauseByTask(pauses []focus.PauseSpell, now time.Time) []focusSlice {
	byTitle := map[string]int{}
	for _, p := range pauses {
		title := focus.PauseLegendTitle(p.TaskTitle)
		secs := int(p.Duration(now).Seconds() + 0.5)
		if secs < 1 && p.WorkBefore > 0 {
			secs = 1
		}
		byTitle[title] += secs
	}
	var out []focusSlice
	for title, secs := range byTitle {
		if secs < 1 {
			continue
		}
		out = append(out, focusSlice{
			Title: title,
			Secs:  secs,
			Color: colorBlue,
			Kind:  pomoSessionPause,
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Secs != out[b].Secs {
			return out[a].Secs > out[b].Secs
		}
		return out[a].Title < out[b].Title
	})
	return out
}

func buildLegendSlices(records []ticktick.FocusRecord, pauses []focus.PauseSpell, now time.Time) []focusSlice {
	slices := aggregateFocusByTask(records)
	for i := range slices {
		if isUnclaimedFocusTitle(slices[i].Title) {
			slices[i].Kind = pomoSessionUnclaimed
		} else {
			slices[i].Kind = classifyPomoSession(slices[i].Title, max(1, slices[i].Secs/60))
		}
	}
	slices = append(slices, aggregatePauseByTask(pauses, now)...)
	sort.Slice(slices, func(i, j int) bool {
		if slices[i].Secs != slices[j].Secs {
			return slices[i].Secs > slices[j].Secs
		}
		return slices[i].Title < slices[j].Title
	})
	return slices
}

func totalPauseSecs(pauses []focus.PauseSpell, now time.Time) int {
	n := 0
	for _, p := range pauses {
		n += int(p.Duration(now).Seconds() + 0.5)
	}
	return n
}

var ringPalette = []lipgloss.Color{
	colorPeach, colorTeal, colorMauve, colorGreen, colorBlue, colorYellow, colorPink, colorRed,
}

func aggregateFocusByTask(records []ticktick.FocusRecord) []focusSlice {
	byTitle := map[string]int{}
	for _, r := range records {
		title := r.TaskTitle()
		if title == "" {
			title = "(untitled)"
		}
		secs := int(focusRecordDuration(r).Seconds() + 0.5)
		if secs < 1 {
			secs = 1
		}
		byTitle[title] += secs
	}
	var out []focusSlice
	for title, secs := range byTitle {
		kind := pomoSessionFull
		if focus.IsOvertimeTitle(title) {
			kind = pomoSessionUnclaimed
		} else {
			kind = classifyPomoSession(title, max(1, secs/60))
		}
		out = append(out, focusSlice{
			Title: title,
			Secs:  secs,
			Color: ringColorForTitle(title),
			Kind:  kind,
		})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Secs != out[b].Secs {
			return out[a].Secs > out[b].Secs
		}
		return out[a].Title < out[b].Title
	})
	return out
}

// ringColorForTitle returns a stable slice color for a task (independent of map iteration order).
func ringColorForTitle(title string) lipgloss.Color {
	if focus.IsOvertimeTitle(title) {
		return colorRed
	}
	var h uint32
	for _, r := range title {
		h = h*31 + uint32(r)
	}
	return ringPalette[int(h%uint32(len(ringPalette)))]
}

func taskColorMap(slices []focusSlice) map[string]lipgloss.Color {
	m := make(map[string]lipgloss.Color, len(slices))
	for _, s := range slices {
		m[s.Title] = s.Color
	}
	return m
}

func totalFocusSecs(slices []focusSlice) int {
	n := 0
	for _, s := range slices {
		n += s.Secs
	}
	return n
}

func formatFocusTotal(secs int) string {
	if secs <= 0 {
		return "0m"
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm", m)
	}
	return fmt.Sprintf("%ds", secs)
}

// renderFocusRing draws a compact segmented arc with focus or live-session
// progress in the outline and the primary timer value in its center.
func renderFocusRing(slices []focusSlice, pauses []focus.PauseSpell, now time.Time, width int) []string {
	var live *focus.Session
	if dateKey(now) == dateKey(time.Now()) {
		if sess, err := focus.Load(); err == nil && sess.Active() {
			live = sess
		}
	}
	return renderSegmentedFocusArc(slices, pauses, now, width, live)
}

func renderFocusLegend(slices []focusSlice, width, cursor, maxLines int) []string {
	if len(slices) == 0 {
		return []string{truncateInner(hintStyle.Render("  no focus yet today"), width)}
	}
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(slices) {
		cursor = len(slices) - 1
	}
	if maxLines < 1 {
		maxLines = 1
	}
	win := computeScrollWindow(cursor, len(slices), maxLines)
	var lines []string
	if hint := renderScrollHint(win); hint != "" {
		lines = append(lines, truncateInner(hintStyle.Render("  "+hint), width))
	}
	totalSecs := totalFocusSecs(slices)
	for i := win.Start; i < win.End; i++ {
		lines = append(lines, renderFocusLegendRow(slices[i], width, i == cursor, totalSecs))
	}
	return lines
}

func renderFocusLegendRow(s focusSlice, width int, selected bool, totalSecs int) string {
	if s.Kind == pomoSessionPause || focus.IsPauseLegendTitle(s.Title) {
		titleSt := pauseTaskStyle
		if selected {
			titleSt = pauseTaskStyle.Copy().Bold(true)
		}
		title := focus.PauseLegendBaseTitle(s.Title)
		maxTitleW := width - 16
		if maxTitleW < 8 {
			maxTitleW = 8
		}
		if lipgloss.Width(title) > maxTitleW {
			title = truncateRunes(title, max(1, maxTitleW-1)) + "…"
		}
		indicator := renderPauseMark()
		left := "  " + indicator + " " + titleSt.Render(title)
		mins := max(1, s.Secs/60)
		right := pauseLegendDurationCluster(mins, s.Secs, totalSecs, selected)
		line := truncateInner(alignRightInWidth(left, right, width), width)
		if selected {
			return highlightPomoTimelineRow(line, true)
		}
		return line
	}
	if isUnclaimedFocusTitle(s.Title) {
		titleSt := unclaimedTaskStyle
		if selected {
			titleSt = unclaimedTaskStyle.Copy().Bold(true)
		}
		title := unclaimedBaseTitle(s.Title)
		maxTitleW := width - 16
		if maxTitleW < 8 {
			maxTitleW = 8
		}
		if lipgloss.Width(title) > maxTitleW {
			title = truncateRunes(title, max(1, maxTitleW-1)) + "…"
		}
		indicator := renderUnclaimedMark()
		left := "  " + indicator + " " + titleSt.Render(title)
		mins := max(1, s.Secs/60)
		right := unclaimedLegendDurationCluster(mins, s.Secs, totalSecs, selected)
		line := truncateInner(alignRightInWidth(left, right, width), width)
		if selected {
			return highlightPomoTimelineRow(line, true)
		}
		return line
	}
	dot := lipgloss.NewStyle().Foreground(s.Color).Render("●")
	titleSt := listIdleStyle
	if selected {
		titleSt = listSelStyle
	}
	title := s.Title
	maxTitleW := width - 18
	if maxTitleW < 8 {
		maxTitleW = 8
	}
	if lipgloss.Width(title) > maxTitleW {
		title = truncateRunes(title, max(1, maxTitleW-1)) + "…"
	}
	left := "  " + dot + " " + titleSt.Render(title)
	mins := max(1, s.Secs/60)
	right := focusLegendDurationCluster(s.Title, mins, s.Secs, totalSecs, selected, s.Color)
	line := truncateInner(alignRightInWidth(left, right, width), width)
	if selected {
		return highlightPomoTimelineRow(line, true)
	}
	return line
}

func pauseLegendDurationCluster(mins, secs, totalSecs int, selected bool) string {
	const barW = 4
	filled := 1
	if totalSecs > 0 {
		filled = secs * barW / totalSecs
		if filled < 1 {
			filled = 1
		}
		if filled > barW {
			filled = barW
		}
	}
	barSt := lipgloss.NewStyle().Foreground(colorBlue)
	emptySt := dayEmptySlotStyle
	if selected {
		barSt = barSt.Bold(true)
	}
	bar := barSt.Render(strings.Repeat("▫", filled)) + emptySt.Render(strings.Repeat("▯", barW-filled))
	return bar + " " + pomoDurationBadgeStyled(mins, pomoSessionPause, colorBlue, selected)
}

func unclaimedLegendDurationCluster(mins, secs, totalSecs int, selected bool) string {
	const barW = 4
	filled := 1
	if totalSecs > 0 {
		filled = secs * barW / totalSecs
		if filled < 1 {
			filled = 1
		}
		if filled > barW {
			filled = barW
		}
	}
	barSt := lipgloss.NewStyle().Foreground(colorRed)
	emptySt := dayEmptySlotStyle
	if selected {
		barSt = barSt.Bold(true)
	}
	bar := barSt.Render(strings.Repeat("▪", filled)) + emptySt.Render(strings.Repeat("▯", barW-filled))
	return bar + " " + pomoDurationBadgeStyled(mins, pomoSessionUnclaimed, colorRed, selected)
}

func focusLegendDurationCluster(title string, mins, secs, totalSecs int, selected bool, taskColor lipgloss.Color) string {
	const barW = 4
	filled := 1
	if totalSecs > 0 {
		filled = secs * barW / totalSecs
		if filled < 1 {
			filled = 1
		}
		if filled > barW {
			filled = barW
		}
	}
	kind := classifyPomoSession(title, mins)
	barSt := pomoBarStyle
	emptySt := dayEmptySlotStyle
	fillChar := "▮"
	switch kind {
	case pomoSessionUnclaimed:
		barSt = lipgloss.NewStyle().Foreground(colorRed)
		fillChar = "▪"
	case pomoSessionPartial:
		barSt = lipgloss.NewStyle().Foreground(colorYellow)
	default:
		if taskColor != "" {
			barSt = lipgloss.NewStyle().Foreground(taskColor)
		}
	}
	if selected {
		barSt = barSt.Copy().Bold(true)
	}
	bar := barSt.Render(strings.Repeat(fillChar, filled)) + emptySt.Render(strings.Repeat("▯", barW-filled))
	return bar + " " + pomoDurationBadgeStyled(mins, kind, taskColor, selected)
}

func alignRightInWidth(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return truncateInner(left+strings.Repeat(" ", gap)+right, width)
}

func liveClockRight(label string, nowLabel string, width int) string {
	return alignRightInWidth(headerStyle.Render(label), pomoTimeStyle.Render(nowLabel), width)
}

func formatLiveClock(t time.Time) string {
	return t.Format("15:04:05")
}
