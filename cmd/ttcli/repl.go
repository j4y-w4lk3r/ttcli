package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/j4y-w4lk3r/ttcli/internal/version"
)

func cmdShell(_ []string) error {
	os.Setenv("TTCLI_SHELL", "1")
	defer os.Unsetenv("TTCLI_SHELL")
	in := bufio.NewReader(os.Stdin)
	fmt.Println("ttcli shell — interactive mode (type help, exit to quit)")
	fmt.Println("  tab-completion: install with  make install-completions")
	for {
		fmt.Print("ttcli> ")
		line, err := in.ReadString('\n')
		if err != nil {
			return nil
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch strings.ToLower(line) {
		case "exit", "quit", "q":
			return nil
		case "help", "?":
			printHelp(os.Stdout)
			continue
		}
		if err := dispatchLine(line); err != nil {
			fmt.Fprintf(os.Stderr, "ttcli: %v\n", err)
		}
	}
}

// dispatchLine parses one REPL line and runs it like a shell command.
func dispatchLine(line string) error {
	fields := splitShellLine(line)
	if len(fields) == 0 {
		return nil
	}
	return runCommand(fields)
}

func runCommand(args []string) error {
	if len(args) == 0 {
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		return cmdHelp(args[1:])
	case "login":
		return cmdLogin(args[1:])
	case "ls", "lists", "projects":
		return cmdLists(args[1:])
	case "tree":
		return cmdTree(args[1:])
	case "folders":
		return cmdFolder(args[1:])
	case "folder":
		return cmdFolder(args[1:])
	case "project", "list":
		return cmdProject(args[1:])
	case "edit":
		return cmdEdit(args[1:])
	case "status":
		return cmdStatus(args[1:])
	case "tasks":
		return cmdTasks(args[1:])
	case "due", "schedule":
		return cmdDue(args[1:])
	case "add":
		return cmdAdd(args[1:])
	case "done":
		return cmdDone(args[1:])
	case "rm", "delete":
		return cmdRm(args[1:])
	case "focus":
		return cmdFocus(args[1:])
	case "pomo":
		return cmdPomo(args[1:])
	case "shell", "repl":
		if isNestedShell() {
			fmt.Println("(already in shell)")
			return nil
		}
		return cmdShell(args[1:])
	case "serve", "webhook":
		return cmdServe(args[1:])
	case "raw":
		return cmdRaw(args[1:])
	case "version", "-v", "--version", "-version":
		fmt.Println(version.String())
		return nil
	default:
		return fmt.Errorf("unknown command: %s (type help)", args[0])
	}
}

func isNestedShell() bool {
	return os.Getenv("TTCLI_SHELL") == "1"
}

// splitShellLine splits on spaces but respects single/double quotes.
func splitShellLine(line string) []string {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	for _, r := range line {
		switch {
		case inQuote != 0:
			if r == inQuote {
				inQuote = 0
			} else {
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			inQuote = r
		case r == ' ' || r == '\t':
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}
