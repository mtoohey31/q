package tui

import (
	"image"
	"unicode/utf8"

	"mtoohey.com/q/internal/util"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// draw is a wrapper for t.screen.SetContent that sets a point on the screen to
// the given rune and style.
func (t *tui) draw(p image.Point, r rune, s tcell.Style) {
	t.screen.SetContent(p.X, p.Y, r, nil, s)
}

// clear uses d to clear r.
func (t *tui) clear(r image.Rectangle) {
	for x := r.Min.X; x < r.Max.X; x++ {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			t.draw(image.Pt(x, y), ' ', styleDefault)
		}
	}
}

func (t *tui) drawString(o image.Point, maxX int, s string, style tcell.Style) (stopX int) {
	c := o
	for r, rl := utf8.DecodeRuneInString(s); len(s) > 0; r, rl = utf8.DecodeRuneInString(s) {
		w := runewidth.RuneWidth(r)
		if c.X+w >= maxX && !(c.X+w == maxX && len(s) == rl) {
			for ; c.X < maxX; c.X++ {
				t.draw(c, '…', style)
			}
			return c.X
		}
		t.draw(c, r, style)

		c.X += w
		s = s[rl:]
	}
	return c.X
}

// centeredString assumes len(s) == runewidth.StringWidth(s).
func (t *tui) centeredString(r image.Rectangle, s string) {
	t.clear(r)
	textOrigin := image.Point{
		X: r.Min.X + util.Max(r.Dx()-len(s), 0)/2,
		Y: r.Min.Y + (r.Dy() / 2),
	}
	t.drawString(textOrigin, r.Max.X, s, styleDefault.
		Dim(true).Italic(true).Foreground(tcell.ColorGray))
}

func vSplitFixedBottom(r image.Rectangle, bottomH int) (top, bottom image.Rectangle) {
	topMaxY := r.Max.Y - bottomH - 1
	return image.Rect(r.Min.X, r.Min.Y, r.Max.X, topMaxY),
		image.Rect(r.Min.X, topMaxY+1, r.Max.X, r.Max.Y)
}

func hSplitRatio(r image.Rectangle, ratio float64) (left, right image.Rectangle) {
	leftMaxX := r.Min.X + int(float64(r.Dx())*ratio)
	return image.Rect(r.Min.X, r.Min.Y, leftMaxX, r.Max.Y),
		image.Rect(leftMaxX+1, r.Min.Y, r.Max.X, r.Max.Y)
}

var (
	styleDefault   = tcell.StyleDefault
	styleDim       = styleDefault.Dim(true)
	styleUnderline = styleDefault.Underline(true)
)
