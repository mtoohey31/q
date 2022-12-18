package draw

type SwitchableDrawer struct {
	Drawers []Drawer

	idx int
	scope
}

func (s *SwitchableDrawer) Cycle() (int, error) {
	if len(s.Drawers) > 1 {
		if clearer, ok := s.Drawers[s.idx].(Clearer); ok {
			if err := clearer.Clear(); err != nil {
				return 0, err
			}
		}
	}
	s.idx = (s.idx + 1) % len(s.Drawers)
	return s.idx, s.Draw()
}

func (s *SwitchableDrawer) Draw() error {
	s.Drawers[s.idx].setScope(s.d, s.w, s.h)
	return s.Drawers[s.idx].Draw()
}

func (s *SwitchableDrawer) DrawIfVisible(idx int) error {
	if s.idx == idx {
		return s.Draw()
	}

	return nil
}

var _ Drawer = &SwitchableDrawer{}
