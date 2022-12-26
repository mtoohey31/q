package draw

import (
	"errors"
	"image"
	"math"
	"os"

	"mtoohey.com/q/internal/termimage"
	"mtoohey.com/q/internal/track"
)

// CoverDynWDrawer draws the cover of a song. It breaks all the rules cause it
// writes directly to stdout instead of calling the drawFunc, and it gets its
// position absolutely using absHeight then does math assuming it's in the
// bottom left corner...
type CoverDynWDrawer struct {
	Queue *[]*track.Track

	// AbsHeight fetch the full height of the screen
	AbsHeight func() int

	// prevCover is the cover that was drawn last time. This is used to
	// determine whether we need to re-send the image.
	prevCover image.Image
	// prevW is the width that was occupied when prevCover was drawn.
	prevW *int
}

func (c *CoverDynWDrawer) Clear() error {
	err := termimage.ClearImages(os.Stdout)
	if err != nil {
		if errors.Is(err, termimage.ErrTerminalUnsupported) {
			return nil
		}

		return err
	}

	c.prevCover = nil
	z := 0
	c.prevW = &z

	return nil
}

func (c *CoverDynWDrawer) dynWDraw(d drawFunc, maxW, h int) (w int, err error) {
	if len(*c.Queue) == 0 {
		if c.prevCover != nil {
			return -1, c.Clear()
		}

		return -1, nil
	}

	cover, err := (*c.Queue)[0].Cover()
	if err != nil {
		return 0, err
	}

	if cover == nil {
		return -1, c.Clear()
	}

	if cover != nil && cover == c.prevCover {
		return *c.prevW, nil
	}

	coverBounds := cover.Bounds()
	coverAspectRatio := float64(coverBounds.Dx()) / float64(coverBounds.Dy())
	cellAspectRatio := 2.28
	w = int(math.Round(float64(h) * coverAspectRatio * cellAspectRatio))
	imageH := h
	if w > maxW {
		w = maxW
		imageH = int(math.Round(float64(w) / coverAspectRatio / cellAspectRatio))
	}

	err = termimage.WriteImage(os.Stdout, cover, image.Rect(0, c.AbsHeight()-h, w, c.AbsHeight()-h+imageH))
	if err != nil {
		if errors.Is(err, termimage.ErrTerminalUnsupported) {
			return -1, nil
		}

		return 0, err
	}

	clear(d, w, h)

	c.prevCover = cover
	c.prevW = &w

	return w, nil
}
