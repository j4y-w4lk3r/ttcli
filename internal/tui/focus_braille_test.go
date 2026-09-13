package tui

import (
	"testing"
	"time"
)

func TestBrailleMask(t *testing.T) {
	if brailleMask(0, 0) != 0x01 {
		t.Fatal("top-left dot")
	}
	if brailleMask(1, 3) != 0x80 {
		t.Fatal("bottom-right dot")
	}
}

func TestRenderFocusRingBraille(t *testing.T) {
	lines := renderFocusRing([]focusSlice{
		{Title: "CLI", Secs: 300, Color: colorPeach},
		{Title: "Read", Secs: 600, Color: colorTeal},
	}, nil, time.Now(), 30)
	if len(lines) < 6 {
		t.Fatalf("expected several braille rows, got %d", len(lines))
	}
	found := false
	for _, ln := range lines {
		for _, r := range stripANSI(ln) {
			if r >= 0x2801 && r <= 0x28ff {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatalf("no braille ring rendered")
	}
}

func TestColorAtAngle(t *testing.T) {
	slices := []focusSlice{
		{Secs: 50, Color: colorPeach},
		{Secs: 50, Color: colorTeal},
	}
	if colorAtAngle(0, slices, 100) != colorPeach {
		t.Fatal("top should be first slice")
	}
}

func TestDonutRingIsCircular(t *testing.T) {
	charW, charH := donutRingSize(30)
	canvas := newBrailleCanvas(charW, charH)
	if canvas.pixelW != canvas.pixelH {
		t.Fatalf("pixel grid not square: %dx%d", canvas.pixelW, canvas.pixelH)
	}
	drawDonut(canvas, []focusSlice{
		{Secs: 100, Color: colorPeach},
		{Secs: 100, Color: colorTeal},
	}, colorOverlay)

	minX, maxX, minY, maxY, ok := ringPixelExtents(canvas)
	if !ok {
		t.Fatal("no ring pixels")
	}
	w := float64(maxX - minX)
	h := float64(maxY - minY)
	ratio := w / h
	if ratio < 0.88 || ratio > 1.12 {
		t.Fatalf("ring bbox ratio=%.2f (w=%.0f h=%.0f) want ~1.0", ratio, w, h)
	}
}

func TestBrailleCanvasSquareOnScreen(t *testing.T) {
	charW, charH := donutRingSize(40)
	if charW != charH*2 {
		t.Fatalf("charW=%d charH=%d not 2:1", charW, charH)
	}
	c := newBrailleCanvas(charW, charH)
	if c.pixelW != c.pixelH {
		t.Fatalf("pixels %dx%d", c.pixelW, c.pixelH)
	}
}
