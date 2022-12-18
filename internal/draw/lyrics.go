package draw

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"mtoohey.com/q/internal/track"
)

type LyricsDrawer struct {
	Queue *[]*track.Track

	scope
}

func (l *LyricsDrawer) Draw() error {
	if len(*l.Queue) == 0 {
		clear(l.d, l.w, l.h)
		return nil
	}

	lyrics, err := (*l.Queue)[0].Lyrics()
	if err != nil {
		return fmt.Errorf("failed to get lyrics for queue[0]: %w", err)
	}

	if lyrics == "" {
		centeredString(l.d, l.w, l.h, "no lyrics found")
		return nil
	}

	y := 0
	for _, line := range strings.Split(lyrics, "\n") {
		if y >= l.h {
			break
		}
		x := drawString(offset(l.d, 0, y), l.w, line, tcell.StyleDefault)
		for ; x < l.w; x++ {
			l.d(x, y, ' ', tcell.StyleDefault)
		}
		y++
	}

	if y < l.h {
		clear(offset(l.d, 0, y), l.w, l.h-y)
	}

	return nil
}

var _ Drawer = &LyricsDrawer{}
