package draw

import (
	"path/filepath"
	"strings"
	"unicode/utf8"

	"mtoohey.com/q/internal/query"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

type SearchDrawer struct {
	Screen   tcell.Screen
	MusicDir string

	query, prevQuery string
	queryIdx         int // index of cursor within query text

	resultsIdx int // 0 means typing, nothing focused
	results    []string

	scope
}

func (s *SearchDrawer) Clear() error {
	s.Screen.HideCursor()

	return nil
}

func (s *SearchDrawer) ShiftFocus(i int) error {
	newIdx := s.resultsIdx + i
	if newIdx < 0 {
		newIdx = 0
	}
	// values that are too large will be caught by draw since the length of the
	// results may have changed anyways
	s.resultsIdx = newIdx
	return s.Draw()
}

func (s *SearchDrawer) Draw() error {
	w, _ := s.Screen.Size()

	// TODO: this uses knowledge of its absolute position
	s.Screen.ShowCursor(w/3+1+runewidth.StringWidth(s.query[:s.queryIdx]), 0)
	s.Screen.SetCursorStyle(tcell.CursorStyleSteadyBar)

	x := drawString(s.d, s.w, s.query, tcell.StyleDefault.Underline(true))

	for ; x < w; x++ {
		s.d(x, 0, ' ', tcell.StyleDefault.Underline(true))
	}

	if s.query != s.prevQuery {
		var err error
		s.results, err = query.Query(s.MusicDir, s.query)
		if err != nil {
			return err
		}

		if s.resultsIdx > len(s.results) {
			s.resultsIdx = len(s.results)
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
		if y == s.resultsIdx {
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
	if s.resultsIdx == 0 {
		return ""
	}

	return s.results[s.resultsIdx-1]
}

func (s *SearchDrawer) Backspace() error {
	_, size := utf8.DecodeLastRuneInString(s.query)
	if size == 0 {
		return nil
	}

	s.query = s.query[:len(s.query)-size]
	s.queryIdx = len(s.query)
	return s.Draw()
}

func (s *SearchDrawer) Insert(r rune) error {
	s.query = strings.Join([]string{
		s.query[:s.queryIdx],
		string(r),
		s.query[s.queryIdx:],
	}, "")
	s.queryIdx += len(string(r))

	return s.Draw()
}

func (s *SearchDrawer) ToStart() error {
	if s.queryIdx == 0 {
		return nil
	}

	s.queryIdx = 0

	return s.Draw()
}

func (s *SearchDrawer) ToEnd() error {
	queryLen := len(s.query)
	if s.queryIdx == queryLen {
		return nil
	}

	s.queryIdx = queryLen

	return s.Draw()
}

// NOTE: Left and Right assume that the utf8 package won't return something
// nonsensical that makes us jump the cursor out of range. This should be
// safe... hopefully...

func (s *SearchDrawer) Left() error {
	if s.queryIdx == 0 {
		return nil
	}

	_, size := utf8.DecodeLastRuneInString(s.query[:s.queryIdx])
	s.queryIdx -= size

	return s.Draw()
}

func (s *SearchDrawer) Right() error {
	if s.queryIdx == len(s.query) {
		return nil
	}

	_, size := utf8.DecodeRuneInString(s.query[s.queryIdx:])
	s.queryIdx += size

	return s.Draw()
}

func (s *SearchDrawer) KillWord() error {
	if s.query == "" {
		return nil
	}

	searchIdx := s.queryIdx
	if s.queryIdx > 0 && s.query[s.queryIdx-1] == ' ' {
		searchIdx--
	}

	// If LastIndexByte returns -1, that's fine because we want to remove
	// everything until the start then
	idx := strings.LastIndexByte(s.query[:searchIdx], ' ') + 1
	s.query = s.query[:idx] + s.query[s.queryIdx:]
	s.queryIdx = idx

	_, size := utf8.DecodeRuneInString(s.query[s.queryIdx:])
	s.queryIdx += size

	return s.Draw()
}

var _ DrawClearer = &SearchDrawer{}
