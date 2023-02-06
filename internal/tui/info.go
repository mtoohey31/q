package tui

import (
	"fmt"
	"image"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/util"

	"github.com/gdamore/tcell/v2"
)

func (t *tui) drawInfoAndProgress() {
	stopX := t.infoMaxR.Min.X
	var zeroNowPlaying protocol.NowPlayingState
	if t.NowPlaying != zeroNowPlaying {
		tX := t.drawString(t.infoMaxR.Min, t.infoMaxR.Max.X, t.NowPlaying.Title, styleDefault)
		aX := t.drawString(t.infoMaxR.Min.Add(image.Pt(0, 1)), t.infoMaxR.Max.X, t.NowPlaying.Artist, styleDim)

		// the first argument ensures that we take up at least 5 cells so that
		// the mode can be drawn properly
		stopX = util.Max(t.infoMaxR.Min.X+5, tX, aX)

		t.clear(image.Rect(tX, t.infoMaxR.Min.Y, t.infoMaxR.Max.X, t.infoMaxR.Min.Y+1))
		t.clear(image.Rect(aX, t.infoMaxR.Min.Y+1, t.infoMaxR.Max.X, t.infoMaxR.Min.Y+2))
		t.clear(image.Rect(t.infoMaxR.Min.X, t.infoMaxR.Min.Y+2, t.infoMaxR.Max.X, t.infoMaxR.Max.Y))
	}

	t.modeR = image.Rect(t.infoMaxR.Min.X, t.bottomR.Max.Y-1, stopX, t.bottomR.Max.Y)
	t.drawMode()

	t.progressR = image.Rect(stopX+1, t.bottomR.Min.Y, t.bottomR.Max.X, t.bottomR.Max.Y)

	lineR := image.Rect(t.progressR.Min.X, t.progressR.Min.Y, t.progressR.Max.X, t.progressR.Min.Y+1)
	// clear top row, since point draws won't
	t.clear(lineR)
	t.drawShuffle()
	t.drawPause()
	t.drawRepeat()

	t.barR = lineR.Add(image.Pt(0, 1))
	t.drawBar()

	t.errorR = t.barR.Add(image.Pt(0, 1))
	t.drawError()
}

type mode uint8

const (
	modeNormal mode = iota
	modeInsert
	modeSelect
)

func (m mode) String() string {
	switch m {
	case modeNormal:
		return " NOR "
	case modeInsert:
		return " INS "
	case modeSelect:
		return " SEL "
	default:
		panic(fmt.Sprintf(`invalid mode "%d"`, m))
	}
}

func (m mode) Style() tcell.Style {
	var bg tcell.Color
	switch m {
	case modeNormal:
		bg = tcell.ColorAqua
	case modeInsert:
		bg = tcell.ColorGreen
	case modeSelect:
		bg = tcell.ColorYellow
	default:
		panic(fmt.Sprintf(`invalid mode "%d"`, m))
	}
	return styleDefault.Background(bg).Foreground(tcell.ColorBlack)
}

func (t *tui) drawMode() {
	stopX := t.drawString(t.modeR.Min, t.modeR.Max.X, t.mode.String(), t.mode.Style())
	t.clear(image.Rect(stopX, t.modeR.Min.Y, t.modeR.Max.X, t.modeR.Max.Y))
}
