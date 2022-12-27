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

	t.infoMaxR = image.Rect(stopX+1, t.bottomR.Min.Y, t.bottomR.Max.X, t.bottomR.Max.Y-1)

	t.drawInfoAndProgress()

	return nil
}
