package draw

import (
	"image"
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

	ResultsIdx int // 0 means typing, nothing focused
	results    []string

	scope
}

func (s *SearchDrawer) Clear() error {
	s.Screen.HideCursor()

	return nil
}

func (s *SearchDrawer) ShiftFocus(d drawFunc, i int) error {
	newIdx := s.ResultsIdx + i
	if newIdx < 0 {
		newIdx = 0
	}
	// values that are too large will be caught by draw since the length of the
	// results may have changed anyways
	s.ResultsIdx = newIdx
	return s.Draw(d)
}

func (s *SearchDrawer) Draw(d drawFunc) error {
	w, _ := s.Screen.Size()

	s.Screen.ShowCursor(s.Min.X+runewidth.StringWidth(s.query[:s.queryIdx]), s.Min.Y)
	s.Screen.SetCursorStyle(tcell.CursorStyleSteadyBar)

	x := drawString(d, s.Min, s.Max.X, s.query, tcell.StyleDefault.Underline(true))

	for ; x < w; x++ {
		d(image.Pt(x, s.Min.Y), ' ', tcell.StyleDefault.Underline(true))
	}

	if s.query != s.prevQuery || s.results == nil {
		var err error
		s.results, err = query.Query(s.MusicDir, s.query)
		if err != nil {
			return err
		}
		s.prevQuery = s.query

		if s.ResultsIdx > len(s.results) {
			s.ResultsIdx = len(s.results)
		}
	}

	y := 1
	for _, line := range s.results {
		if y >= s.Dy() {
			break
		}

		relLine, err := filepath.Rel(s.MusicDir, line)
		if err == nil {
			line = relLine
		}

		style := tcell.StyleDefault
		if y == s.ResultsIdx {
			style = style.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
		}
		d(s.Min.Add(image.Pt(0, y)), ' ', style)
		x := drawString(d, s.Min.Add(image.Pt(1, y)), s.Max.X-1, line, style)
		for ; x < s.Max.X; x++ {
			d(image.Pt(x, y), ' ', style)
		}
		y++
	}
	clear(d, image.Rectangle{
		Min: image.Point{
			X: s.Min.X,
			Y: s.Min.Y + y,
		},
		Max: s.Max,
	})

	return nil
}

func (s *SearchDrawer) FocusedResult() string {
	if s.ResultsIdx == 0 {
		return ""
	}

	return s.results[s.ResultsIdx-1]
}

func (s *SearchDrawer) Backspace(d drawFunc) error {
	_, size := utf8.DecodeLastRuneInString(s.query)
	if size == 0 {
		return nil
	}

	s.query = s.query[:len(s.query)-size]
	s.queryIdx = len(s.query)
	return s.Draw(d)
}

func (s *SearchDrawer) Insert(d drawFunc, r rune) error {
	s.query = strings.Join([]string{
		s.query[:s.queryIdx],
		string(r),
		s.query[s.queryIdx:],
	}, "")
	s.queryIdx += len(string(r))

	return s.Draw(d)
}

func (s *SearchDrawer) ToStart(d drawFunc) error {
	if s.queryIdx == 0 {
		return nil
	}

	s.queryIdx = 0

	return s.Draw(d)
}

func (s *SearchDrawer) ToEnd(d drawFunc) error {
	queryLen := len(s.query)
	if s.queryIdx == queryLen {
		return nil
	}

	s.queryIdx = queryLen

	return s.Draw(d)
}

// NOTE: Left and Right assume that the utf8 package won't return something
// nonsensical that makes us jump the cursor out of range. This should be
// safe... hopefully...

func (s *SearchDrawer) Left(d drawFunc) error {
	if s.queryIdx == 0 {
		return nil
	}

	_, size := utf8.DecodeLastRuneInString(s.query[:s.queryIdx])
	s.queryIdx -= size

	return s.Draw(d)
}

func (s *SearchDrawer) Right(d drawFunc) error {
	if s.queryIdx == len(s.query) {
		return nil
	}

	_, size := utf8.DecodeRuneInString(s.query[s.queryIdx:])
	s.queryIdx += size

	return s.Draw(d)
}

func (s *SearchDrawer) KillWord(d drawFunc) error {
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

	return s.Draw(d)
}

var _ DrawClearer = &SearchDrawer{}
