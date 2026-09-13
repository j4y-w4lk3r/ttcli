package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/j4y-w4lk3r/ttcli/internal/ticktick"
)

func TestListsPaneKeepsSideBordersWithFolders(t *testing.T) {
	l := layout{
		leftW:      26,
		leftBoxW:   30,
		rightBoxW:  50,
		bodyLines:  12,
		innerLines: 10,
	}
	rows := []listRow{
		{node: ticktick.ProjectTreeNode{Name: "List0", Kind: "list", Depth: 0, ID: "1"}, selectable: true},
		{node: ticktick.ProjectTreeNode{Name: "Tech", Kind: "folder", Depth: 0}, selectable: false},
		{node: ticktick.ProjectTreeNode{Name: "X", Kind: "folder", Depth: 1}, selectable: false},
		{node: ticktick.ProjectTreeNode{Name: "Y", Kind: "folder", Depth: 1}, selectable: false},
		{node: ticktick.ProjectTreeNode{Name: "PXC", Kind: "list", Depth: 1, ID: "2"}, selectable: true},
	}
	m := model{listRows: rows, listCursor: 0}
	pane := renderPane(m.renderLists(l), l, l.leftBoxW, true)
	lines := strings.Split(pane, "\n")
	for i, line := range lines {
		if i >= l.bodyLines {
			break
		}
		if lipgloss.Width(line) != l.leftBoxW {
			t.Fatalf("line %d width=%d want %d", i, lipgloss.Width(line), l.leftBoxW)
		}
		switch {
		case strings.HasPrefix(line, "╭"):
			if lastRune(line) != '╮' {
				t.Fatalf("top line missing ╮: %q", line)
			}
		case strings.HasPrefix(line, "╰"):
			if lastRune(line) != '╯' {
				t.Fatalf("bottom line missing ╯: %q", line)
			}
		case strings.HasPrefix(line, "│"):
			if lastRune(line) != '│' {
				t.Fatalf("row %d missing right │: %q", i, line)
			}
		}
	}
}
