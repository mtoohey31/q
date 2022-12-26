package draw

import (
	"fmt"
	"image"
	"strings"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type LyricsDrawer struct {
	Queue *[]*track.Track

	scope
}

func (l *LyricsDrawer) Draw(d drawFunc) error {
	if len(*l.Queue) == 0 {
		clear(d, l.Rectangle)
		return nil
	}

	lyrics, err := (*l.Queue)[0].Lyrics()
	if err != nil {
		return fmt.Errorf("failed to get lyrics for queue[0]: %w", err)
	}

	if lyrics == "" {
		centeredString(d, l.Rectangle, "no lyrics found")
		return nil
	}

	y := l.Min.Y
	for _, line := range strings.Split(lyrics, "\n") {
		if y >= l.Max.Y {
			break
		}
		x := drawString(d, image.Pt(l.Min.X, y), l.Max.X, line, tcell.StyleDefault)
		for ; x < l.Max.X; x++ {
			d(image.Pt(x, y), ' ', tcell.StyleDefault)
		}
		y++
	}

	clear(d, image.Rectangle{
		Min: image.Point{
			X: l.Min.X,
			Y: y,
		},
		Max: l.Max,
	})

	return nil
}

var _ Drawer = &LyricsDrawer{}
