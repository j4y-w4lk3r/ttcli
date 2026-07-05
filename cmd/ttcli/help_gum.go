package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/term"
)

func cmdHelpGum() error {
	if _, err := exec.LookPath("gum"); err != nil {
		return fmt.Errorf("gum not found (brew install gum)\n  fallback: ttcli help bt")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("gum menu needs an interactive terminal (TTY)\n  run this directly in your terminal, not via a pipe")
	}

	groupChoices := make([]string, len(commandGroups))
	for i, g := range commandGroups {
		groupChoices[i] = g.Name
	}
	group, err := gumChoose("Pick a section", groupChoices...)
	if err != nil {
		return err
	}
	if group == "" {
		fmt.Println("(cancelled)")
		return nil
	}

	var cmds []commandHelp
	for _, c := range allCommands() {
		if c.Group == group {
			cmds = append(cmds, c)
		}
	}
	if len(cmds) == 0 {
		return nil
	}

	labels := make([]string, len(cmds))
	for i, c := range cmds {
		labels[i] = fmt.Sprintf("%-10s  %s", c.Name, c.Summary)
	}
	picked, err := gumChoose("Pick a command (↑↓ enter)", labels...)
	if err != nil {
		return err
	}
	if picked == "" {
		fmt.Println("(cancelled)")
		return nil
	}
	name := strings.Fields(picked)[0]
	c, ok := lookupCommand(name)
	if !ok {
		return nil
	}

	fmt.Println()
	fmt.Println(formatCommandDetail(c))
	fmt.Println()

	action, err := gumChoose("Next", "Show example command", "Back to menu", "Quit")
	if err != nil {
		return err
	}
	switch action {
	case "Back to menu":
		return cmdHelpGum()
	case "Show example command":
		if len(c.Examples) > 0 {
			fmt.Printf("  %s\n\n", c.Examples[0])
		}
	}
	return nil
}

func gumChoose(header string, choices ...string) (string, error) {
	if len(choices) == 0 {
		return "", nil
	}
	args := []string{
		"choose",
		"--header=" + header,
		"--height=14",
		"--cursor=› ",
	}
	args = append(args, choices...)

	cmd := exec.Command("gum", args...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	var buf bytes.Buffer
	cmd.Stdout = &buf

	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			switch exit.ExitCode() {
			case 1:
				return "", nil // user cancelled (esc/ctrl+c)
			}
		}
		return "", fmt.Errorf("gum choose: %w", err)
	}
	return strings.TrimSpace(buf.String()), nil
}
