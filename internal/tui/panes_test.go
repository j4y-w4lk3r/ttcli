package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderPaneCompleteBorder(t *testing.T) {
	l := layout{
		leftBoxW:   30,
		bodyLines:  10,
		innerLines: 8,
	}
	longFolder := strings.Repeat("X", 60)
	content := fillInner(
		headerStyle.Render(iconList+" Lists"),
		"scroll",
		[]string{folderStyle.Render(iconFolder + " " + longFolder)},
		l.innerLines,
	)
	pane := renderPane(content, l, l.leftBoxW, true)
	lines := strings.Split(pane, "\n")
	if len(lines) != l.bodyLines {
		t.Fatalf("got %d lines, want %d", len(lines), l.bodyLines)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != l.leftBoxW {
			t.Errorf("line %d width=%d want %d", i, w, l.leftBoxW)
		}
	}
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "╰") || lastRune(last) != '╯' {
		t.Fatalf("missing bottom border, last line: %q", last)
	}
	if lastRune(lines[0]) != '╮' {
		t.Fatalf("missing top-right corner, first line: %q", lines[0])
	}
	for i, line := range lines[1 : len(lines)-1] {
		if strings.HasPrefix(line, "│") && lastRune(line) != '│' {
			t.Fatalf("line %d missing right border: %q", i+1, line)
		}
	}
}

func TestRenderPaneFullWidth(t *testing.T) {
	termW := 100
	l := layout{
		termW:      termW,
		fullW:      termW - 4,
		bodyLines:  12,
		innerLines: 10,
	}
	content := strings.Repeat("W", 200) + "\n" + strings.Repeat("Z", 200)
	pane := renderPane(content, l, l.termW, false)
	lines := strings.Split(pane, "\n")
	if len(lines) != l.bodyLines {
		t.Fatalf("got %d lines, want %d", len(lines), l.bodyLines)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != termW {
			t.Errorf("line %d width=%d want %d", i, w, termW)
		}
	}
	if !strings.HasPrefix(lines[0], "╭") || lastRune(lines[0]) != '╮' {
		t.Fatal("missing top border corners")
	}
	if !strings.HasPrefix(lines[len(lines)-1], "╰") || lastRune(lines[len(lines)-1]) != '╯' {
		t.Fatal("missing bottom border corners")
	}
}

func TestJoinHorizPanesNoMidLineBreak(t *testing.T) {
	l := layout{leftBoxW: 30, rightBoxW: 50, bodyLines: 6, innerLines: 4}
	left := renderPane("left\ncontent", l, l.leftBoxW, false)
	right := renderPane("right\ncontent", l, l.rightBoxW, false)
	joined := joinHorizPanes(left, right, l.bodyLines, l.leftBoxW, l.rightBoxW, l.leftBoxW+1+l.rightBoxW)
	for i, line := range strings.Split(joined, "\n") {
		if strings.Count(line, "╭")+strings.Count(line, "╰") > 0 && strings.Contains(line, "\n") {
			t.Fatalf("line %d contains embedded newline", i)
		}
		if lipgloss.Width(line) != l.leftBoxW+1+l.rightBoxW {
			t.Errorf("line %d width=%d want %d", i, lipgloss.Width(line), l.leftBoxW+1+l.rightBoxW)
		}
	}
}

func TestFitViewClampsWidth(t *testing.T) {
	w, h := 80, 24
	wide := strings.Repeat("x", 200)
	view := fitView("header\n"+wide+"\nfooter", w, h)
	for i, line := range strings.Split(view, "\n") {
		if lw := lipgloss.Width(line); lw > w {
			t.Fatalf("line %d width %d exceeds terminal %d", i, lw, w)
		}
	}
	if len(strings.Split(view, "\n")) != h {
		t.Fatalf("want %d lines got %d", h, len(strings.Split(view, "\n")))
	}
}
