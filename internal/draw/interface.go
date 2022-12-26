package draw

import (
	"image"

	"github.com/gdamore/tcell/v2"
)

type drawFunc func(p image.Point, r rune, s tcell.Style)

// Drawer draws to the screen, taking up a fixed rectangle.
type Drawer interface {
	// setScope sets area that this drawer should draw to.
	setScope(image.Rectangle)

	// Draw fills the rectangle specified by setScope. It must fill the entire
	// rectangle, since no guarantees are made about the prior contents.
	Draw(d drawFunc) error
}

// SetScope allows a scope to be set.
type SetScoper interface {
	// SetScope sets area that this drawer should draw to.
	SetScope(image.Rectangle)
}

// Clearer can clear the area previously drawn to.
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
	// setScope sets area that this drawer can draw to.
	setScope(image.Rectangle)

	// dynWDraw fills at most the rectangle specified by setScope. It returns
	// stopX, the x value before which it filled all columns within the
	// rectangle.
	dynWDraw(d drawFunc) (stopX int, err error)
}

type DynWDrawClearer interface {
	DynWDrawer
	Clearer
}
