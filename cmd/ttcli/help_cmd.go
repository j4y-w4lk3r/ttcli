package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func cmdHelp(args []string) error {
	if len(args) == 0 {
		printHelp(os.Stderr)
		return nil
	}
	switch strings.ToLower(args[0]) {
	case "gum":
		return cmdHelpGum()
	case "bt", "browse", "tui":
		return cmdHelpBrowse()
	default:
		c, ok := lookupCommand(args[0])
		if !ok {
			return fmt.Errorf("unknown help topic %q (try: ttcli help gum)", args[0])
		}
		printCommandHelp(os.Stdout, c, colorEnabled(os.Stdout))
		return nil
	}
}

func printCommandHelp(w io.Writer, c commandHelp, cEnabled bool) {
	fmt.Fprintf(w, "%s\n\n", paint(w, cEnabled, ansiBold+ansiGreen, c.Name))
	fmt.Fprintf(w, "  %s\n\n", c.Summary)
	fmt.Fprintf(w, "%s\n", paint(w, cEnabled, ansiBold, "usage"))
	fmt.Fprintf(w, "  %s\n", c.Usage)
	if len(c.Aliases) > 0 {
		fmt.Fprintf(w, "\n%s\n", paint(w, cEnabled, ansiBold, "aliases"))
		fmt.Fprintf(w, "  %s\n", strings.Join(c.Aliases, ", "))
	}
	if len(c.Examples) > 0 {
		fmt.Fprintf(w, "\n%s\n", paint(w, cEnabled, ansiBold, "examples"))
		for _, ex := range c.Examples {
			fmt.Fprintf(w, "  %s\n", ex)
		}
	}
}

func formatCommandDetail(c commandHelp) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n\n", c.Name, c.Summary)
	b.WriteString(c.Usage)
	if len(c.Examples) > 0 {
		b.WriteString("\n\nexamples:\n")
		for _, ex := range c.Examples {
			b.WriteString("  " + ex + "\n")
		}
	}
	return b.String()
}
