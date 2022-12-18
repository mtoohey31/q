package draw

import (
	"bytes"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"mtoohey.com/q/internal/query"
)

type SearchDrawer struct {
	Screen   tcell.Screen
	MusicDir string

	idx int // 0 means typing, nothing focused
	bytes.Buffer
	results []string
	prevQ   string
	scope
}

func (s *SearchDrawer) Clear() error {
	s.Screen.HideCursor()

	return nil
}

func (s *SearchDrawer) ShiftFocus(i int) error {
	newIdx := s.idx + i
	if newIdx < 0 {
		newIdx = 0
	}
	// values that are too large will be caught by draw since the length of the
	// results may have changed anyways
	s.idx = newIdx
	return s.Draw()
}

func (s *SearchDrawer) Draw() error {
	w, _ := s.Screen.Size()

	s.Screen.ShowCursor(w/3+1+len(s.String()), 0)
	s.Screen.SetCursorStyle(tcell.CursorStyleSteadyBar)

	x := drawString(s.d, s.w, s.String(), tcell.StyleDefault.Underline(true))
	for ; x < w; x++ {
		s.d(x, 0, ' ', tcell.StyleDefault.Underline(true))
	}

	if s.String() != s.prevQ {
		var err error
		s.results, err = query.Query(s.MusicDir, s.String())
		if err != nil {
			return err
		}

		if s.idx > len(s.results) {
			s.idx = len(s.results)
		}
	}

	y := 1
	for _, line := range s.results {
		if y >= s.h {
			break
		}

		relLine, err := filepath.Rel(s.MusicDir, line)
		if err == nil {
			line = relLine
		}

		style := tcell.StyleDefault
		if y == s.idx {
			style = style.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
		}
		s.d(0, y, ' ', style)
		x := drawString(offset(s.d, 1, y), s.w-2, line, style)
		for x++; x < s.w; x++ {
			s.d(x, y, ' ', style)
		}
		y++
	}

	if y < s.h {
		clear(offset(s.d, 0, y), s.w, s.h-y)
	}

	return nil
}

func (s *SearchDrawer) FocusedResult() string {
	if s.idx == 0 {
		return ""
	}

	return s.results[s.idx-1]
}

var _ DrawClearer = &SearchDrawer{}
