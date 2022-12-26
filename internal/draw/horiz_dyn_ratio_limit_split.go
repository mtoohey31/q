package draw

import (
	"image"

	"github.com/gdamore/tcell/v2"
)

// HorizDynLimitRatioSplitDrawer first draws left will maxW set to w*ratio,
// then draws right in whatever the remaining space is.
type HorizDynLimitRatioSplitDrawer struct {
	Ratio float64
	Left  DynWDrawer
	Right Drawer

	lastRightX *int
	scope
}

func (hdlrs *HorizDynLimitRatioSplitDrawer) setScope(r image.Rectangle) {
	hdlrs.Rectangle = r

	leftR := r
	leftR.Max.X = r.Min.X + int(float64(r.Dx())*hdlrs.Ratio)
	hdlrs.Left.setScope(leftR)

	rightR := r
	// probably incorrect fallback, assuming everything will be filled
	rightR.Min.X = leftR.Max.X + 1
	if hdlrs.lastRightX != nil {
		rightR.Min.X = *hdlrs.lastRightX
	}
	hdlrs.Right.setScope(rightR)
}

func (hdlrs *HorizDynLimitRatioSplitDrawer) Draw(d drawFunc) error {
	leftStopX, err := hdlrs.Left.dynWDraw(d)
	if err != nil {
		return err
	}

	for y := hdlrs.Min.Y; y < hdlrs.Max.Y; y++ {
		d(image.Pt(leftStopX, y), ' ', tcell.StyleDefault)
	}

	rightX := leftStopX + 1
	if hdlrs.lastRightX == nil || *hdlrs.lastRightX != rightX {
		hdlrs.lastRightX = &rightX

		// reset the right scope
		rightR := hdlrs.Rectangle
		rightR.Min.X = rightX
		hdlrs.Right.setScope(rightR)
	}
	return hdlrs.Right.Draw(d)
}

var _ Drawer = &HorizDynLimitRatioSplitDrawer{}
