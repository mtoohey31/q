package draw

// VertFixedBotSplitDrawer draws top and bottom respectively above and below
// each other, providing bottom with a height of bottomH and top with the
// remaining vertical space. It draws no lines.
type VertFixedBotSplitDrawer struct {
	BottomH     int
	Top, Bottom Drawer

	scope
}

func (vfbs *VertFixedBotSplitDrawer) SetScope(d drawFunc, w, h int) {
	vfbs.setScope(d, w, h)
}

func (vfbs *VertFixedBotSplitDrawer) Draw() error {
	topH := vfbs.h - vfbs.BottomH
	vfbs.Top.setScope(vfbs.d, vfbs.w, topH)
	if err := vfbs.Top.Draw(); err != nil {
		return err
	}

	vfbs.Bottom.setScope(offset(vfbs.d, 0, topH), vfbs.w, vfbs.BottomH)
	return vfbs.Bottom.Draw()
}

var _ Drawer = &VertFixedBotSplitDrawer{}
var _ DrawSetScoper = &VertFixedBotSplitDrawer{}
