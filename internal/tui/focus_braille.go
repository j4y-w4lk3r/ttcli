package tui

import (
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Braille cells are 2×4 dots. With charW×charH cells where charW == 2*charH,
// the pixel grid is square on screen (terminal cells are ~2× taller than wide).
const (
	brailleDotsW = 2
	brailleDotsH = 4
)

// brailleMask maps a dot within a 2×4 braille cell to its Unicode bit.
func brailleMask(dx, dy int) byte {
	return [4][2]byte{
		{0x01, 0x08},
		{0x02, 0x10},
		{0x04, 0x20},
		{0x40, 0x80},
	}[dy][dx]
}

type brailleCell struct {
	bits  byte
	color lipgloss.Color
}

type brailleCanvas struct {
	charW, charH int
	pixelW       int
	pixelH       int
	cells        [][]brailleCell
}

func newBrailleCanvas(charW, charH int) *brailleCanvas {
	if charW < 4 {
		charW = 4
	}
	if charH < 2 {
		charH = 2
	}
	charW -= charW % 2 // keep pixel grid square: 2*charW == 4*charH
	charH = charW / 2
	cells := make([][]brailleCell, charH)
	for y := range cells {
		cells[y] = make([]brailleCell, charW)
	}
	return &brailleCanvas{
		charW:  charW,
		charH:  charH,
		pixelW: charW * brailleDotsW,
		pixelH: charH * brailleDotsH,
		cells:  cells,
	}
}

func (c *brailleCanvas) setDot(px, py int, color lipgloss.Color) {
	c.setDotMode(px, py, color, false)
}

func (c *brailleCanvas) setDotOverwrite(px, py int, color lipgloss.Color) {
	c.setDotMode(px, py, color, true)
}

func (c *brailleCanvas) setDotMode(px, py int, color lipgloss.Color, overwrite bool) {
	if color == "" {
		return
	}
	cx, dx := px/brailleDotsW, px%brailleDotsW
	cy, dy := py/brailleDotsH, py%brailleDotsH
	if cx < 0 || cy < 0 || cx >= c.charW || cy >= c.charH {
		return
	}
	cell := &c.cells[cy][cx]
	mask := brailleMask(dx, dy)
	if cell.bits == 0 || overwrite {
		cell.color = color
	}
	cell.bits |= mask
}

func (c *brailleCanvas) lines() []string {
	lines := make([]string, c.charH)
	for y := 0; y < c.charH; y++ {
		var b strings.Builder
		for x := 0; x < c.charW; x++ {
			cell := c.cells[y][x]
			if cell.bits == 0 {
				b.WriteRune(' ')
				continue
			}
			ch := string(rune(0x2800 + rune(cell.bits)))
			b.WriteString(lipgloss.NewStyle().Foreground(cell.color).Render(ch))
		}
		lines[y] = b.String()
	}
	return lines
}

func donutRingSize(width int) (charW, charH int) {
	charW = 20
	if width > 0 && width < charW+4 {
		charW = max(12, width-2)
	}
	charW -= charW % 2
	charH = charW / 2
	return charW, charH
}

type ringGeom struct {
	cx, cy, innerR, outerR float64
}

func newRingGeom(canvas *brailleCanvas) ringGeom {
	cx := float64(canvas.pixelW) / 2
	cy := float64(canvas.pixelH) / 2
	r := math.Min(cx, cy) * 0.90
	return ringGeom{
		cx:     cx,
		cy:     cy,
		outerR: r,
		innerR: r * 0.60,
	}
}

func (g ringGeom) distance(px, py int) float64 {
	dx := float64(px) + 0.5 - g.cx
	dy := float64(py) + 0.5 - g.cy
	return math.Hypot(dx, dy)
}

func (g ringGeom) inRing(px, py int) bool {
	d := g.distance(px, py)
	return d >= g.innerR-0.40 && d <= g.outerR+0.40
}

func (g ringGeom) angle(px, py int) float64 {
	dx := float64(px) + 0.5 - g.cx
	dy := float64(py) + 0.5 - g.cy
	a := math.Atan2(dy, dx) + math.Pi/2
	if a < 0 {
		a += 2 * math.Pi
	}
	return a
}

const sliceGapRad = 0.05

type ringSegment struct {
	start, end float64
	color      lipgloss.Color
}

func buildRingSegments(slices []focusSlice, total int) []ringSegment {
	if total <= 0 || len(slices) == 0 {
		return nil
	}
	gap := sliceGapRad
	usable := 2*math.Pi - gap*float64(len(slices))
	start := gap / 2
	var segs []ringSegment
	for _, s := range slices {
		sweep := (float64(s.Secs) / float64(total)) * usable
		if sweep < 0.02 {
			sweep = 0.02
		}
		segs = append(segs, ringSegment{start: start, end: start + sweep, color: s.Color})
		start += sweep + gap
	}
	return segs
}

func colorForAngle(angle float64, segs []ringSegment, fallback lipgloss.Color) lipgloss.Color {
	for _, s := range segs {
		if angle >= s.start && angle < s.end {
			return s.color
		}
	}
	return fallback
}

func drawDonut(canvas *brailleCanvas, slices []focusSlice, emptyColor lipgloss.Color) {
	geom := newRingGeom(canvas)
	total := totalFocusSecs(slices)
	segs := buildRingSegments(slices, total)

	for py := 0; py < canvas.pixelH; py++ {
		for px := 0; px < canvas.pixelW; px++ {
			if !geom.inRing(px, py) {
				continue
			}
			var col lipgloss.Color
			if len(segs) == 0 {
				col = emptyColor
			} else {
				col = colorForAngle(geom.angle(px, py), segs, colorOverlay)
			}
			canvas.setDot(px, py, col)
		}
	}
}

// drawSessionArc overlays live timer progress on the outer band (top = 0, clockwise).
func drawSessionArc(canvas *brailleCanvas, elapsed, duration time.Duration, col lipgloss.Color) {
	if duration <= 0 || elapsed <= 0 {
		return
	}
	frac := float64(elapsed) / float64(duration)
	if frac > 1 {
		frac = 1
	}
	if frac > 0 && frac < 0.05 {
		frac = 0.05
	}
	drawSessionArcFrac(canvas, 0, frac, col)
}

// drawSessionOvertimeArc draws the post-deadline slice after the planned arc completes.
func drawSessionOvertimeArc(canvas *brailleCanvas, overtime, duration time.Duration, col lipgloss.Color) {
	if duration <= 0 || overtime <= 0 {
		return
	}
	frac := float64(overtime) / float64(duration)
	drawSessionArcFrac(canvas, 1, 1+frac, col)
}

func drawSessionArcFrac(canvas *brailleCanvas, startFrac, endFrac float64, col lipgloss.Color) {
	if endFrac <= startFrac {
		return
	}
	startAngle := startFrac * 2 * math.Pi
	endAngle := endFrac * 2 * math.Pi
	if endAngle-startAngle < 0.05 {
		endAngle = startAngle + 0.05
	}

	geom := newRingGeom(canvas)
	bandInner := geom.innerR + (geom.outerR-geom.innerR)*0.5
	for py := 0; py < canvas.pixelH; py++ {
		for px := 0; px < canvas.pixelW; px++ {
			d := geom.distance(px, py)
			if d < bandInner-0.40 || d > geom.outerR+0.40 {
				continue
			}
			a := geom.angle(px, py)
			if a >= startAngle && a <= endAngle {
				canvas.setDotOverwrite(px, py, col)
			}
		}
	}
}

func colorAtAngle(angle float64, slices []focusSlice, total int) lipgloss.Color {
	if total <= 0 || len(slices) == 0 {
		return colorOverlay
	}
	start := 0.0
	for _, s := range slices {
		sweep := (float64(s.Secs) / float64(total)) * 2 * math.Pi
		end := start + sweep
		if angle >= start && angle < end {
			return s.Color
		}
		start = end
	}
	return slices[len(slices)-1].Color
}

func overlayCenterLabel(lines []string, label string, width int) []string {
	if len(lines) == 0 {
		return lines
	}
	if strings.Contains(label, "\n") {
		parts := strings.Split(label, "\n")
		mid := len(lines) / 2
		start := mid - len(parts)/2
		for i, part := range parts {
			row := start + i
			if row < 0 || row >= len(lines) {
				continue
			}
			pad := max(0, (width-lipgloss.Width(part))/2)
			lines[row] = truncateInner(strings.Repeat(" ", pad)+part, width)
		}
		return lines
	}
	mid := len(lines) / 2
	pad := max(0, (width-lipgloss.Width(label))/2)
	centered := strings.Repeat(" ", pad) + label
	lines[mid] = truncateInner(centered, width)
	return lines
}

// ringPixelExtents returns the bounding box of lit pixels (for tests).
func ringPixelExtents(canvas *brailleCanvas) (minX, maxX, minY, maxY int, ok bool) {
	minX, minY = canvas.pixelW, canvas.pixelH
	for py := 0; py < canvas.pixelH; py++ {
		for px := 0; px < canvas.pixelW; px++ {
			cx, dx := px/brailleDotsW, px%brailleDotsW
			cy, dy := py/brailleDotsH, py%brailleDotsH
			if canvas.cells[cy][cx].bits&brailleMask(dx, dy) == 0 {
				continue
			}
			ok = true
			if px < minX {
				minX = px
			}
			if px > maxX {
				maxX = px
			}
			if py < minY {
				minY = py
			}
			if py > maxY {
				maxY = py
			}
		}
	}
	return minX, maxX, minY, maxY, ok
}
