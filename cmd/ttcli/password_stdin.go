package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// maxPasswordBytes bounds what --password-stdin reads, so a wrong pipe
// (a whole file, a binary) is refused instead of sent to TickTick.
const maxPasswordBytes = 4096

// passwordFromStdin reads the password for `login --password-stdin`.
// It refuses a terminal (a typed password would echo and land in scrollback)
// and anything but a single line, e.g.
//
//	keyhole get VAULT -i ID ticktick | ttcli login --email you@x --password-stdin
func passwordFromStdin() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("--password-stdin reads a pipe, not a terminal (e.g. keyhole get ... | ttcli login --email ... --password-stdin)")
	}
	return readPassword(os.Stdin)
}

func readPassword(r io.Reader) (string, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxPasswordBytes+1))
	if err != nil {
		return "", fmt.Errorf("read password from stdin: %w", err)
	}
	if len(buf) > maxPasswordBytes {
		return "", fmt.Errorf("password on stdin is longer than %d bytes", maxPasswordBytes)
	}
	buf = bytes.TrimSuffix(buf, []byte("\n"))
	buf = bytes.TrimSuffix(buf, []byte("\r"))
	if len(buf) == 0 {
		return "", errors.New("no password on stdin")
	}
	if bytes.ContainsAny(buf, "\r\n") {
		return "", errors.New("stdin has more than one line; pipe only the password")
	}
	return string(buf), nil
}
