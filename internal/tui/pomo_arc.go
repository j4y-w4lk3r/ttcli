package tui

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

const focusArcSegments = 12

type focusArcTemplate struct {
	rows         []string
	primaryRow   int
	secondaryRow int
}

var (
	wideFocusArc = focusArcTemplate{
		rows: []string{
			"      ╭────────╮      ",
			"  ╭───╯        ╰───╮  ",
			" │                  │ ",
			" │                  │ ",
			"  ╰───╮        ╭───╯  ",
			"      ╰────────╯      ",
		},
		primaryRow:   2,
		secondaryRow: 3,
	}
	compactFocusArc = focusArcTemplate{
		rows: []string{
			"   ╭──────╮   ",
			" ╭─╯      ╰─╮ ",
			"│            │",
			" ╰─╮      ╭─╯ ",
			"   ╰──────╯   ",
		},
		primaryRow:   2,
		secondaryRow: -1,
	}
)

func focusArcForWidth(width int) focusArcTemplate {
	if width >= lipgloss.Width(wideFocusArc.rows[0])+2 {
		return wideFocusArc
	}
	return compactFocusArc
}

func focusArcAngle(row, col, height, width int) float64 {
	cx := float64(width-1) / 2
	cy := float64(height-1) / 2
	dx := float64(col) - cx
	// Terminal cells are taller than they are wide. Stretch the row delta so
	// segment assignment follows the perceived ellipse rather than the grid.
	dy := (float64(row) - cy) * 2
	angle := math.Atan2(dy, dx) + math.Pi/2
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return angle
}

func colorAtAngle(angle float64, slices []focusSlice, total int) lipgloss.Color {
	if total <= 0 || len(slices) == 0 {
		return colorOverlay
	}
	start := 0.0
	for _, slice := range slices {
		end := start + float64(slice.Secs)/float64(total)*2*math.Pi
		if angle >= start && angle < end {
			return slice.Color
		}
		start = end
	}
	return slices[len(slices)-1].Color
}

func focusArcColor(
	angle float64,
	slices []focusSlice,
	sess *focus.Session,
) lipgloss.Color {
	segment := math.Floor(angle / (2 * math.Pi) * focusArcSegments)
	angle = (segment + 0.5) / focusArcSegments * 2 * math.Pi
	if sess == nil || !sess.Active() {
		if len(slices) == 0 {
			return colorMuted
		}
		return colorAtAngle(angle, slices, totalFocusSecs(slices))
	}

	duration := sess.Duration
	if duration < time.Second {
		duration = time.Second
	}
	progress := angle / (2 * math.Pi)
	if sess.State == focus.StateAwaitingDismiss && !sess.InOvertimeGrace() {
		overtime := sess.OvertimeElapsed()
		overtimeFrac := float64(overtime) / float64(duration)
		if progress <= math.Min(1, overtimeFrac) {
			return colorRed
		}
		return colorPeach
	}

	elapsedFrac := float64(sess.Elapsed()) / float64(duration)
	elapsedFrac = math.Max(0, math.Min(1, elapsedFrac))
	if progress <= elapsedFrac {
		if sess.State == focus.StatePaused {
			return colorBlue
		}
		return colorPeach
	}
	return colorOverlay
}

func focusArcLabels(
	slices []focusSlice,
	pauses []focus.PauseSpell,
	now time.Time,
	sess *focus.Session,
) (primary, secondary string) {
	total := formatFocusTotal(totalFocusSecs(slices))
	if sess == nil || !sess.Active() {
		primary = timerBigStyle.Render(total)
		secondaryText := total + " focused"
		if paused := totalPauseSecs(pauses, now); paused > 0 {
			secondaryText += " · " + formatFocusTotal(paused) + " paused"
		}
		return primary, hintStyle.Render(secondaryText)
	}

	switch {
	case sess.State == focus.StateAwaitingDismiss && sess.InOvertimeGrace():
		primary = timerBigStyle.Render(formatClock(sess.OvertimeGraceRemaining()))
		secondary = hintStyle.Render("dismiss · no extra time")
	case sess.State == focus.StateAwaitingDismiss:
		primary = lipgloss.NewStyle().Bold(true).Foreground(colorRed).Render(formatClock(sess.OvertimeElapsed()))
		secondary = unclaimedLabelStyle.Render("EXTRA TIME")
	default:
		primary = timerBigStyle.Render(formatClock(sess.Remaining()))
		state := sess.State
		if state == "" {
			state = focus.StateRunning
		}
		secondary = hintStyle.Render(state + " · " + total + " today")
	}
	return primary, secondary
}

func focusArcOutlineRow(
	template focusArcTemplate,
	row int,
	slices []focusSlice,
	sess *focus.Session,
) ([]string, int, int) {
	runes := []rune(template.rows[row])
	cells := make([]string, len(runes))
	first, last := -1, -1
	for col, r := range runes {
		if r == ' ' {
			cells[col] = " "
			continue
		}
		if first < 0 {
			first = col
		}
		last = col
		angle := focusArcAngle(row, col, len(template.rows), len(runes))
		color := focusArcColor(angle, slices, sess)
		cells[col] = lipgloss.NewStyle().Foreground(color).Render(string(r))
	}
	return cells, first, last
}

func focusArcLabelRow(cells []string, first, last int, label string) string {
	if first < 0 || last <= first || label == "" {
		return strings.Join(cells, "")
	}
	innerW := last - first - 1
	if innerW < 1 {
		return strings.Join(cells, "")
	}
	label = truncateInner(label, innerW)
	leftPad := max(0, (innerW-lipgloss.Width(label))/2)
	rightPad := max(0, innerW-leftPad-lipgloss.Width(label))
	return strings.Join(cells[:first+1], "") +
		strings.Repeat(" ", leftPad) + label + strings.Repeat(" ", rightPad) +
		strings.Join(cells[last:], "")
}

func renderSegmentedFocusArc(
	slices []focusSlice,
	pauses []focus.PauseSpell,
	now time.Time,
	width int,
	sess *focus.Session,
) []string {
	template := focusArcForWidth(width)
	primary, secondary := focusArcLabels(slices, pauses, now, sess)
	arcWidth := lipgloss.Width(template.rows[0])
	offset := max(0, (width-arcWidth)/2)
	pad := strings.Repeat(" ", offset)
	lines := make([]string, 0, len(template.rows)+1)
	for row := range template.rows {
		cells, first, last := focusArcOutlineRow(template, row, slices, sess)
		line := strings.Join(cells, "")
		switch row {
		case template.primaryRow:
			line = focusArcLabelRow(cells, first, last, primary)
		case template.secondaryRow:
			line = focusArcLabelRow(cells, first, last, secondary)
		}
		lines = append(lines, truncateInner(pad+line, width))
	}
	if template.secondaryRow < 0 && secondary != "" {
		secondary = truncateInner(secondary, arcWidth)
		centered := strings.Repeat(" ", max(0, (arcWidth-lipgloss.Width(secondary))/2)) + secondary
		lines = append(lines, truncateInner(pad+centered, width))
	}
	return lines
}
