package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha–inspired palette (matches hs-tui family).
// Base UI fill uses the terminal default background (no Background() on pad styles)
// so Kitty/theme colors show through unchanged.
var (
	colorBase     = lipgloss.Color("#1e1e2e") // foreground on accent badges
	colorSurface  = lipgloss.Color("#313244")
	colorOverlay  = lipgloss.Color("#45475a")
	colorText     = lipgloss.Color("#cdd6f4")
	colorSubtext  = lipgloss.Color("#a6adc8")
	colorMuted    = lipgloss.Color("#6c7086")
	colorPink     = lipgloss.Color("#f5c2e7")
	colorMauve    = lipgloss.Color("#cba6f7")
	colorBlue     = lipgloss.Color("#89b4fa")
	colorGreen    = lipgloss.Color("#a6e3a1")
	colorYellow   = lipgloss.Color("#f9e2af")
	colorPeach    = lipgloss.Color("#fab387")
	colorRed      = lipgloss.Color("#f38ba8")
	colorTeal     = lipgloss.Color("#94e2d5")
)

const dailyPomoGoal = 30

var (
	// rowPadStyle pads rows to width without painting a background color.
	rowPadStyle = lipgloss.NewStyle()
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBase).
			Background(colorMauve).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	folderStyle = lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true)

	listSelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve)

	listIdleStyle = lipgloss.NewStyle().
			Foreground(colorText)

	taskSelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorGreen)

	taskIdleStyle = lipgloss.NewStyle().
			Foreground(colorText)

	taskDoneStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Strikethrough(true)

	taskTrashedStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Strikethrough(true).
			Italic(true)

	prioHighStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	prioMedStyle  = lipgloss.NewStyle().Foreground(colorYellow)
	prioLowStyle  = lipgloss.NewStyle().Foreground(colorBlue)

	dueStyle = lipgloss.NewStyle().Foreground(colorTeal)
	dueOverStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true)

	paneBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorOverlay).
			Padding(0, 1)

	paneFocusBorder = paneBorder.Copy().
			BorderForeground(colorMauve)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	statusKeyStyle = lipgloss.NewStyle().
			Foreground(colorPink).
			Bold(true)

	errStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	inputPromptStyle = lipgloss.NewStyle().
			Foreground(colorPink)

	inputStyle = lipgloss.NewStyle().
			Foreground(colorText)

	navActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBase).
			Background(colorMauve).
			Padding(0, 1)

	navIdleStyle = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 1)

	headerBarStyle = lipgloss.NewStyle()

	headerBrandStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Bold(true)

	headerSubStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	pomoBadgeBaseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	helpBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMauve).
			Padding(1, 2)

	focusPickerSelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve)

	helpScreenStyle = rowPadStyle

	helpMarkerStyle = rowPadStyle

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorPink).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	helpTitleInBoxStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBlue)

	helpHintInBoxStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	helpSearchStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	helpMatchStyle = lipgloss.NewStyle().
			Foreground(colorYellow).
			Bold(true)

	helpSelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorBase).
			Background(colorMauve)

	helpIdleStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	timerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPeach)

	timerBigStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPink).
			Padding(0, 2)

	noteStyle = lipgloss.NewStyle().
			Foreground(colorText)

	noteLabelStyle = lipgloss.NewStyle().
			Foreground(colorPeach).
			Bold(true)

	pomoTimeStyle = lipgloss.NewStyle().
			Foreground(colorTeal).
			Bold(true)

	pomoRailStyle = lipgloss.NewStyle().
			Foreground(colorOverlay)

	pomoNowStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			Bold(true)

	pomoBarStyle = lipgloss.NewStyle().
			Foreground(colorPeach)

	pomoBarSelStyle = lipgloss.NewStyle().
			Foreground(colorMauve).
			Bold(true)

	pomoEndStyle = lipgloss.NewStyle().
			Foreground(colorTeal)

	pomoEndArrowStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	pomoDurationLabelStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorYellow)

	sectionRuleStyle = lipgloss.NewStyle().
			Foreground(colorOverlay)

	dayEmptySlotStyle = lipgloss.NewStyle().
			Foreground(colorMuted)
)

func prioStyle(label string) lipgloss.Style {
	switch label {
	case "high":
		return prioHighStyle
	case "med":
		return prioMedStyle
	case "low":
		return prioLowStyle
	default:
		return lipgloss.NewStyle().Foreground(colorMuted)
	}
}

// pomoTodayStyle colors today's pomodoro count by progress toward the daily goal.
func pomoTodayStyle(count, goal int) lipgloss.Style {
	if goal < 1 {
		goal = dailyPomoGoal
	}
	switch {
	case count >= goal:
		return pomoBadgeBaseStyle.Copy().Bold(true).Foreground(colorGreen)
	case count >= goal*9/10:
		return pomoBadgeBaseStyle.Copy().Bold(true).Foreground(colorTeal)
	case count >= goal*2/3:
		return pomoBadgeBaseStyle.Copy().Foreground(colorYellow)
	case count >= goal/3:
		return pomoBadgeBaseStyle.Copy().Foreground(colorPeach)
	default:
		return pomoBadgeBaseStyle.Copy().Foreground(colorMuted)
	}
}
