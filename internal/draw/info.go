package draw

import (
	"fmt"
	"image"

	"mtoohey.com/q/internal/track"

	"github.com/gdamore/tcell/v2"
)

type InfoDynWDrawer struct {
	Queue *[]*track.Track

	scope
}

func (i *InfoDynWDrawer) dynWDraw(d drawFunc) (stopX int, err error) {
	if len(*i.Queue) == 0 {
		return -1, nil
	}

	title, err := (*i.Queue)[0].Title()
	if err != nil {
		return 0, fmt.Errorf("failed to get title of queue[0]: %w", err)
	}

	artist, err := (*i.Queue)[0].Artist()
	if err != nil {
		return 0, fmt.Errorf("failed to get artist for queue[0]: %w", err)
	}

	tX := drawString(d, i.Min, i.Max.X, title, tcell.StyleDefault)
	aX := drawString(d, i.Min.Add(image.Pt(0, 1)), i.Max.X, artist, tcell.StyleDefault.Dim(true))

	stopX = tX
	if aX > stopX {
		stopX = aX
	}

	clear(d, image.Rect(tX, i.Min.Y, i.Max.X, i.Min.Y+1))
	clear(d, image.Rect(aX, i.Min.Y+1, i.Max.X, i.Min.Y+2))
	bottomClearR := i.Rectangle
	bottomClearR.Min.Y = i.Min.Y + 2
	clear(d, bottomClearR)

	return stopX, nil
}

var _ DynWDrawer = &InfoDynWDrawer{}
