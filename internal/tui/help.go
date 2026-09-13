package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type helpRow struct {
	key     string
	desc    string
	section bool
}

func helpSection(title string) helpRow {
	return helpRow{desc: title, section: true}
}

var helpRows = []helpRow{
	helpSection("Global"),
	{"1 / 2 / 3 / 4", "switch Tasks · Calendar · Pomodoro · Habits views", false},
	{"Tab", "next view", false},
	{"Shift+Tab", "previous view · focus lists pane (tasks view)", false},
	{"?", "toggle this help menu", false},
	{".", "toggle key hints in footer and panes", false},
	{"r / R", "refresh current view", false},
	{"", "last view (Tasks · Calendar · Pomo · Habits) is restored on launch", false},
	{"q / Ctrl+C", "quit", false},

	helpSection("Help menu"),
	{"type", "filter keys by description text", false},
	{"j / k", "move selection in filtered list", false},
	{"g / G", "jump to first / last match", false},
	{"Backspace", "delete last search character", false},
	{"Ctrl+U", "clear search", false},
	{"Esc / ?", "close help", false},

	helpSection("Tasks view — lists pane"),
	{"h / Shift+Tab", "focus lists pane", false},
	{"j / k", "move list selection", false},
	{"PgUp / PgDn", "scroll lists one page", false},
	{"Ctrl+U / Ctrl+D", "scroll lists one page (alternative)", false},
	{"g / G", "first / last list", false},
	{"n", "new list — pick folder, then name", false},
	{"N", "new folder", false},
	{"m", "move selected list to folder", false},
	{"e", "rename list or folder", false},
	{"x / Backspace", "delete list or folder", false},

	helpSection("Tasks view — tasks pane"),
	{"l / Tab", "focus tasks pane", false},
	{"j / k", "move task selection", false},
	{"PgUp / PgDn", "scroll tasks one page", false},
	{"Ctrl+U / Ctrl+D", "scroll tasks one page (alternative)", false},
	{"g / G", "first / last task", false},
	{"Enter / d", "complete open task(s) · reopen done task(s)", false},
	{"c", "show / hide completed tasks in list", false},
	{"C", "show / hide trashed tasks in list", false},
	{"n", "new task form (title · notes · due · time · duration · reminder · priority)", false},
	{"Space", "mark / unmark task for bulk actions", false},
	{"a", "mark all visible open and done tasks", false},
	{"A", "mark all visible completed tasks only", false},
	{"u", "clear marked tasks", false},
	{"m", "move marked tasks or cursor task to another list (incl. done when shown)", false},
	{"e", "edit task (title · notes · due · time · duration · reminder · priority)", false},
	{"x / Backspace", "delete marked tasks or cursor task", false},
	{"/", "filter tasks by title", false},
	{"o", "cycle task sort (custom · due · priority · title)", false},
	{"z", "toggle task detail layout (bottom panel · side panel)", false},

	helpSection("New task form"),
	{"Tab / Shift+Tab", "next / previous field", false},
	{"[ / ]", "cycle reminder or priority options", false},
	{"Enter", "next field · save on last field · newline in notes", false},
	{"Esc", "cancel", false},
	{"Due date", "DD/MM/YYYY with auto slashes · empty = no date", false},

	helpSection("Edit task form"),
	{"e", "open edit form on selected task", false},
	{"Tab / Shift+Tab", "next / previous field", false},
	{"[ / ]", "cycle reminder or priority options", false},
	{"Enter", "next field · save on last field · newline in notes", false},
	{"Esc", "cancel", false},
	{"Clear due", "empty due date removes schedule", false},

	helpSection("New list prompt"),
	{"Ctrl+F", "change target folder", false},
	{"Enter", "create list", false},
	{"Esc", "cancel", false},

	helpSection("Calendar view"),
	{"d / w / m / y", "day · week · month · year sub-view", false},
	{"t", "jump to today", false},
	{"Enter", "open selected day (month/week → day view · year → month)", false},
	{"[ / ] / ← / →", "previous / next period", false},
	{"h / l / j / k", "move grid selection (week: j/k scroll timeline)", false},

	helpSection("Pomodoro view"),
	{"z", "toggle timeline density (compact · stretch)", false},
	{"[ / ]", "previous / next timeline day", false},
	{"t", "jump timeline to today", false},
	{"j / k", "move timeline / session selection", false},
	{"h / l", "move task legend selection", false},
	{"n", "log focus session (task · start · duration · pause)", false},
	{"e", "rename pomodoro entry", false},
	{"x / Backspace", "delete pomodoro entry", false},
	{"s", "open focus picker (25 min preset)", false},
	{"f", "open focus picker (5 min preset)", false},
	{"t", "type custom focus duration (in picker)", false},
	{"1 – 5", "focus duration presets (in picker)", false},
	{"p", "pause / resume active focus timer", false},
	{"R", "log session and restart same task + duration (works on completion alert too)", false},
	{"S", "stop focus and log session to TickTick", false},
	{"T", "switch task mid-session (timer keeps running)", false},
	{"D", "dismiss focus alert / notification", false},

	helpSection("Focus complete alert"),
	{"Enter / D", "dismiss alert (log planned time · skip unclaimed if in grace)", false},
	{"R", "log session and start another pomodoro on the same task + duration", false},
	{"S", "stop and log (includes unclaimed overtime if any)", false},
	{"P", "pause timer from alert", false},

	helpSection("Focus picker"),
	{"type", "filter tasks by title or list name", false},
	{"j / k", "select task", false},
	{"Enter", "start focus on selected task", false},
	{"Backspace", "delete last filter character", false},
	{"Ctrl+U", "clear filter", false},
	{"Esc", "clear filter · cancel picker", false},

	helpSection("Focus alert overlay"),
	{"n t", "dismiss alert (chord)", false},
	{"n r / r", "log session and restart same task", false},
	{"D", "dismiss notification", false},

	helpSection("Habits view"),
	{"j / k", "move habit selection", false},
	{"Space / Enter", "check in or undo today's habit", false},
	{"e", "rename habit", false},
	{"x / Backspace", "delete habit", false},

	helpSection("Move / folder pickers"),
	{"j / k", "select destination", false},
	{"Enter", "confirm", false},
	{"Esc", "cancel", false},
}

const helpMarkerW = 3 // "▸  " or "   "

func helpKeyColWidth() int {
	max := 0
	for _, row := range helpRows {
		if row.section {
			continue
		}
		if w := lipgloss.Width(row.key); w > max {
			max = w
		}
	}
	return helpMarkerW + max + 2 // gap before description
}

func (m model) renderHelpOverlay() string {
	boxW := m.width - 4
	if boxW > 72 {
		boxW = 72
	}
	if boxW < 40 {
		boxW = m.width - 2
	}

	innerW := boxW - 4 // border + padding
	if innerW < 20 {
		innerW = 20
	}
	keyColW := helpKeyColWidth()

	var lines []string
	lines = append(lines, helpInnerLine(helpTitleInBoxStyle.Render(iconHelp+"  Keybindings"), innerW))
	lines = append(lines, helpInnerLine(helpHintInBoxStyle.Render("type to search · j/k scroll · esc or ? close"), innerW))
	if q := strings.TrimSpace(m.helpFilter); q != "" {
		lines = append(lines, helpInnerLine(helpSearchStyle.Render("search: "+q), innerW))
	}
	lines = append(lines, helpInnerLine("", innerW))

	indices := m.helpFilteredIndices()
	if len(indices) == 0 {
		lines = append(lines, helpInnerLine(helpHintInBoxStyle.Render("(no matches)"), innerW))
		box := helpBoxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
		return centerBoxOnScreen(box, m.width, m.height)
	}

	maxRows := m.height - 10
	if m.helpFilter != "" {
		maxRows = m.height - 11
	}
	if maxRows < 6 {
		maxRows = 6
	}
	if maxRows > len(indices) {
		maxRows = len(indices)
	}

	cur := m.clampHelpCursor(m.helpCursor)
	nav := helpNavigableIndices(indices)
	selectedIdx := -1
	if cur >= 0 && cur < len(nav) {
		selectedIdx = nav[cur]
	}
	selDisplay := 0
	for i, idx := range indices {
		if idx == selectedIdx {
			selDisplay = i
			break
		}
	}
	start := 0
	if selDisplay >= maxRows {
		start = selDisplay - maxRows + 1
	}
	end := start + maxRows
	if end > len(indices) {
		end = len(indices)
	}

	for i := start; i < end; i++ {
		rowIdx := indices[i]
		lines = append(lines, renderHelpLine(helpRows[rowIdx], rowIdx == selectedIdx, innerW, keyColW, m.helpFilter))
	}

	box := helpBoxStyle.Width(boxW).Render(strings.Join(lines, "\n"))
	return centerBoxOnScreen(box, m.width, m.height)
}

// helpInnerLine pads a box content row to full inner width (terminal default background).
func helpInnerLine(content string, innerW int) string {
	return fillBarRow(content, innerW, rowPadStyle)
}

// centerBoxOnScreen centers the modal box on plain spaces (terminal default background).
func centerBoxOnScreen(box string, termW, termH int) string {
	return centerBoxOnPlainScreen(box, termW, termH)
}

// centerBoxOnPlainScreen fills the screen with unpainted spaces and centers the box.
func centerBoxOnPlainScreen(box string, termW, termH int) string {
	return centerBoxOnBackground(box, termW, termH, rowPadStyle)
}

// centerBoxOnBackground fills the screen with a solid backdrop and centers the box.
func centerBoxOnBackground(box string, termW, termH int, bg lipgloss.Style) string {
	if termW < 1 {
		termW = 1
	}
	if termH < 1 {
		termH = 1
	}
	boxLines := strings.Split(strings.TrimSuffix(box, "\n"), "\n")
	if len(boxLines) == 0 {
		boxLines = []string{""}
	}
	top := max(0, (termH-len(boxLines))/2)

	bgRow := bg.Render(strings.Repeat(" ", termW))
	rows := make([]string, termH)
	for y := 0; y < termH; y++ {
		rows[y] = bgRow
	}
	for i, line := range boxLines {
		y := top + i
		if y >= termH {
			break
		}
		rows[y] = centerRowOnBackground(termW, line, bg)
	}
	return strings.Join(rows, "\n")
}

// centerBoxOnDimScreen fills the terminal with a dim layer and centers the box on top.
// When heavy is true the backdrop is darker (used for escalated focus alerts).
func centerBoxOnDimScreen(box string, termW, termH int, heavy bool) string {
	if termW < 1 {
		termW = 1
	}
	if termH < 1 {
		termH = 1
	}
	boxLines := strings.Split(strings.TrimSuffix(box, "\n"), "\n")
	if len(boxLines) == 0 {
		boxLines = []string{""}
	}
	top := max(0, (termH-len(boxLines))/2)

	dimStyle := lipgloss.NewStyle().Foreground(colorMuted).Faint(true)
	dimChar := "▒"
	if heavy {
		dimStyle = lipgloss.NewStyle().Foreground(colorOverlay).Faint(true)
		dimChar = "░"
	}
	dimRow := dimStyle.Render(strings.Repeat(dimChar, termW))

	rows := make([]string, termH)
	for y := 0; y < termH; y++ {
		rows[y] = dimRow
	}
	for i, line := range boxLines {
		y := top + i
		if y >= termH {
			break
		}
		rows[y] = centerRowOnDimBackground(termW, line, dimChar, dimStyle)
	}
	return strings.Join(rows, "\n")
}

func centerRowOnDimBackground(fullW int, content, dimChar string, dimStyle lipgloss.Style) string {
	content = strings.ReplaceAll(content, "\n", "")
	content = strings.ReplaceAll(content, "\r", "")
	cw := lipgloss.Width(content)
	if cw >= fullW {
		return truncateRenderedWidth(content, fullW)
	}
	left := (fullW - cw) / 2
	right := fullW - cw - left
	return dimStyle.Render(strings.Repeat(dimChar, left)) + content + dimStyle.Render(strings.Repeat(dimChar, right))
}

func centerRowOnBackground(fullW int, content string, bg lipgloss.Style) string {
	content = strings.ReplaceAll(content, "\n", "")
	content = strings.ReplaceAll(content, "\r", "")
	cw := lipgloss.Width(content)
	if cw >= fullW {
		return truncateRenderedWidth(content, fullW)
	}
	left := (fullW - cw) / 2
	right := fullW - cw - left
	return bg.Render(strings.Repeat(" ", left)) + content + bg.Render(strings.Repeat(" ", right))
}

// renderHelpLine renders one help row with a fixed-width key column.
func renderHelpLine(row helpRow, selected bool, width, keyColW int, query string) string {
	if row.section {
		title := row.desc
		if lipgloss.Width(title) > width-2 {
			title = truncateRunes(title, max(1, width-3)) + "…"
		}
		st := helpHintInBoxStyle.Copy().Bold(true).Foreground(colorBlue)
		return fillBarRow(st.Render(title), width, rowPadStyle)
	}
	if width < keyColW+4 {
		width = keyColW + 4
	}
	descW := width - keyColW
	if descW < 8 {
		descW = 8
	}
	desc := row.desc
	if lipgloss.Width(desc) > descW {
		desc = truncateRunes(desc, max(1, descW-1)) + "…"
	}
	keyText := row.key
	if lipgloss.Width(keyText) > keyColW-helpMarkerW {
		keyText = truncateRunes(keyText, max(1, keyColW-helpMarkerW-1)) + "…"
	}

	marker := "   "
	if selected {
		marker = "▸  "
	}
	keyPad := padPlainRight(keyText, keyColW-helpMarkerW)

	if selected {
		text := marker + keyPad + desc
		return fillBarRow(helpSelStyle.Render(text), width, helpSelStyle)
	}

	gap := keyColW - helpMarkerW - lipgloss.Width(keyText)
	if gap < 1 {
		gap = 1
	}
	line := helpMarkerStyle.Render(marker) +
		highlightHelpMatch(keyText, query, helpKeyStyle) +
		helpMarkerStyle.Render(strings.Repeat(" ", gap)) +
		highlightHelpMatch(desc, query, helpDescStyle)
	return fillBarRow(line, width, rowPadStyle)
}

func (m model) helpFilteredIndices() []int {
	return helpFilteredIndices(m.helpFilter)
}

func helpNavigableIndices(indices []int) []int {
	var out []int
	for _, i := range indices {
		if !helpRows[i].section {
			out = append(out, i)
		}
	}
	return out
}

func helpFilteredIndices(query string) []int {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		out := make([]int, len(helpRows))
		for i := range helpRows {
			out[i] = i
		}
		return out
	}
	tokens := strings.Fields(q)
	var out []int
	for i, row := range helpRows {
		if helpRowMatches(row, tokens) {
			out = append(out, i)
		}
	}
	return out
}

func helpRowMatches(row helpRow, tokens []string) bool {
	if row.section {
		hay := strings.ToLower(row.desc)
		for _, tok := range tokens {
			if strings.Contains(hay, tok) {
				return true
			}
		}
		return false
	}
	hay := strings.ToLower(row.key + " " + row.desc)
	for _, tok := range tokens {
		if !strings.Contains(hay, tok) {
			return false
		}
	}
	return true
}

func helpFilterKey(msg tea.KeyMsg) (string, bool) {
	if msg.Alt {
		return "", false
	}
	switch msg.Type {
	case tea.KeyRunes:
		return string(msg.Runes), true
	case tea.KeySpace:
		return " ", true
	}
	return "", false
}

func helpHighlightToken(query string) string {
	q := strings.TrimSpace(strings.ToLower(query))
	if q == "" {
		return ""
	}
	return strings.Fields(q)[0]
}

func highlightHelpMatch(text, query string, base lipgloss.Style) string {
	if text == "" {
		return base.Render(text)
	}
	tok := helpHighlightToken(query)
	if tok == "" {
		return base.Render(text)
	}
	lower := strings.ToLower(text)
	idx := strings.Index(lower, tok)
	if idx < 0 {
		return base.Render(text)
	}
	before := text[:idx]
	match := text[idx : idx+len(tok)]
	after := text[idx+len(tok):]
	return base.Render(before) +
		helpMatchStyle.Render(match) +
		base.Render(after)
}

func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

func padPlainRight(s string, w int) string {
	if w < 1 {
		return s
	}
	cur := lipgloss.Width(s)
	if cur >= w {
		return s
	}
	return s + strings.Repeat(" ", w-cur)
}
