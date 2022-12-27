package tui

import (
	"image"
	"path/filepath"

	"mtoohey.com/q/internal/util"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

func (t *tui) drawQuery() {
	if t.mode == modeInsert {
		t.screen.ShowCursor(t.queryR.Min.X+1+runewidth.StringWidth(t.queryString[:t.queryMouseIdx]), t.queryR.Min.Y)
		t.screen.SetCursorStyle(tcell.CursorStyleSteadyBar)
	} else {
		t.screen.HideCursor()
	}

	t.draw(t.queryR.Min, ' ', styleDefault)
	stopX := t.drawString(t.queryR.Min.Add(image.Pt(1, 0)), t.queryR.Max.X-1, t.queryString, styleUnderline)
	for c := image.Pt(stopX, t.queryR.Min.Y); c.X < t.queryR.Max.X-1; c.X++ {
		t.draw(c, ' ', styleUnderline)
	}
	t.draw(image.Pt(t.queryR.Max.X-1, t.queryR.Min.Y), ' ', styleDefault)

	if len(t.queryResults) == 0 {
		t.centeredString(image.Rect(t.queryR.Min.X, t.queryR.Min.Y+1, t.queryR.Max.X, t.queryR.Max.Y), "no results")
		return
	}

	// make sure scrolloff is no greater than half the current height
	t.ScrollOff = util.Min(t.ScrollOff, t.queryR.Dy()/2)

	distFromTop := t.queryFocusIdx - t.queryScrollIdx
	if distFromTop <= t.ScrollOff {
		t.queryScrollIdx = util.Max(0, t.queryScrollIdx-(t.ScrollOff-distFromTop))
	}

	distFromBottom := t.queryR.Dy() - 1 - distFromTop
	if distFromBottom <= t.ScrollOff {
		t.queryScrollIdx += 1 + t.ScrollOff - distFromBottom
	}

	i := 0
	for ; i < t.queryR.Dy()-1 && i+t.queryScrollIdx < len(t.queryResults); i++ {
		style := styleDefault
		if i+t.queryScrollIdx == t.queryFocusIdx && t.mode == modeSelect {
			style = style.Background(tcell.ColorYellow).Foreground(tcell.ColorBlack)
		}
		t.draw(t.queryR.Min.Add(image.Pt(0, i+1)), ' ', style)
		path := string(t.queryResults[i+t.queryScrollIdx])
		if filepath.HasPrefix(path, t.MusicDir) {
			newPath, err := filepath.Rel(t.MusicDir, path)
			if err == nil {
				path = newPath
			}
		}
		x := t.drawString(t.queryR.Min.Add(image.Pt(1, i+1)), t.queryR.Max.X-1, path, style)
		for ; x < t.queryR.Max.X; x++ {
			t.draw(image.Pt(x, i+1), ' ', style)
		}
	}
	t.clear(image.Rect(t.queryR.Min.X, t.queryR.Min.Y+i+1, t.queryR.Max.X, t.queryR.Max.Y))
}

func (t *tui) queryFocus(to int) {
	t.queryFocusIdx = util.Clamp(0, to, len(t.queryResults)-1)
	t.drawQuery()
}

func (t *tui) queryShiftFocus(by int) {
	t.queryFocus(t.queryFocusIdx + by)
}
