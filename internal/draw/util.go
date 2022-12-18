package draw

import (
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type drawFunc func(x, y int, r rune, s tcell.Style)

// offset returns a new drawFunc final based on prev that is offset by xOff and
// yOff.
func offset(prev drawFunc, xOff, yOff int) (final drawFunc) {
	return func(x, y int, r rune, s tcell.Style) {
		prev(x+xOff, y+yOff, r, s)
	}
}

// fill uses d to fill from x 0..w to y 0..h (both non-inclusive).
func fill(d drawFunc, w, h int, r rune) {
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			d(x, y, r, tcell.StyleDefault)
		}
	}
}

// clear uses d to clear from x 0..w to y 0..h (both non-inclusive).
func clear(d drawFunc, w, h int) {
	fill(d, w, h, ' ')
}

func drawString(d drawFunc, maxW int, s string, style tcell.Style) int {
	x := 0
	for r, rl := utf8.DecodeRuneInString(s); len(s) > 0; r, rl = utf8.DecodeRuneInString(s) {
		w := runewidth.RuneWidth(r)
		if maxW >= 0 {
			if x+w >= maxW && !(x+w == maxW && len(s) == rl) {
				for ; x < maxW; x++ {
					d(x, 0, '…', style)
				}
				return x
			}
		}
		d(x, 0, r, style)

		x += w
		s = s[rl:]
	}
	return x
}

func centeredString(d drawFunc, w, h int, s string) {
	textY := h / 2
	clear(d, w, h)
	drawString(offset(d, (w-len(s))/2, textY), w, s, tcell.StyleDefault.
		Dim(true).Italic(true).Foreground(tcell.ColorGray))
	clear(offset(d, 0, textY+1), w, h-textY-1)
}
