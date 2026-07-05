package main

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	ansiReset  = "\033[0m"
	ansiBold   = "\033[1m"
	ansiDim    = "\033[2m"
	ansiCyan   = "\033[36m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

func colorEnabled(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func paint(w io.Writer, enabled bool, code, s string) string {
	if !enabled {
		return s
	}
	return code + s + ansiReset
}

func printHelp(w io.Writer) {
	c := colorEnabled(w)
	title := paint(w, c, ansiBold+ansiCyan, "ttcli") + " — TickTick from the terminal\n\n"

	sections := []struct {
		head string
		lines []string
	}{
		{
			head: "getting started",
			lines: []string{
				"ttcli login              mint session (1Password)",
				"ttcli status [--ping]    version + auth health",
				"ttcli tree               folder → list tree",
			},
		},
		{
			head: "lists & folders",
			lines: []string{
				"ttcli ls [--tree]        flat or tree project list",
				"ttcli folder add|rename|rm <name>",
				"ttcli project add|rename|move|rm",
			},
		},
		{
			head: "tasks",
			lines: []string{
				"ttcli tasks <list|all> [--completed]",
				"ttcli add <title> [-p LIST] [-P prio] [-n NOTE]",
				"ttcli edit <task> [--title T] [-n NOTE] [-P prio] [-p LIST]",
				"ttcli due <task> -d DATE [-t TIME]",
				"ttcli done <list> <id>    ttcli rm <list> <id>",
			},
		},
		{
			head: "focus & misc",
			lines: []string{
				"ttcli focus [DATE]       pomodoro summary",
				"ttcli pomo               short focus line (tmux)",
				"ttcli help gum|bt        interactive help menus",
				"ttcli shell              interactive REPL",
				"ttcli raw <path>         debug API GET",
				"ttcli version",
			},
		},
	}

	fmt.Fprint(w, title)
	for _, sec := range sections {
		fmt.Fprintf(w, "%s\n", paint(w, c, ansiBold+ansiYellow, sec.head))
		for _, line := range sec.lines {
			parts := strings.SplitN(line, "  ", 2)
			cmd := paint(w, c, ansiGreen, parts[0])
			if len(parts) == 2 {
				fmt.Fprintf(w, "  %s  %s\n", cmd, parts[1])
			} else {
				fmt.Fprintf(w, "  %s\n", cmd)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "%s\n", paint(w, c, ansiDim, "tips:  ttcli help tasks | ttcli help gum | ttcli help bt | ttcli shell"))
	fmt.Fprintln(w)
}
