package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

func (m model) pomoFocusDesign() PomoFocusDesign {
	return normalizePomoFocusDesign(m.uiSettings.PomoFocusDesign)
}

func liveFocusSessionFor(day time.Time) *focus.Session {
	if dateKey(day) != dateKey(time.Now()) {
		return nil
	}
	session, err := focus.Load()
	if err != nil || !session.Active() {
		return nil
	}
	return session
}

func centerPomoVisualLine(line string, width int) string {
	line = truncateInner(line, width)
	padding := max(0, (width-lipgloss.Width(line))/2)
	return truncateInner(strings.Repeat(" ", padding)+line, width)
}

func focusBarWidth(width int) int {
	barWidth := width - 8
	if barWidth > 32 {
		barWidth = 32
	}
	if barWidth < 8 {
		barWidth = 8
	}
	return barWidth
}

func renderFocusBarLine(slices []focusSlice, session *focus.Session, width int) string {
	barWidth := focusBarWidth(width)
	var bar strings.Builder
	for i := 0; i < barWidth; i++ {
		angle := (float64(i) + 0.5) / float64(barWidth) * 2 * math.Pi
		color := focusArcColor(angle, slices, session)
		glyph := "━"
		if color == colorOverlay || color == colorMuted {
			glyph = "─"
		}
		bar.WriteString(lipgloss.NewStyle().Foreground(color).Render(glyph))
	}
	return "╺" + bar.String() + "╸"
}

func renderFocusBar(
	slices []focusSlice,
	pauses []focus.PauseSpell,
	now time.Time,
	width int,
	session *focus.Session,
) []string {
	primary, secondary := focusArcLabels(slices, pauses, now, session)
	return []string{
		centerPomoVisualLine(primary, width),
		centerPomoVisualLine(renderFocusBarLine(slices, session, width), width),
		centerPomoVisualLine(secondary, width),
	}
}

func focusCardBorderColor(session *focus.Session) lipgloss.Color {
	switch {
	case session == nil || !session.Active():
		return colorMauve
	case session.State == focus.StateAwaitingDismiss && !session.InOvertimeGrace():
		return colorRed
	case session.State == focus.StatePaused:
		return colorBlue
	default:
		return colorPeach
	}
}

func renderPomoGoalBar(completed, goal, width int) string {
	goal = max(goal, 1)
	barWidth := min(max(width-4, 8), 20)
	filled := min(completed*barWidth/goal, barWidth)
	if completed > 0 && filled == 0 {
		filled = 1
	}
	return lipgloss.NewStyle().Foreground(colorGreen).Render(strings.Repeat("━", filled)) +
		dayEmptySlotStyle.Render(strings.Repeat("─", barWidth-filled))
}

func renderFocusCard(
	slices []focusSlice,
	pauses []focus.PauseSpell,
	now time.Time,
	width int,
	session *focus.Session,
	completed, goal int,
) []string {
	cardWidth := width - 4
	if cardWidth > 34 {
		cardWidth = 34
	}
	if cardWidth < 16 {
		cardWidth = 16
	}
	innerWidth := cardWidth - 2
	primary, secondary := focusArcLabels(slices, pauses, now, session)
	if session == nil || !session.Active() {
		remaining := max(goal-completed, 0)
		if remaining == 0 {
			secondary = hintStyle.Render("daily goal complete")
		} else {
			secondary = hintStyle.Render(fmt.Sprintf("%d pomodoros remaining", remaining))
		}
	}
	bar := renderFocusBarLine(slices, session, innerWidth)
	goalLabel := pomoTodayStyle(completed, goal).Render(fmt.Sprintf("%d / %d pomodoros", completed, goal))
	goalBar := renderPomoGoalBar(completed, goal, innerWidth)
	borderStyle := lipgloss.NewStyle().Foreground(focusCardBorderColor(session))
	border := func(value string) string { return borderStyle.Render(value) }
	row := func(value string) string {
		value = truncateInner(value, innerWidth-2)
		left := max(0, (innerWidth-lipgloss.Width(value))/2)
		right := max(0, innerWidth-left-lipgloss.Width(value))
		return border("│") + strings.Repeat(" ", left) + value + strings.Repeat(" ", right) + border("│")
	}
	lines := []string{
		border("╭" + strings.Repeat("─", innerWidth) + "╮"),
		row(primary),
		row(bar),
		row(secondary),
		row(goalLabel),
		row(goalBar),
		border("╰" + strings.Repeat("─", innerWidth) + "╯"),
	}
	for i := range lines {
		lines[i] = centerPomoVisualLine(lines[i], width)
	}
	return lines
}

func (m model) displayedPomoCount() int {
	if m.focusStats != nil {
		return m.focusStats.FullPomoCount
	}
	if m.pomoViewIsToday() {
		return m.todayPomos
	}
	return 0
}

func (m model) renderPomoFocusVisual(
	slices []focusSlice,
	pauses []focus.PauseSpell,
	now time.Time,
	width int,
) []string {
	session := liveFocusSessionFor(now)
	switch m.pomoFocusDesign() {
	case PomoFocusBar:
		return renderFocusBar(slices, pauses, now, width, session)
	case PomoFocusCard:
		return renderFocusCard(
			slices, pauses, now, width, session,
			m.displayedPomoCount(), m.uiSettings.pomoDailyGoal(),
		)
	default:
		return renderSegmentedFocusArc(slices, pauses, now, width, session)
	}
}

func pomoFocusDesignToast(design PomoFocusDesign) string {
	return fmt.Sprintf("pomodoro design · %s", design.Label())
}
