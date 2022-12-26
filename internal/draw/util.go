package draw

import (
	"image"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"golang.org/x/exp/constraints"
)

func min[T constraints.Ordered](a, b T) T {
	if a < b {
		return a
	}

	return b
}

func max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}

	return b
}

// fill uses d to fill r.
func fill(d drawFunc, r image.Rectangle, u rune) {
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			d(image.Pt(x, y), u, tcell.StyleDefault)
		}
	}
}

// clear uses d to clear r.
func clear(d drawFunc, r image.Rectangle) {
	fill(d, r, ' ')
}

func drawString(d drawFunc, o image.Point, maxX int, s string, style tcell.Style) (stopX int) {
	c := o
	for r, rl := utf8.DecodeRuneInString(s); len(s) > 0; r, rl = utf8.DecodeRuneInString(s) {
		w := runewidth.RuneWidth(r)
		if c.X+w >= maxX && !(c.X+w == maxX && len(s) == rl) {
			for ; c.X < maxX; c.X++ {
				d(c, '…', style)
			}
			return c.X
		}
		d(c, r, style)

		c.X += w
		s = s[rl:]
	}
	return c.X
}

// centeredString assumes len(s) == runewidth.StringWidth(s).
func centeredString(d drawFunc, r image.Rectangle, s string) {
	clear(d, r)
	textOrigin := image.Point{
		X: r.Min.X + max(r.Dx()-len(s), 0)/2,
		Y: r.Min.Y + (r.Dy() / 2),
	}
	drawString(d, textOrigin, r.Max.X, s, tcell.StyleDefault.
		Dim(true).Italic(true).Foreground(tcell.ColorGray))
}
