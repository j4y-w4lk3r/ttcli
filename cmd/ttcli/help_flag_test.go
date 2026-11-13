package main

import "testing"

func TestWantsHelp(t *testing.T) {
	cases := []struct {
		args []string
		want bool
	}{
		{[]string{"--help"}, true},
		{[]string{"buy", "milk", "-h"}, true},
		{[]string{"--", "--help"}, false},
		{[]string{"buy milk", "-p", "Home"}, false},
		{nil, false},
	}
	for _, c := range cases {
		if got := wantsHelp(c.args); got != c.want {
			t.Errorf("wantsHelp(%q) = %v, want %v", c.args, got, c.want)
		}
	}
}

func TestAddRejectsUnknownFlag(t *testing.T) {
	// Must fail before any network call.
	if err := cmdAdd([]string{"--bogus", "title"}); err == nil {
		t.Fatal("cmdAdd accepted an unknown flag as title")
	}
}
