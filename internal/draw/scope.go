package draw

import "image"

type scope struct {
	image.Rectangle
}

func (s *scope) setScope(r image.Rectangle) {
	s.Rectangle = r
}
