package tui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

var errClipboard = errors.New("clipboard: unavailable")

func TestYankCopiesTheSelectedTaskTitle(t *testing.T) {
	prev := clipboardWrite
	var got string
	clipboardWrite = func(text string) error {
		got = text
		return nil
	}
	t.Cleanup(func() { clipboardWrite = prev })

	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	m.taskCursor = 0
	want := m.tasks[0].Title

	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil {
		t.Fatal("y should copy the selected title")
	}
	if out.errMsg != "" {
		t.Fatalf("unexpected error %q", out.errMsg)
	}
	msg := cmd().(taskTitleCopiedMsg)
	if msg.err != nil || msg.title != want || got != want {
		t.Fatalf("msg=%+v copied=%q want=%q", msg, got, want)
	}
	updated, _ := out.Update(msg)
	next := updated.(model)
	if !strings.Contains(next.toast, want) {
		t.Fatalf("toast %q", next.toast)
	}
}

func TestYankOnTheListsPaneDoesNothing(t *testing.T) {
	m := fixtureModel(100, 30)
	m.paneFocus = paneLists
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd != nil || out.errMsg != "" {
		t.Fatalf("cmd=%v err=%q", cmd != nil, out.errMsg)
	}
}

func TestYankReportsAClipboardFailure(t *testing.T) {
	prev := clipboardWrite
	clipboardWrite = func(string) error { return errClipboard }
	t.Cleanup(func() { clipboardWrite = prev })

	m := fixtureModel(100, 30)
	m.paneFocus = paneTasks
	out, cmd := m.updateTasksKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	updated, _ := out.Update(cmd())
	next := updated.(model)
	if !strings.Contains(next.errMsg, "clipboard") {
		t.Fatalf("err %q", next.errMsg)
	}
}

func TestOSC52WrapsForTmux(t *testing.T) {
	plain := osc52Sequence("Buy milk", false)
	if !strings.HasPrefix(plain, "\033]52;c;") || !strings.HasSuffix(plain, "\a") {
		t.Fatalf("plain %q", plain)
	}
	wrapped := osc52Sequence("Buy milk", true)
	if !strings.HasPrefix(wrapped, "\033Ptmux;\033") || !strings.Contains(wrapped, plain) {
		t.Fatalf("tmux %q", wrapped)
	}
}

func TestCopiedTitleToastTruncates(t *testing.T) {
	long := strings.Repeat("a", 60)
	got := copiedTitleToast("  " + long + "  ")
	if len([]rune(got)) != 48 || !strings.HasSuffix(got, "…") {
		t.Fatalf("toast %q len %d", got, len([]rune(got)))
	}
}
