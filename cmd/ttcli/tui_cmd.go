package main

import (
	"fmt"
	"os"

	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
	"github.com/j4y-w4lk3r/ttcli/internal/tui"
	"golang.org/x/term"
)

func cmdTui(_ []string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("ttcli tui needs an interactive terminal (TTY)\n  try: ssh -t … ttcli tui")
	}
	c, err := ticktick.New("", ticktick.CredentialOptions{})
	if err != nil {
		return err
	}
	return tui.Run(c)
}
