package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func parseFocusTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	t, err := ticktick.ParseAPITime(s)
	if err != nil && len(s) >= 16 {
		t, err = time.Parse("2006-01-02T15:04", s[:16])
	}
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func focusTimeLocal(s string) (time.Time, bool) {
	t, ok := parseFocusTime(s)
	if !ok {
		return time.Time{}, false
	}
	return t.Local(), true
}

func focusRecordClock(r ticktick.FocusRecord) string {
	if t, ok := focusTimeLocal(r.StartTime); ok {
		return t.Format("15:04")
	}
	return "??:??"
}

func focusRecordEndClock(r ticktick.FocusRecord) string {
	if t, ok := focusTimeLocal(r.EndTime); ok {
		return t.Format("15:04")
	}
	return ""
}

func timelineSectionHeader(records []ticktick.FocusRecord, viewDate time.Time, width int) string {
	totalMins := 0
	full := ticktick.CountFullPomos(records)
	for _, r := range records {
		totalMins += int(ticktick.RecordDuration(r).Minutes() + 0.5)
	}
	day := pomoTimelineDayLabel(viewDate)
	label := headerStyle.Render(iconPomodoro + " Timeline")
	var meta string
	if dateKey(viewDate) == dateKey(time.Now()) {
		meta = hintStyle.Render(fmt.Sprintf("%d full · %d logged · %dm focused today", full, len(records), totalMins))
	} else {
		meta = hintStyle.Render(fmt.Sprintf("%d full · %d logged · %dm", full, len(records), totalMins))
	}
	return truncateInner(day+"  "+label+"  "+meta, width)
}

func focusRecordDuration(r ticktick.FocusRecord) time.Duration {
	st, ok1 := focusTimeLocal(r.StartTime)
	et, ok2 := focusTimeLocal(r.EndTime)
	if !ok1 || !ok2 || !et.After(st) {
		return 25 * time.Minute
	}
	return et.Sub(st)
}

const (
	dayHourStep = 2
	dayTimeColW = 5
)

type dayGridRow struct {
	hour          int
	hourLabel     string
	kind          string // "slot", "pomo", "pause", "now", "live", "gap"
	inHourGrid    bool
	slotBusy      bool
	slotSummary   slotSummary
	recIdx        int
	rec           ticktick.FocusRecord
	pauseIdx      int
	pause         focus.PauseSpell
	pomoSlotIndex int
	pomoSlotCount int
}

func slotFocusMins(items []indexedFocus) int {
	total := 0
	for _, item := range items {
		total += int(focusRecordDuration(item.rec).Minutes() + 0.5)
	}
	return total
}

func buildDayGrid(records []ticktick.FocusRecord, pauses []focus.PauseSpell, now time.Time, live *focus.Session, showNow bool) []dayGridRow {
	return buildPomoHourGrid(records, pauses, now, live, showNow)
}

func dayGridTimeCol(label string) string {
	if label == "" {
		return hintStyle.Render(strings.Repeat(" ", dayTimeColW))
	}
	return pomoTimeStyle.Render(fmt.Sprintf("%-*s", dayTimeColW, label))
}

func renderDayGridRow(row dayGridRow, now time.Time, selected bool, width int, taskColors map[string]lipgloss.Color, live *focus.Session, layout pomoTimelineLayout) string {
	switch row.kind {
	case "slot":
		return renderHourSlotRow(row.hourLabel, row.slotBusy, width, selected)
	case "gap":
		return renderHourSlotGapRow(width, row.slotBusy, selected)
	case "now":
		label := row.hourLabel
		if label == "" {
			label = now.Format("15:04")
		}
		return renderHourNowRow(label, width, selected)
	case "live":
		title := ""
		if live != nil {
			title = live.TaskTitle
		}
		col := taskColors[title]
		if col == "" {
			col = colorPeach
		}
		if row.inHourGrid {
			return renderHourLiveRow(live, col, width, layout, selected)
		}
		return renderDayLiveRow(now.Format("15:04"), live, col, width, layout)
	case "pause":
		if row.inHourGrid {
			return renderHourPauseRow(row.pause, selected, width, layout, now)
		}
		return ""
	default:
		title := row.rec.TaskTitle()
		if title == "" {
			title = "(untitled)"
		}
		col := taskColors[title]
		if col == "" {
			col = colorMuted
		}
		mins := int(focusRecordDuration(row.rec).Minutes() + 0.5)
		start := focusRecordClock(row.rec)
		end := focusRecordEndClock(row.rec)
		if end == start {
			end = ""
		}
		if row.inHourGrid {
			return renderHourPomoRow(
				row.pomoSlotIndex,
				row.pomoSlotCount,
				selected,
				title,
				col,
				start,
				end,
				mins,
				width,
				layout,
			)
		}
		return renderDayPomoRow(
			start,
			row.pomoSlotIndex,
			row.pomoSlotCount,
			selected,
			title,
			col,
			end,
			mins,
			width,
			layout,
		)
	}
}

func (m model) pomoDayGrid() []dayGridRow {
	l := m.layout()
	return m.pomoDayGridFor(l.innerLines)
}

func gridRowForRecord(grid []dayGridRow, recIdx int) int {
	for i, row := range grid {
		if row.kind == "pomo" && row.recIdx == recIdx {
			return i
		}
	}
	return 0
}

func sectionHeader(title string, width int) string {
	label := headerStyle.Render(title)
	ruleLen := dayRailWidth(width)
	if ruleLen > 32 {
		ruleLen = 32
	}
	rule := sectionRuleStyle.Render(strings.Repeat("─", ruleLen))
	return truncateInner(label+"  "+rule, width)
}

func renderCompletedTaskLine(t ticktick.Task, width int) string {
	clock := pomoTimeStyle.Render(completedTaskClock(t))
	check := taskDoneStyle.Render(iconCheck)
	title := taskDoneStyle.Render(t.Title)
	return truncateInner(clock+" "+check+" "+title, width)
}

func completedTaskClock(t ticktick.Task) string {
	if t, ok := focusTimeLocal(t.CompletedT); ok {
		return t.Format("15:04")
	}
	return ""
}

func (m model) renderPomodoroView(l layout) string {
	if l.narrow {
		focusLines, timelineLines := pomoStackHeights(l.innerLines)
		var b strings.Builder
		b.WriteString(m.renderPomoFocusPanel(l.fullW, focusLines))
		b.WriteString("\n")
		b.WriteString(m.renderPomoTimelinePanel(l.fullW, timelineLines))
		content := fitLines(b.String(), l.innerLines)
		return renderPane(content, l, l.termW, false)
	}
	leftPane := renderPane(m.renderPomoFocusPanel(l.leftW, l.innerLines), l, l.leftBoxW, false)
	rightPane := renderPane(m.renderPomoTimelinePanel(l.rightW, l.innerLines), l, l.rightBoxW, false)
	return joinHorizPanes(leftPane, rightPane, l.bodyLines, l.leftBoxW, l.rightBoxW, l.termW)
}

// pomoStackHeights splits vertical space for narrow terminals; timeline gets ~68%.
func pomoStackHeights(innerLines int) (focusLines, timelineLines int) {
	if innerLines < 12 {
		return innerLines / 3, innerLines - innerLines/3
	}
	focusLines = innerLines * 32 / 100
	if focusLines < 9 {
		focusLines = 9
	}
	timelineLines = innerLines - focusLines
	minTimeline := innerLines * 60 / 100
	if timelineLines < minTimeline {
		timelineLines = minTimeline
		focusLines = innerLines - timelineLines
	}
	return focusLines, timelineLines
}

func (m model) renderPomoDoneSection(w int) []string {
	if len(m.todayCompleted) == 0 {
		return nil
	}
	var lines []string
	lines = append(lines, truncateInner(sectionHeader(pomoDoneSectionTitle(m.pomoViewDate), w), w))
	maxDone := 2
	if len(m.todayCompleted) < maxDone {
		maxDone = len(m.todayCompleted)
	}
	for i := 0; i < maxDone; i++ {
		lines = append(lines, renderCompletedTaskLine(m.todayCompleted[i], w))
	}
	if len(m.todayCompleted) > maxDone {
		lines = append(lines, truncateInner(hintStyle.Render(fmt.Sprintf("  … +%d more", len(m.todayCompleted)-maxDone)), w))
	}
	return lines
}

func (m model) renderPomoFocusPanel(w, innerLines int) string {
	var lines []string

	sess, err := focus.Load()
	if err == nil && sess.Active() {
		if sess.State == focus.StateAwaitingDismiss {
			if sess.InOvertimeGrace() {
				lines = append(lines, truncateInner(timerBigStyle.Render(formatClock(sess.OvertimeGraceRemaining())), w))
				lines = append(lines, truncateInner(hintStyle.Render("dismiss now — no unclaimed time · S stop & log"), w))
			} else {
				lines = append(lines, truncateInner(lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render(formatClock(sess.OvertimeElapsed())), w))
				lines = append(lines, truncateInner(renderUnclaimedLiveHint(), w))
			}
			if sess.TaskTitle != "" {
				titleSt := taskSelStyle
				if !sess.InOvertimeGrace() {
					lines = append(lines, truncateInner(renderUnclaimedMark()+" "+unclaimedTaskStyle.Render(sess.TaskTitle), w))
				} else {
					lines = append(lines, truncateInner(titleSt.Render(iconTaskOpen+" "+sess.TaskTitle), w))
				}
			}
			if !sess.InOvertimeGrace() {
				lines = append(lines, truncateInner(errStyle.Render(iconCheck+" planned block done — S stop & log · D dismiss notify"), w))
			}
		} else {
			lines = append(lines, truncateInner(timerBigStyle.Render(formatClock(sess.Remaining())), w))
			detailParts := []string{sess.State, formatClock(sess.Elapsed()) + " elapsed"}
			detailParts = appendFocusPauseDetail(detailParts, sess)
			lines = append(lines, truncateInner(timerStyle.Render(strings.Join(detailParts, " · ")), w))
			if sess.TaskTitle != "" {
				lines = append(lines, truncateInner(taskSelStyle.Render(iconTaskOpen+" "+sess.TaskTitle), w))
			}
			if sess.Finished() {
				lines = append(lines, truncateInner(errStyle.Render(iconCheck+" complete — S stop · D dismiss notify"), w))
			}
		}
		lines = append(lines, "")
	}

	if m.focusStats != nil {
		records := m.focusStatsRecords()
		anchor := m.pomoTimelineAnchor()
		var live *focus.Session
		if m.pomoViewIsToday() {
			live, _ = focus.Load()
		}
		pauses, _ := focus.PauseSpellsForTimeline(m.pomoViewDate, live, anchor)
		focusSlices := aggregateFocusByTask(records)
		legendSlices := buildLegendSlices(records, pauses, anchor)
		lines = append(lines, truncateInner(sectionHeader(pomoDaySectionTitle(m.pomoViewDate), w), w))
		lines = append(lines, renderFocusRing(focusSlices, pauses, anchor, w)...)
		if done := m.renderPomoDoneSection(w); len(done) > 0 {
			lines = append(lines, "")
			lines = append(lines, done...)
		}
		legendBudget := innerLines - len(lines) - 2
		if legendBudget < 1 {
			legendBudget = 1
		}
		if legendBudget > 4 {
			legendBudget = 4
		}
		lines = append(lines, renderFocusLegend(legendSlices, w, m.pomoLegendCursor, legendBudget)...)
	}

	if m.mode == modeRenamePomo {
		lines = append(lines, "", m.renameInput.View())
		if h := m.keyHint("enter save · esc cancel"); h != "" {
			lines = append(lines, h)
		}
	}

	for len(lines) < innerLines-1 {
		lines = append(lines, "")
	}
	if len(lines) > innerLines-1 {
		lines = lines[:innerLines-1]
	}
	if h := m.keyHint("h/l legend · j/k timeline · e rename · n add · x delete · s/f start · p pause"); h != "" {
		lines = append(lines, truncateInner(h, w))
	}
	return strings.Join(lines, "\n")
}

func (m model) renderPomoTimelinePanel(w, innerLines int) string {
	contentW := pomoTimelineContentW(w)
	anchor := m.pomoTimelineAnchor()
	showNow := m.pomoViewIsToday()
	var lines []string

	if m.focusStats == nil {
		lines = append(lines, truncateInner(hintStyle.Render("(loading…)"), contentW))
		return fitLines(strings.Join(padTimelinePanelLines(lines, w, contentW), "\n"), innerLines)
	}

	records := m.focusStatsRecords()
	var live *focus.Session
	if showNow {
		live, _ = focus.Load()
	}
	pauses, _ := focus.PauseSpellsForTimeline(m.pomoViewDate, live, anchor)
	compactGrid := buildDayGrid(records, pauses, anchor, live, showNow)
	taskColors := taskColorMap(aggregateFocusByTask(records))
	layout := computePomoTimelineLayout(compactGrid, dayTimelineContentW(contentW), taskColors, live, anchor)
	lines = append(lines, truncateInner(timelineSectionHeader(records, m.pomoViewDate, contentW)+"  "+hintStyle.Render(m.pomoTimelineDensity().Label()), contentW))

	maxRows := m.pomoTimelineMaxRows(innerLines)
	grid := expandPomoGrid(compactGrid, m.pomoTimelineGapLines(innerLines))
	gridCursor := m.pomoTimelineGridCursor(grid)
	win := computeScrollWindow(gridCursor, len(grid), maxRows)
	gridBudget := maxRows
	if hint := m.scrollHint(win); hint != "" {
		lines = append(lines, truncateInner(hint, contentW))
		gridBudget--
	}
	var gridLines []string
	tailBusy := false
	for i := win.Start; i < win.End; i++ {
		row := grid[i]
		if row.kind == "slot" || row.kind == "gap" {
			tailBusy = row.slotBusy
		}
		selected := i == m.pomoGridCursor
		gridLines = append(gridLines, renderDayGridRow(row, anchor, selected, contentW, taskColors, live, layout))
	}
	if m.pomoTimelineDensity() == PomoDensityStretch {
		gridLines = fillPomoTimelineStretch(gridLines, gridBudget, contentW, tailBusy)
	} else if len(gridLines) > gridBudget {
		gridLines = gridLines[:gridBudget]
	}
	lines = append(lines, gridLines...)

	if h := m.keyHint("[/] day · t today · z density · T switch · j/k scroll · e rename · n add · x delete"); h != "" {
		lines = append(lines, truncateInner(h, contentW))
	}
	return fitLines(strings.Join(padTimelinePanelLines(lines, w, contentW), "\n"), innerLines)
}

func pomoTimelineContentW(panelW int) int {
	return panelW
}

func padTimelinePanelLines(lines []string, panelW, contentW int) []string {
	if contentW >= panelW {
		out := make([]string, len(lines))
		for i, ln := range lines {
			out[i] = truncateInner(ln, panelW)
		}
		return out
	}
	out := make([]string, len(lines))
	for i, ln := range lines {
		body := truncateInner(ln, contentW)
		pad := panelW - lipgloss.Width(body)
		if pad < 0 {
			pad = 0
		}
		out[i] = body + strings.Repeat(" ", pad)
	}
	return out
}
