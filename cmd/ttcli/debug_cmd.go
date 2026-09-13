package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/tui"
	"golang.org/x/term"
)

func cmdDebug(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: ttcli debug <frame|termsize> …")
	}
	switch args[0] {
	case "frame":
		return cmdDebugFrame(args[1:])
	case "termsize":
		return cmdDebugTermsize(args[1:])
	default:
		return fmt.Errorf("unknown debug subcommand: %s (try frame, termsize)", args[0])
	}
}

func cmdDebugFrame(args []string) error {
	fs := flag.NewFlagSet("debug frame", flag.ContinueOnError)
	width := fs.Int("width", 120, "terminal width in columns")
	height := fs.Int("height", 40, "terminal height in rows")
	view := fs.String("view", "tasks", "view: tasks|calendar|pomo|habits")
	check := fs.Bool("check", false, "exit 1 if layout invariants fail")
	dump := fs.Bool("dump", false, "print per-line width dump to stderr")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *width < 20 || *height < 10 {
		return fmt.Errorf("width/height too small (need at least 20x10)")
	}

	lines, report, ok := tui.DebugFrameLines(*width, *height, *view)
	if *dump {
		fmt.Fprintln(os.Stderr, report)
		fmt.Fprint(os.Stderr, tui.FormatFrameDump(lines, 100))
	}
	if *check && !ok {
		fmt.Fprintln(os.Stderr, report)
		return fmt.Errorf("layout check failed for %dx%d view=%s", *width, *height, *view)
	}
	if !*check && !ok {
		fmt.Fprintln(os.Stderr, report)
	}
	fmt.Print(strings.Join(lines, "\n"))
	return nil
}

func cmdDebugTermsize(_ []string) error {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return fmt.Errorf("term.GetSize: %w (need a TTY — try: script -q -c 'ttcli debug termsize' /dev/null)", err)
	}
	fmt.Printf("term.GetSize(stdout): %dx%d\n", w, h)
	if cols := os.Getenv("COLUMNS"); cols != "" {
		fmt.Printf("$COLUMNS: %s\n", cols)
	}
	if lines := os.Getenv("LINES"); lines != "" {
		fmt.Printf("$LINES: %s\n", lines)
	}
	return nil
}
