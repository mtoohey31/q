package tui

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"

	"mtoohey.com/q/internal/tui/termimage"
)

func (t *tui) drawCover(new, old image.Image) (stopX int, err error) {
	t.clear(t.coverMaxR)

	if new != old {
		if err := termimage.ClearImages(os.Stdout); err != nil &&
			!errors.Is(err, termimage.ErrTerminalUnsupported) {
			return t.coverMaxR.Min.X, fmt.Errorf("failed to clear images: %w", err)
		}
	}

	if new == nil {
		return t.coverMaxR.Min.X, nil
	}

	coverBounds := new.Bounds()
	coverAspectRatio := float64(coverBounds.Dx()) / float64(coverBounds.Dy())
	cellAspectRatio := 2.28
	stopX = int(math.Round(float64(t.coverMaxR.Dy()) * coverAspectRatio * cellAspectRatio))
	imageH := t.coverMaxR.Dy()
	if stopX > t.coverMaxR.Max.X {
		stopX = t.coverMaxR.Max.X
		imageH = int(math.Round(float64(stopX) / coverAspectRatio / cellAspectRatio))
	}
	imageR := t.coverMaxR
	imageR.Max = t.coverMaxR.Min.Add(image.Pt(stopX, imageH))

	err = termimage.WriteImage(os.Stdout, new, imageR)
	if err != nil {
		if errors.Is(err, termimage.ErrTerminalUnsupported) {
			return t.coverMaxR.Min.X, nil
		}

		return 0, fmt.Errorf("failed to write image: %w", err)
	}

	return imageR.Max.X, nil
}
