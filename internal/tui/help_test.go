package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRenderHelpLineSelectedHighlight(t *testing.T) {
	keyColW := helpKeyColWidth()
	row := helpRow{key: "j / k", desc: "move selection"}
	sel := renderHelpLine(row, true, 56, keyColW, "")
	idle := renderHelpLine(row, false, 56, keyColW, "")

	if !strings.Contains(stripANSI(sel), "▸") {
		t.Fatalf("selected row missing marker: %q", previewLine(sel, 56))
	}
	if strings.Contains(stripANSI(idle), "▸") {
		t.Fatal("idle row should not have marker")
	}
	if lipgloss.Width(sel) > 56 {
		t.Fatalf("selected width=%d", lipgloss.Width(sel))
	}
}

func TestHelpLineDescAlignment(t *testing.T) {
	keyColW := helpKeyColWidth()
	w := 64
	var viewRow, refreshRow helpRow
	for _, row := range helpRows {
		if row.key == "1 / 2 / 3 / 4" {
			viewRow = row
		}
		if row.key == "r / R" {
			refreshRow = row
		}
	}
	a := stripANSI(renderHelpLine(viewRow, false, w, keyColW, ""))
	b := stripANSI(renderHelpLine(refreshRow, false, w, keyColW, ""))
	idxA := strings.Index(a, "switch")
	idxB := strings.Index(b, "refresh")
	if idxA < 1 || idxB < 1 || idxA != idxB {
		t.Fatalf("desc columns misaligned: %d vs %d\n%q\n%q", idxA, idxB, a, b)
	}
}

func TestHelpOverlayFullWidthBackground(t *testing.T) {
	withTrueColor(t)
	m := fixtureModel(120, 40)
	m.showHelp = true
	m.helpCursor = 0
	out := m.renderHelpOverlay()
	lines := strings.Split(out, "\n")
	if len(lines) != 40 {
		t.Fatalf("rows=%d want 40", len(lines))
	}
	accent := map[string]bool{ansiBGMauve: true}
	for i, line := range lines {
		if lipgloss.Width(line) != 120 {
			t.Fatalf("row %d width=%d want 120", i, lipgloss.Width(line))
		}
		plain := stripANSI(line)
		label := fmt.Sprintf("screen row %d", i)
		if isHelpBoxLine(plain) {
			assertNoHardcodedFillBackgrounds(t, line, accent, label)
		} else {
			assertNoHardcodedFillBackgrounds(t, line, nil, label)
		}
	}
}

func TestHelpBoxNoHardcodedFill(t *testing.T) {
	withTrueColor(t)
	innerW := 56
	keyColW := helpKeyColWidth()
	var content []string
	content = append(content, helpInnerLine(helpTitleInBoxStyle.Render(iconHelp+"  Keybindings"), innerW))
	content = append(content, helpInnerLine(helpHintInBoxStyle.Render("j/k scroll · esc or ? close"), innerW))
	content = append(content, helpInnerLine("", innerW))
	for _, row := range helpRows {
		if row.key == "1 / 2 / 3 / 4" {
			content = append(content, renderHelpLine(row, false, innerW, keyColW, ""))
			break
		}
	}
	box := helpBoxStyle.Width(innerW + 4).Render(strings.Join(content, "\n"))
	for i, line := range strings.Split(box, "\n") {
		assertNoHardcodedFillBackgrounds(t, line, nil, fmt.Sprintf("box row %d", i))
	}
}

func TestHelpLongDescTruncates(t *testing.T) {
	keyColW := helpKeyColWidth()
	row := helpRow{key: "s / f", desc: "open focus picker (choose task + duration; defaults 25m / 5m)"}
	line := stripANSI(renderHelpLine(row, false, 56, keyColW, ""))
	if strings.Contains(line, "\n") {
		t.Fatal("help line should not wrap")
	}
	if lipgloss.Width(line) > 56 {
		t.Fatalf("line too wide: %d", lipgloss.Width(line))
	}
}

func TestHelpFilterMatchesDescription(t *testing.T) {
	indices := helpFilteredIndices("calendar")
	if len(indices) == 0 {
		t.Fatal("expected calendar matches")
	}
	found := false
	for _, i := range indices {
		if strings.Contains(strings.ToLower(helpRows[i].desc), "calendar") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("calendar filter did not match description text")
	}
}

func TestHelpFilterHighlight(t *testing.T) {
	row := helpRow{key: "r / R", desc: "refresh current view"}
	line := stripANSI(renderHelpLine(row, false, 64, helpKeyColWidth(), "refresh"))
	if !strings.Contains(line, "refresh") {
		t.Fatalf("expected match text in line: %q", line)
	}
}

func TestHelpFilterNoMatches(t *testing.T) {
	if len(helpFilteredIndices("zzzznotakey")) != 0 {
		t.Fatal("expected no matches")
	}
}
