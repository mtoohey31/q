package draw

// FillDrawer fills its whole box with r.
type FillDrawer struct {
	R rune

	scope
}

func (f *FillDrawer) Draw() error {
	fill(f.d, f.w, f.h, f.R)
	return nil
}

func (f *FillDrawer) dynWDraw(d drawFunc, maxW, h int) (w int, err error) {
	fill(d, maxW, h, f.R)
	return maxW, nil
}

var _ Drawer = &FillDrawer{}
var _ DynWDrawer = &FillDrawer{}
