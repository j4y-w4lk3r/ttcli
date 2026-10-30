package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func pausesInsideRecord(rec ticktick.FocusRecord, pauses []focus.PauseSpell) []focus.PauseSpell {
	start, ok1 := focusTimeLocal(rec.StartTime)
	end, ok2 := focusTimeLocal(rec.EndTime)
	if !ok1 || !ok2 || !end.After(start) {
		return nil
	}
	title := strings.TrimSpace(rec.TaskTitle())
	if title == "" {
		return nil
	}
	var out []focus.PauseSpell
	for _, spell := range pauses {
		if spell.Start.IsZero() {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(spell.TaskTitle), title) {
			continue
		}
		t := spell.Start
		if t.Before(start) || !t.Before(end) {
			continue
		}
		out = append(out, spell)
	}
	return out
}

func appendSessionPauseTrack(line string, contentW int, taskColor lipgloss.Color, start, end time.Time, pauses []focus.PauseSpell, selected bool) string {
	if len(pauses) == 0 || !end.After(start) {
		return line
	}
	label := ""
	if len(pauses) > 1 {
		label = fmt.Sprintf("  %d pauses", len(pauses))
	}
	room := contentW - lipgloss.Width(line) - 2 - len(label)
	if room < 8 {
		return line
	}
	trackW := room
	if trackW > 28 {
		trackW = 28
	}
	track := renderSessionPauseTrack(trackW, taskColor, start, end, pauses, selected)
	out := line + "  " + track
	if label != "" {
		out += pauseLabelStyle.Render(label)
	}
	return out
}

func renderSessionPauseTrack(trackW int, taskColor lipgloss.Color, start, end time.Time, pauses []focus.PauseSpell, selected bool) string {
	if trackW < 1 || !end.After(start) {
		return ""
	}
	bubbles := make([]bool, trackW)
	dur := end.Sub(start).Seconds()
	for _, spell := range pauses {
		if spell.Start.IsZero() {
			continue
		}
		pos := spell.Start.Sub(start).Seconds() / dur
		if pos < 0 {
			pos = 0
		}
		if pos > 0.999 {
			pos = 0.999
		}
		i := int(pos * float64(trackW))
		if i >= trackW {
			i = trackW - 1
		}
		if i < 0 {
			i = 0
		}
		bubbles[i] = true
	}
	barSt := lipgloss.NewStyle().Foreground(taskColor)
	if taskColor == "" {
		barSt = lipgloss.NewStyle().Foreground(colorMuted)
	}
	bubSt := pauseLabelStyle
	if selected {
		barSt = barSt.Bold(true)
		bubSt = pauseLabelStyle.Copy().Bold(true)
	}
	var b strings.Builder
	for _, bubble := range bubbles {
		if bubble {
			b.WriteString(bubSt.Render("◉"))
		} else {
			b.WriteString(barSt.Render("━"))
		}
	}
	return b.String()
}

func closeSessionAroundPauses(grid []dayGridRow) []dayGridRow {
	var out []dayGridRow
	for i := 0; i < len(grid); i++ {
		row := grid[i]
		if row.kind != "pomo" || len(row.sessionPauses) == 0 {
			out = append(out, row)
			continue
		}
		row.sessionOpen = true
		out = append(out, row)
		want := map[string]int{}
		for _, spell := range row.sessionPauses {
			want[pauseInsideKey(spell)]++
		}
		seen := 0
		i++
		for i < len(grid) && seen < len(row.sessionPauses) {
			next := grid[i]
			key := pauseInsideKey(next.pause)
			if next.kind == "pause" && want[key] > 0 {
				want[key]--
				out = append(out, next)
				seen++
				i++
				continue
			}
			break
		}
		if seen == 0 {
			out[len(out)-1].sessionOpen = false
			i--
			continue
		}
		end := row
		end.sessionOpen = false
		end.sessionClose = true
		out = append(out, end)
		i--
	}
	return out
}

func pauseInsideKey(spell focus.PauseSpell) string {
	return strings.TrimSpace(spell.TaskTitle) + "|" + spell.Start.UTC().Format(time.RFC3339Nano)
}
