package draw

// Drawer draws to the screen, taking up a fixed box.
type Drawer interface {
	// setScope sets the scope of this drawer to be the box from x=0..w and
	// y=0..h (both non-inclusive), which should be filled using d.
	setScope(d drawFunc, w, h int)

	// Draw fills the box specified by setScope. It must fill the entire box,
	// since no guarantees are made about the box's prior contents.
	Draw() error
}

type SetScoper interface {
	// setScope sets the scope of this drawer to be the box from x=0..w and
	// y=0..h (both non-inclusive), which should be filled using d.
	SetScope(d drawFunc, w, h int)
}

type Clearer interface {
	// Clear clears the previous draw.
	Clear() error
}

type DrawSetScoper interface {
	Drawer
	SetScoper
}

type DrawClearer interface {
	Drawer
	Clearer
}

// DynWDrawer draws to the screen, taking up a fixed height but variable width.
type DynWDrawer interface {
	// dynWDraw fills the box from x value 0..w where w <= maxW and y value
	// 0..h (both non-inclusive) by calling d. It is the caller's responsibility
	// to fill the range w..maxW.
	dynWDraw(d drawFunc, maxW, h int) (w int, err error)
}

// DynWDrawClearer draws to the screen, taking up a fixed height but variable
// width. It can also be cleared.
type DynWDrawClearer interface {
	DynWDrawer
	Clearer
}

var _ DynWDrawer = DynWDrawClearer(nil)
