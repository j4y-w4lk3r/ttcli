package tui

import (
	"strings"
)

// keyHint renders grey keybinding advice when showKeyHints is enabled.
func (m model) keyHint(s string) string {
	if !m.showKeyHints || strings.TrimSpace(s) == "" {
		return ""
	}
	return hintStyle.Render(s)
}

func (m model) keyHintInner(s string, width int) string {
	if h := m.keyHint(s); h != "" {
		return truncateInner(h, width)
	}
	return ""
}

func (m model) scrollHint(win scrollWindow) string {
	if !m.showKeyHints {
		return ""
	}
	return renderScrollHint(win)
}

func (m model) footerKeyHint() string {
	if !m.showKeyHints {
		return ""
	}
	if m.showHelp {
		return "help open — j/k scroll · esc/? close"
	}
	if m.mode == modeFocusPicker {
		if m.focusPickerSwitch {
			return "switch task — j/k · enter switch · esc cancel · timer keeps running"
		}
		return "focus picker — j/k task · t type minutes · [ ] ±5m · enter start · esc cancel"
	}
	if m.mode == modeAddPomoForm {
		return "log focus — tab fields · j/k task · 1–5 duration · enter save · esc cancel"
	}
	return footerHint(m.view)
}
