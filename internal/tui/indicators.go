package tui

import (
	"image"

	"mtoohey.com/q/internal/protocol"

	"github.com/gdamore/tcell/v2"
)

type runeStylePair struct {
	r rune
	s tcell.Style
}

func (t *tui) initIndicatorRunes() {
	t.shuffleRune = 's'
	if t.screen.CanDisplay('󰒝', false) {
		t.shuffleRune = '󰒝'
	}

	t.pauseRuneMap = map[protocol.PauseState]rune{false: '>', true: '>'}
	if t.screen.CanDisplay('󰏤', false) && t.screen.CanDisplay('󰐊', false) {
		t.pauseRuneMap = map[protocol.PauseState]rune{false: '󰏤', true: '󰐊'}
	}

	t.repeatRuneStyleMap = map[protocol.RepeatState]runeStylePair{
		protocol.RepeatStateNone:  {'r', styleDim},
		protocol.RepeatStateQueue: {'r', styleDefault},
		protocol.RepeatStateTrack: {'r', styleUnderline},
	}
	if t.screen.CanDisplay('󰑗', false) && t.screen.CanDisplay('󰑖', false) &&
		t.screen.CanDisplay('󰑘', false) {

		t.repeatRuneStyleMap = map[protocol.RepeatState]runeStylePair{
			protocol.RepeatStateNone:  {'󰑗', styleDim},
			protocol.RepeatStateQueue: {'󰑖', styleDefault},
			protocol.RepeatStateTrack: {'󰑘', styleDefault},
		}
	}
}

func (t *tui) drawShuffle() {
	style := styleDim
	if t.Shuffle {
		style = styleDefault
	}

	t.draw(t.progressR.Min.Add(image.Pt(t.progressR.Dx()/2-5, 0)), t.shuffleRune, style)
}

func (t *tui) drawPause() {
	style := styleDefault
	if t.Pause {
		style = styleDim
	}

	t.draw(t.progressR.Min.Add(image.Pt(t.progressR.Dx()/2, 0)), t.pauseRuneMap[t.Pause], style)
}

func (t *tui) drawRepeat() {
	pair := t.repeatRuneStyleMap[t.Repeat]
	t.draw(t.progressR.Min.Add(image.Pt(t.progressR.Dx()/2+5, 0)), pair.r, pair.s)
}
