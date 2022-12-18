package draw

type scope struct {
	d    drawFunc
	w, h int
}

func (s *scope) setScope(d drawFunc, w, h int) {
	s.d = d
	s.w = w
	s.h = h
}
