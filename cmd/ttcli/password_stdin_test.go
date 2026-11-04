package main

import (
	"strings"
	"testing"
)

func TestReadPassword(t *testing.T) {
	cases := []struct {
		in, want, err string
	}{
		{in: "s3cret\n", want: "s3cret"},
		{in: "s3cret\r\n", want: "s3cret"},
		{in: "s3cret", want: "s3cret"},
		{in: "with space and  tabs\t\n", want: "with space and  tabs\t"},
		{in: "", err: "no password"},
		{in: "\n", err: "no password"},
		{in: "line1\nline2\n", err: "more than one line"},
		{in: strings.Repeat("x", maxPasswordBytes+1), err: "longer than"},
	}
	for _, c := range cases {
		got, err := readPassword(strings.NewReader(c.in))
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("readPassword(%q) error = %v, want %q", c.in, err, c.err)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("readPassword(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}
