package draw

import (
	"fmt"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type InfoDynWDrawer struct {
	Queue *[]*track.Track
}

func (i *InfoDynWDrawer) dynWDraw(d drawFunc, maxW, _ int) (w int, err error) {
	if len(*i.Queue) == 0 {
		return -1, nil
	}

	title, err := (*i.Queue)[0].Title()
	if err != nil {
		return -1, fmt.Errorf("failed to get title of queue[0]: %w", err)
	}

	artist, err := (*i.Queue)[0].Artist()
	if err != nil {
		return -1, fmt.Errorf("failed to get artist for queue[0]: %w", err)
	}

	tW := drawString(d, maxW, title, tcell.StyleDefault)
	aW := drawString(offset(d, 0, 1), maxW, artist, tcell.StyleDefault.Dim(true))

	w = tW
	if aW > w {
		w = aW
	}

	clear(offset(d, tW, 0), w-tW, 1)
	clear(offset(d, aW, 1), w-aW, 1)
	clear(offset(d, 0, 2), w, 1)

	return w, nil
}

var _ DynWDrawer = &InfoDynWDrawer{}
