package draw

import (
	"image"

	"github.com/gdamore/tcell/v2"
)

// HorizRatioSplitDrawer draws left and right next to each other, along with
// lines between the two, and a line at the bottom.
//
// The bottom line is this type's responsibility because it has to be drawn
// with knowledge of where the horizontal split is since there is a joining '┴'
// in the bottom line.
type HorizRatioSplitDrawer struct {
	Ratio       float64
	Left, Right Drawer

	scope
}

func (hrs *HorizRatioSplitDrawer) setScope(r image.Rectangle) {
	hrs.Rectangle = r

	leftR := hrs.Rectangle
	leftR.Max.X = hrs.Min.X + int(float64(hrs.Dx())*hrs.Ratio)
	leftR.Max.Y-- // account for bottom line
	hrs.Left.setScope(leftR)

	rightR := hrs.Rectangle
	rightR.Min.X = leftR.Max.X + 1 // account for middle line
	rightR.Max.Y--                 // account for bottom line
	hrs.Right.setScope(rightR)
}

func (hrs *HorizRatioSplitDrawer) Draw(d drawFunc) error {
	if err := hrs.Left.Draw(d); err != nil {
		return err
	}

	// draw lines
	lineX := hrs.Min.X + int(float64(hrs.Dx())*hrs.Ratio)
	for c := image.Pt(lineX, hrs.Min.Y); c.Y < hrs.Max.Y-1; c.Y++ {
		d(c, '│', tcell.StyleDefault)
	}
	for c := image.Pt(hrs.Min.X, hrs.Max.Y-1); c.X < hrs.Max.X; c.X++ {
		d(c, '─', tcell.StyleDefault)
	}
	d(image.Pt(lineX, hrs.Max.Y-1), '┴', tcell.StyleDefault)

	return hrs.Right.Draw(d)
}

var _ Drawer = &HorizDynLimitRatioSplitDrawer{}
