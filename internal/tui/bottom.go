package tui

import (
	"fmt"
	"image"
)

func (t *tui) drawBottom(new, old image.Image) error {
	stopX, err := t.drawCover(new, old)
	if err != nil {
		return fmt.Errorf("failed to draw cover: %w", err)
	}

	t.clear(image.Rect(stopX, t.bottomR.Min.Y, stopX+1, t.bottomR.Max.Y))

	infoMinX := stopX + 1
	t.infoMaxR = image.Rect(infoMinX, t.bottomR.Min.Y, infoMinX+((t.bottomR.Max.X-infoMinX)/3), t.bottomR.Max.Y-1)

	t.drawInfoAndProgress()

	return nil
}
