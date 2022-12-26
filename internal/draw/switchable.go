package draw

import (
	"image"

	"mtoohey.com/q/internal/types"
)

type SwitchableDrawer struct {
	Drawers []Drawer

	Tab types.Tab
	scope
}

func (s *SwitchableDrawer) setScope(r image.Rectangle) {
	s.Rectangle = r
	for _, d := range s.Drawers {
		d.setScope(r)
	}
}

func (s *SwitchableDrawer) Cycle(d drawFunc) error {
	if len(s.Drawers) > 1 {
		if clearer, ok := s.Drawers[s.Tab].(Clearer); ok {
			if err := clearer.Clear(); err != nil {
				return err
			}
		}
	}
	s.Tab = types.Tab((int(s.Tab) + 1) % len(s.Drawers))
	return s.Draw(d)
}

func (s *SwitchableDrawer) Draw(d drawFunc) error {
	return s.Drawers[s.Tab].Draw(d)
}

func (s *SwitchableDrawer) DrawIfVisible(d drawFunc, tab types.Tab) error {
	if s.Tab == tab {
		return s.Draw(d)
	}

	return nil
}

var _ Drawer = &SwitchableDrawer{}
