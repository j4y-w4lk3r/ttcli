package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/j4y-w4lk3r/ttcli/internal/notify"
	"github.com/j4y-w4lk3r/ttcli/internal/tui"
)

func cmdFocusNotifyTest(args []string) error {
	fs := flag.NewFlagSet("focus notify-test", flag.ExitOnError)
	task := fs.String("task", "Cancel Proton Pass manager", "sample task title in notification body")
	all := fs.Bool("all", false, "send every design variant (3s apart)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() == 0 {
		printNotifyTestUsage(os.Stdout)
		return nil
	}

	name := fs.Arg(0)
	if name == "list" {
		printNotifyTestUsage(os.Stdout)
		return nil
	}

	if name == "nt-escalate" {
		if err := notify.FocusDone(*task, false); err != nil {
			return err
		}
		fmt.Println("sent nt escalate notification (dunst → growing overlay)")
		return nil
	}

	if *all || name == "all" {
		for i, v := range notify.Variants() {
			if i > 0 {
				time.Sleep(3 * time.Second)
			}
			fmt.Fprintf(os.Stderr, "→ %s (%s)\n", v.Title, v.Description)
			if err := notify.SendVariant(v.ID, *task, false); err != nil {
				return fmt.Errorf("%s: %w", v.ID, err)
			}
		}
		fmt.Println("sent all notification variants")
		return nil
	}

	v, ok := notify.ParseVariant(name)
	if !ok {
		return fmt.Errorf("unknown variant %q — run: ttcli focus notify-test list", name)
	}
	if v == notify.VariantEscalated || name == "nt-escalate" {
		if err := notify.FocusDone(*task, false); err != nil {
			return err
		}
		fmt.Println("sent nt escalate notification (dunst → growing overlay)")
		return nil
	}
	if err := notify.SendVariant(v, *task, false); err != nil {
		return err
	}
	fmt.Printf("sent %q notification\n", v)
	return nil
}

func printNotifyTestUsage(w fmtWriter) {
	fmt.Fprintln(w, "Usage: ttcli focus notify-test <variant> [--task TITLE]")
	fmt.Fprintln(w, "       ttcli focus notify-test all [--task TITLE]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Variants:")
	for _, v := range notify.Variants() {
		fmt.Fprintf(w, "  %-10s %s\n", v.ID, v.Description)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Tips:")
	fmt.Fprintln(w, "  ttcli focus notify-test escalated   nt escalate smooth overlay (same as focus end)")
	fmt.Fprintln(w, "  nt escalate --delay 2 \"Title\" \"Body\"   quick manual test")
	fmt.Fprintln(w, "  ttcli focus notify-setup     mako theme for peach border + larger critical alerts")
	fmt.Fprintln(w, "  ttcli focus alert-preview    terminal full-screen alert (normal vs escalated)")
}

type fmtWriter interface {
	Write([]byte) (int, error)
}

func cmdFocusNotifySetup(_ []string) error {
	path := os.Getenv("HOME") + "/.config/mako/config"
	fmt.Print(notify.MakoConfigSnippet())
	fmt.Printf("\n# Append the block above to %s then run: makoctl reload\n", path)
	fmt.Println("# dunst users: themed hints are sent automatically (frcolor/bgcolor/fgcolor).")
	return nil
}

func cmdFocusAlertPreview(args []string) error {
	fs := flag.NewFlagSet("focus alert-preview", flag.ExitOnError)
	escalated := fs.Bool("escalated", false, "preview the 45s+ mega alert with dim backdrop")
	width := fs.Int("width", 100, "terminal width")
	height := fs.Int("height", 28, "terminal height")
	task := fs.String("task", "Cancel Proton Pass manager", "task title shown in alert")
	if err := fs.Parse(args); err != nil {
		return err
	}
	out := tui.PreviewFocusAlert(*width, *height, *escalated, *task)
	fmt.Println(out)
	mode := "normal"
	if *escalated {
		mode = "escalated (45s+)"
	}
	fmt.Fprintf(os.Stderr, "preview: %s alert %dx%d\n", mode, *width, *height)
	return nil
}

func cmdFocusNotify(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: ttcli focus notify-test|notify-setup|alert-preview")
	}
	switch args[0] {
	case "notify-test", "test-notify":
		return cmdFocusNotifyTest(args[1:])
	case "notify-setup", "setup-notify":
		return cmdFocusNotifySetup(args[1:])
	case "alert-preview", "preview-alert":
		return cmdFocusAlertPreview(args[1:])
	default:
		return fmt.Errorf("unknown focus notify command %q", args[0])
	}
}
