package draw

import "github.com/gdamore/tcell/v2"

// HorizDynLimitRatioSplitDrawer first draws left will maxW set to w*ratio,
// then draws right in whatever the remaining space is.
type HorizDynLimitRatioSplitDrawer struct {
	Ratio float32
	Left  DynWDrawer
	Right Drawer

	lastRightX *int
	scope
}

func (hdlrs *HorizDynLimitRatioSplitDrawer) Draw() error {
	leftMaxW := int(float32(hdlrs.w) * hdlrs.Ratio)
	leftW, err := hdlrs.Left.dynWDraw(hdlrs.d, leftMaxW, hdlrs.h)
	if err != nil {
		return err
	}

	for y := 0; y < hdlrs.h; y++ {
		hdlrs.d(leftW, y, ' ', tcell.StyleDefault)
	}

	rightX := leftW + 1
	hdlrs.lastRightX = &rightX
	hdlrs.Right.setScope(offset(hdlrs.d, rightX, 0), hdlrs.w-rightX, hdlrs.h)
	return hdlrs.Right.Draw()
}

var _ Drawer = &HorizDynLimitRatioSplitDrawer{}
