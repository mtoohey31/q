package draw

import "mtoohey.com/q/internal/types"

type SwitchableDrawer struct {
	Drawers []Drawer

	Tab types.Tab
	scope
}

func (s *SwitchableDrawer) Cycle() error {
	if len(s.Drawers) > 1 {
		if clearer, ok := s.Drawers[s.Tab].(Clearer); ok {
			if err := clearer.Clear(); err != nil {
				return err
			}
		}
	}
	s.Tab = types.Tab((int(s.Tab) + 1) % len(s.Drawers))
	return s.Draw()
}

func (s *SwitchableDrawer) Draw() error {
	s.Drawers[s.Tab].setScope(s.d, s.w, s.h)
	return s.Drawers[s.Tab].Draw()
}

func (s *SwitchableDrawer) DrawIfVisible(tab types.Tab) error {
	if s.Tab == tab {
		return s.Draw()
	}

	return nil
}

var _ Drawer = &SwitchableDrawer{}
