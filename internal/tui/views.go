package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m model) renderPomoBadge() string {
	count := m.todayPomos
	goal := m.uiSettings.pomoDailyGoal()
	label := fmt.Sprintf("%s %d/%d", iconPomodoro, count, goal)
	return pomoTodayStyle(count, goal).Render(label)
}

func (m model) renderNav() string {
	tabs := []struct {
		view appView
		icon string
		name string
	}{
		{viewTasks, iconTasks, "Tasks"},
		{viewCalendar, iconCalendar, "Calendar"},
		{viewPomodoro, iconPomodoro, "Pomo"},
		{viewHabits, iconHabit, "Habits"},
	}
	parts := make([]string, len(tabs))
	for i, t := range tabs {
		label := t.icon + " " + t.name
		if m.view == t.view {
			parts[i] = navActiveStyle.Render(label)
		} else {
			parts[i] = navIdleStyle.Render(label)
		}
	}
	return strings.Join(parts, " ")
}

func (m model) renderBrand() string {
	badge := titleStyle.Render(iconApp + " ttcli")
	sep := headerSubStyle.Render(" · ")
	sub := headerBrandStyle.Render("TickTick")
	return badge + sep + sub
}

func headerBarContentW(termW int) int {
	if termW < 2 {
		return 1
	}
	return termW - 1
}

func renderBarRow(content string, termW int) string {
	if termW < 1 {
		termW = 80
	}
	contentW := headerBarContentW(termW)
	content = strings.ReplaceAll(content, "\n", " ")
	content = truncateInner(content, contentW)
	return renderBarRowInner(content, termW)
}

// renderBarRowJoin places right immediately after left (brand + pomo badge).
func renderBarRowJoin(left, right string, termW int) string {
	if termW < 1 {
		termW = 80
	}
	contentW := headerBarContentW(termW)
	left = strings.ReplaceAll(left, "\n", " ")
	right = strings.ReplaceAll(right, "\n", " ")
	gap := "  "
	gw := lipgloss.Width(gap)
	rw := lipgloss.Width(right)
	leftMax := contentW - rw - gw
	if leftMax < 1 {
		leftMax = 1
	}
	left = truncateInner(left, leftMax)
	return renderBarRowInner(left+gap+right, termW)
}

func renderBarRowInner(content string, termW int) string {
	content = strings.ReplaceAll(content, "\n", " ")
	cw := headerBarContentW(termW)
	content = padToWidth(truncateInner(content, cw), cw)
	return headerBarStyle.Render(" " + strings.TrimPrefix(content, " "))
}

func (m model) renderHeader() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	titleRow := fillBarRow(renderBarRowJoin(m.renderBrand(), m.renderPomoBadge(), w), w, headerBarStyle)
	navRow := fillBarRow(renderBarRow(m.renderNav(), w), w, headerBarStyle)
	return titleRow + "\n" + navRow
}

func (m model) renderBody(l layout) string {
	switch m.view {
	case viewCalendar:
		return m.renderCalendarView(l)
	case viewPomodoro:
		return m.renderPomodoroView(l)
	case viewHabits:
		return m.renderHabitsView(l)
	default:
		return m.renderTasksView(l)
	}
}

func (m model) renderHabitsView(l layout) string {
	var b strings.Builder
	b.WriteString(headerStyle.Render(iconHabit + " Habits"))
	b.WriteString("\n\n")
	if len(m.habits) == 0 {
		b.WriteString(hintStyle.Render("(no habits — refresh with r)"))
		return renderPane(b.String(), l, l.termW, false)
	}

	maxRows := l.maxScrollRows()
	win := computeScrollWindow(m.habitCursor, len(m.habits), maxRows)
	if hint := m.scrollHint(win); hint != "" {
		b.WriteString(hint)
		b.WriteString("\n")
	}

	for i := win.Start; i < win.End; i++ {
		h := m.habits[i]
		done := m.habitCheckedToday[h.ID]
		marker := iconTaskOpen
		if done {
			marker = iconCheck
		}
		status := hintStyle.Render("today: open")
		if done {
			status = taskDoneStyle.Render("today: done")
		}
		line := fmt.Sprintf("%s  %s  %s · %d total", marker, h.Name, status, h.TotalCheckIns.Int())
		if i == m.habitCursor {
			line = listSelStyle.Render(iconTaskSel + " " + line)
		} else {
			line = listIdleStyle.Render("  " + line)
		}
		b.WriteString(truncateInner(line, l.fullW))
		b.WriteString("\n")
	}

	if m.mode == modeRenameHabit {
		b.WriteString("\n")
		b.WriteString(m.renameInput.View())
		b.WriteString("\n")
		if h := m.keyHint("enter save · esc cancel"); h != "" {
			b.WriteString(h)
		}
	}

	if h := m.keyHint("j/k · space check-in · e rename · x delete · r refresh"); h != "" {
		b.WriteString("\n")
		b.WriteString(h)
	}
	return renderPane(b.String(), l, l.termW, false)
}

func (m model) renderFooter() string {
	w := m.width
	if w < 1 {
		w = 80
	}
	if m.errMsg != "" {
		text := " " + errStyle.Render("! "+m.errMsg)
		if hint := m.footerKeyHint(); hint != "" {
			text += "  " + hintStyle.Render(hint)
		}
		return fillBarRow(statusBarStyle.Render(text), w, statusBarStyle)
	}
	left := m.footerStatus()
	if m.cacheStale {
		left += " · cached"
		if age := cacheAge(m.cacheSavedAt); age != "" {
			left += " " + age
		}
	}
	text := " " + left
	if hint := m.footerKeyHint(); hint != "" {
		text += "  " + statusKeyStyle.Render(hint)
	}
	return fillBarRow(statusBarStyle.Render(text), w, statusBarStyle)
}

func cacheAge(savedAt time.Time) string {
	if savedAt.IsZero() {
		return ""
	}
	age := time.Since(savedAt)
	switch {
	case age < time.Minute:
		return "just now"
	case age < time.Hour:
		return fmt.Sprintf("%dm ago", int(age.Minutes()))
	case age < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(age.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(age.Hours()/24))
	}
}

func (m model) footerStatus() string {
	if m.toast != "" {
		return m.toast
	}
	if m.loading {
		switch m.view {
		case viewCalendar:
			return iconRefresh + " loading calendar…"
		case viewPomodoro:
			return iconRefresh + " loading pomodoro…"
		case viewHabits:
			return iconRefresh + " loading habits…"
		default:
			if m.projectID != "" {
				return iconRefresh + " loading tasks…"
			}
			return iconRefresh + " loading…"
		}
	}
	switch m.view {
	case viewCalendar:
		n := 0
		for _, entries := range m.calIdx().byDate {
			n += len(entries)
		}
		return fmt.Sprintf("calendar · %d items", n)
	case viewPomodoro:
		if line := activeFocusFooterLine(loadActiveFocusSession()); line != "" {
			return line
		}
		if m.focusStats != nil {
			day := m.pomoViewDate
			if day.IsZero() {
				day = dateOnly(time.Now())
			}
			stats := fmt.Sprintf("%d full · %d logged", m.focusStats.FullPomoCount, m.focusStats.PomoCount)
			if dateKey(day) != dateKey(time.Now()) {
				return fmt.Sprintf("pomodoro · %s · %s", day.Format("Mon 2 Jan"), stats)
			}
			return fmt.Sprintf("pomodoro · %s", stats)
		}
		return "pomodoro"
	case viewHabits:
		return fmt.Sprintf("%d habits", len(m.habits))
	default:
		if line := activeFocusFooterLine(loadActiveFocusSession()); line != "" {
			return line
		}
		if m.projectName != "" {
			total, matching, shown := m.taskScopeStats()
			parts := []string{
				m.projectName,
				strings.ToLower(m.effectiveTaskScope().Label()),
				fmt.Sprintf("%d total", total),
			}
			if strings.TrimSpace(m.filterInput.Value()) != "" {
				parts = append(parts, fmt.Sprintf("%d matching", matching))
			}
			parts = append(parts, fmt.Sprintf("%d shown", shown))
			return strings.Join(parts, " · ")
		}
		return fmt.Sprintf("%d lists", len(m.selectableLists()))
	}
}

func footerHint(v appView) string {
	switch v {
	case viewCalendar:
		return "? help · d/w/m/y subview · j/k · [/] navigate · Enter drill · x check-in · o overdue · t today · 1-4 · q quit"
	case viewPomodoro:
		return "? help · [/] day · t today · v design · z density · j/k timeline · h/l legend · x delete · e rename · n add · s/f timer · p · S stop · 1-4 · r refresh · q quit"
	case viewHabits:
		return "? help · j/k · space check-in · e rename · x delete · 1-4 · r refresh · q quit"
	default:
		return "? help · h/l panes · c/C scope · / search · d done/restore/recreate · x check-in · space mark · Backspace trash/delete · 1-4 · r refresh · q quit"
	}
}

func formatClock(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}
