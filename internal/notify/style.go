package notify

import (
	"github.com/j4y-w4lk3r/ttcli/internal/focus"
)

// EscalateAfter is how long to wait before escalating focus-end alerts.
// Kept in sync with the unclaimed-time grace window.
const EscalateAfter = focus.OvertimeGracePeriod

// FocusNotifyIcon prefixes desktop completion titles (warm, not alarming).
const FocusNotifyIcon = "✓"

// Variant selects a notification design for testing or production sends.
type Variant string

const (
	VariantStandard  Variant = "standard"
	VariantMinimal   Variant = "minimal"
	VariantThemed    Variant = "themed"
	VariantEscalated Variant = "escalated"
	VariantOrange    Variant = "orange"
)

// Theme colors aligned with ttcli TUI (Catppuccin Mocha).
var Theme = struct {
	Border     string
	Background string
	Foreground string
	Accent     string
	Urgent     string
}{
	Border:     "#fab387",
	Background: "#1e1e2e",
	Foreground: "#cdd6f4",
	Accent:     "#cba6f7",
	Urgent:     "#fab387",
}

// Variants lists notification designs available for notify-test.
func Variants() []VariantInfo {
	return []VariantInfo{
		{
			ID:          VariantStandard,
			Title:       "standard",
			Description: "Production default — peach accent, calm completion copy",
		},
		{
			ID:          VariantMinimal,
			Title:       "minimal",
			Description: "Short title and body, no color hints",
		},
		{
			ID:          VariantThemed,
			Title:       "themed",
			Description: "Full Catppuccin hints (best with dunst frcolor/bgcolor)",
		},
		{
			ID:          VariantOrange,
			Title:       "orange",
			Description: "Peach border emphasis — pair with: ttcli focus notify-setup",
		},
		{
			ID:          VariantEscalated,
			Title:       "escalated",
			Description: "nt escalate — dunst then 120 Hz-smooth growing fullscreen overlay",
		},
	}
}

// VariantInfo describes a test notification preset.
type VariantInfo struct {
	ID          Variant
	Title       string
	Description string
}

// Content holds notify-send title/body/urgency for a variant.
type Content struct {
	Title   string
	Body    string
	Urgency string // low, normal, critical
}

// BuildContent returns notification text for a variant.
func BuildContent(v Variant, taskTitle string) Content {
	escalated := v == VariantEscalated
	switch v {
	case VariantMinimal:
		c := buildFocusContent(taskTitle, false)
		return Content{Title: FocusNotifyIcon + " Done", Body: c.Body, Urgency: "normal"}
	case VariantEscalated:
		return buildFocusContent(taskTitle, true)
	default:
		return buildFocusContent(taskTitle, escalated)
	}
}

func buildFocusContent(taskTitle string, escalated bool) Content {
	title := FocusNotifyIcon + " Task complete"
	body := "Nice work — time for a break."
	if taskTitle != "" {
		body = "Done · " + taskTitle
	}
	if escalated {
		body = "Still waiting — " + body
	}
	return Content{Title: title, Body: body, Urgency: "normal"}
}

// AppendHints adds daemon-specific styling hints to notify-send args.
func AppendHints(args []string, v Variant) []string {
	args = append(args,
		"-h", "string:x-dunst-stack-tag:ttcli-focus",
		"-h", "string:synchronous:ttcli-focus",
	)
	switch v {
	case VariantMinimal:
		return args
	case VariantOrange, VariantStandard, VariantThemed, VariantEscalated:
		return append(args,
			"-h", "string:frcolor:"+Theme.Border,
			"-h", "string:border-color:"+Theme.Border,
			"-h", "string:bgcolor:"+Theme.Background,
			"-h", "string:fgcolor:"+Theme.Foreground,
		)
	default:
		return append(args,
			"-h", "string:frcolor:"+Theme.Border,
			"-h", "string:bgcolor:"+Theme.Background,
			"-h", "string:fgcolor:"+Theme.Foreground,
		)
	}
}

// MakoConfigSnippet returns a mako config block for ttcli-themed notifications.
func MakoConfigSnippet() string {
	return `# ttcli focus notifications — append to ~/.config/mako/config then: makoctl reload

[app-name=ttcli]
border-color=#fab387FF
background-color=#1e1e2eEE
text-color=#cdd6f4FF
border-size=3
border-radius=12
width=420
max-icon-size=128
default-timeout=0
font=monospace 12

[app-name=ttcli][urgency=critical]
border-color=#fab387FF
background-color=#1e1e2eF2
text-color=#cdd6f4FF
border-size=4
width=520
font=monospace 14
default-timeout=0
`
}
