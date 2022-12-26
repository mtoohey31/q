package draw

// FillDrawer fills its whole box with r. It is useful for debugging.
type FillDrawer struct {
	R rune

	scope
}

func (f *FillDrawer) Draw(d drawFunc) error {
	fill(d, f.Rectangle, f.R)
	return nil
}

func (f *FillDrawer) dynWDraw(d drawFunc) (x int, err error) {
	return f.Max.X, f.Draw(d)
}

var _ Drawer = &FillDrawer{}
var _ DynWDrawer = &FillDrawer{}
