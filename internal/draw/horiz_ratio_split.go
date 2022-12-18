package draw

import "github.com/gdamore/tcell/v2"

// HorizRatioSplitDrawer draws left and right next to each other, along with
// lines between the two, and a line at the bottom.
//
// The bottom line is this type's responsibility because it has to be drawn
// with knowledge of where the horizontal split is since there is a joining '┴'
// in the bottom line.
type HorizRatioSplitDrawer struct {
	Ratio       float32
	Left, Right Drawer

	scope
}

func (hrs *HorizRatioSplitDrawer) Draw() error {
	leftW := int(float32(hrs.w) * hrs.Ratio)
	// h-1 leaves space for the horizontal line
	hrs.Left.setScope(hrs.d, leftW, hrs.h)
	if err := hrs.Left.Draw(); err != nil {
		return err
	}

	// draw gap
	for y := 0; y < hrs.h; y++ {
		hrs.d(leftW, y, ' ', tcell.StyleDefault)
	}

	rightX := leftW + 1 // leave space for vertical line
	rightW := hrs.w - rightX
	hrs.Right.setScope(offset(hrs.d, rightX, 0), rightW, hrs.h)
	return hrs.Right.Draw()
}

var _ Drawer = &HorizDynLimitRatioSplitDrawer{}
