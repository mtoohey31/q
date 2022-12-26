package draw

import (
	"errors"
	"image"
	"math"
	"os"

	"mtoohey.com/q/internal/draw/termimage"
	"mtoohey.com/q/internal/track"
)

// CoverDynWDrawer draws the cover of a song. It breaks all the rules cause it
// writes directly to stdout instead of calling the drawFunc, and it gets its
// position absolutely using absHeight then does math assuming it's in the
// bottom left corner...
type CoverDynWDrawer struct {
	Queue *[]*track.Track

	// prevCover is the cover that was drawn last time. This is used to
	// determine whether we need to re-send the image.
	prevCover image.Image
	// prevX is the x value that was drawn up to when prevCover was drawn.
	prevX *int

	scope
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
	c.prevX = &c.scope.Min.X

	return nil
}

func (c *CoverDynWDrawer) dynWDraw(d drawFunc) (imageW int, err error) {
	clear(d, c.Rectangle)

	if len(*c.Queue) == 0 {
		if c.prevCover != nil {
			return c.Min.X, c.Clear()
		}

		return c.Min.X, nil
	}

	cover, err := (*c.Queue)[0].Cover()
	if err != nil {
		return 0, err
	}

	if cover == nil {
		return c.Min.X, c.Clear()
	}

	if cover != nil && cover == c.prevCover {
		return *c.prevX, nil
	}

	coverBounds := cover.Bounds()
	coverAspectRatio := float64(coverBounds.Dx()) / float64(coverBounds.Dy())
	cellAspectRatio := 2.28
	imageW = int(math.Round(float64(c.Dy()) * coverAspectRatio * cellAspectRatio))
	imageH := c.Dy()
	if imageW > c.Max.X {
		imageW = c.Max.X
		imageH = int(math.Round(float64(imageW) / coverAspectRatio / cellAspectRatio))
	}
	imageR := c.Rectangle
	imageR.Max = c.Min.Add(image.Pt(imageW, imageH))

	err = termimage.WriteImage(os.Stdout, cover, imageR)
	if err != nil {
		if errors.Is(err, termimage.ErrTerminalUnsupported) {
			return c.Min.X, nil
		}

		return 0, err
	}

	c.prevCover = cover
	c.prevX = &imageR.Max.X

	return imageR.Max.X, nil
}

var _ DynWDrawClearer = &CoverDynWDrawer{}
