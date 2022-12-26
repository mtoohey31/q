package draw

import "image"

// VertFixedBotSplitDrawer draws top and bottom respectively above and below
// each other, providing bottom with a height of bottomH and top with the
// remaining vertical space. It draws no lines.
type VertFixedBotSplitDrawer struct {
	BottomH     int
	Top, Bottom Drawer

	scope
}

func (vfbs *VertFixedBotSplitDrawer) SetScope(r image.Rectangle) {
	vfbs.setScope(r)
}

func (vfbs *VertFixedBotSplitDrawer) setScope(r image.Rectangle) {
	vfbs.Rectangle = r

	vfbs.Top.setScope(image.Rectangle{
		Min: r.Min,
		Max: r.Max.Sub(image.Pt(0, vfbs.BottomH)),
	})
	vfbs.Bottom.setScope(image.Rectangle{
		Min: image.Point{
			X: r.Min.X,
			Y: r.Max.Y - vfbs.BottomH,
		},
		Max: r.Max,
	})
}

func (vfbs *VertFixedBotSplitDrawer) Draw(d drawFunc) error {
	if err := vfbs.Top.Draw(d); err != nil {
		return err
	}
	return vfbs.Bottom.Draw(d)
}

var _ DrawSetScoper = &VertFixedBotSplitDrawer{}
