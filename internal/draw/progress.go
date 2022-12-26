package draw

import (
	"math"
	"strings"
	"time"

	"mtoohey.com/q/internal/types"

	"github.com/faiface/beep"
	"github.com/gdamore/tcell/v2"
)

type runeStylePair struct {
	r rune
	s tcell.Style
}

// progress draw assumes h is always 3
type ProgressDrawer struct {
	pausedRMap  map[bool]rune
	repeatRSMap map[types.Repeat]runeStylePair
	shuffleR    rune

	StreamSeekCloser *beep.StreamSeekCloser
	Format           *beep.Format
	Paused           *bool
	Repeat           *types.Repeat
	ShuffleIdx       **int
	Warning          *error

	cancelDrawers chan struct{}
	scope
}

func NewProgressDrawer(canDisplay func(rune) bool) *ProgressDrawer {
	p := ProgressDrawer{
		pausedRMap: map[bool]rune{false: '>', true: '>'},
		repeatRSMap: map[types.Repeat]runeStylePair{
			types.RepeatNone:  {'r', tcell.StyleDefault.Dim(true)},
			types.RepeatQueue: {'r', tcell.StyleDefault},
			types.RepeatTrack: {'r', tcell.StyleDefault.Underline(true)},
		},
		shuffleR: 's',
	}

	if canDisplay('') && canDisplay('契') {
		p.pausedRMap = map[bool]rune{false: '', true: '契'}
	}

	if canDisplay('稜') && canDisplay('凌') && canDisplay('綾') {
		p.repeatRSMap = map[types.Repeat]runeStylePair{
			types.RepeatNone:  {'稜', tcell.StyleDefault.Dim(true)},
			types.RepeatQueue: {'凌', tcell.StyleDefault},
			types.RepeatTrack: {'綾', tcell.StyleDefault},
		}
	}

	if canDisplay('列') {
		p.shuffleR = '列'
	}

	return &p
}

func (p *ProgressDrawer) DrawPause() {
	s := tcell.StyleDefault
	if *p.Paused || *p.StreamSeekCloser == nil {
		s = s.Dim(true)
	}
	p.scope.d((p.scope.w)/2, 0, p.pausedRMap[*p.Paused], s)
}

func (p *ProgressDrawer) DrawRepeat() {
	p.scope.d(p.scope.w/2+5, 0, p.repeatRSMap[*p.Repeat].r, p.repeatRSMap[*p.Repeat].s)
}

func (p *ProgressDrawer) DrawShuffle() {
	s := tcell.StyleDefault
	if *p.ShuffleIdx == nil {
		s = s.Dim(true)
	}
	p.scope.d(p.scope.w/2-5, 0, p.shuffleR, s)
}

func (p *ProgressDrawer) widths() (dW, barW int) {
	// the total duration of the current song
	totalD := (*p.Format).SampleRate.D((*p.StreamSeekCloser).Len()).Truncate(time.Second)

	dW = 2 // first seconds digit and 's'

	// the second seconds digit
	if totalD >= time.Second*10 {
		dW++
	}

	// the first minutes digit and 'm'
	if totalD >= time.Minute {
		dW += 2
	}

	// the second minutes digit
	if totalD >= time.Minute*10 {
		dW++
	}

	// the remaining hours digits and 'h'
	if totalD >= time.Hour {
		dW += int(math.Floor(math.Log10(float64(totalD/time.Hour)))) + 2
	}

	return dW, p.scope.w - (2*dW + 2)
}

func (p *ProgressDrawer) DrawBar() {
	if *p.StreamSeekCloser == nil {
		clear(offset(p.d, 0, 1), p.w, 1)

		return
	}

	currD := (*p.Format).SampleRate.D((*p.StreamSeekCloser).Position())
	currS := currD.Truncate(time.Second).String()
	totalD := (*p.Format).SampleRate.D((*p.StreamSeekCloser).Len())
	totalS := totalD.Truncate(time.Second).String()

	dW, barW := p.widths()
	currS = strings.Repeat(" ", dW-len(currS)) + currS
	totalS = strings.Repeat(" ", dW-len(totalS)) + totalS

	drawString(offset(p.scope.d, 0, 1), -1, currS, tcell.StyleDefault)
	p.scope.d(dW, 1, '|', tcell.StyleDefault)

	progress := float64(currD) / float64(totalD)
	barCompleteW := int(progress * float64(barW))
	drawString(offset(p.scope.d, dW+1, 1), -1, strings.Repeat("█", barCompleteW), tcell.StyleDefault)

	if barCompleteW > barW {
		barCompleteW = barW
	}

	if barCompleteW != barW {
		// the fractional part of a box not drawn, should be between 0 and 1
		remainder := progress*float64(barW) - float64(barCompleteW)
		partialBoxes := [8]rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}
		p.scope.d(dW+1+barCompleteW, 1, partialBoxes[int(remainder*8)], tcell.StyleDefault)

		drawString(offset(p.d, dW+1+barCompleteW+1, 1), -1,
			strings.Repeat(" ", barW-barCompleteW-1), tcell.StyleDefault)
	}

	p.d(p.w-dW-1, 1, '|', tcell.StyleDefault)
	drawString(offset(p.d, p.w-dW, 1), -1, totalS, tcell.StyleDefault)
}

func (p *ProgressDrawer) SpawnProgressDrawers(show func()) {
	if p.cancelDrawers != nil {
		// drawers already running
		return
	}

	p.cancelDrawers = make(chan struct{})

	// bar drawer
	go func() {
		for {
			p.DrawBar()
			show()

			// the time it takes to draw one fractional section of the bar
			_, barW := p.widths()
			redrawTime := (p.Format).SampleRate.D((*p.StreamSeekCloser).Len() / barW / 8)

			select {
			case <-time.After(redrawTime):
			case <-p.cancelDrawers:
				return
			}
		}
	}()

	// time drawer
	go func() {
		for {
			p.DrawBar()
			show()

			select {
			case <-time.After(time.Second):
			case <-p.cancelDrawers:
				return
			}
		}
	}()
}

func (p *ProgressDrawer) CancelProgressDrawers() {
	if p.cancelDrawers != nil {
		close(p.cancelDrawers)
		p.cancelDrawers = nil
	}
}

func (p *ProgressDrawer) DrawWarning() {
	clear(offset(p.d, 0, 2), p.w, 1)

	if *p.Warning == nil {
		return
	}

	warningString := (*p.Warning).Error()
	warningLen := len(warningString)

	style := tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack)

	if warningLen > p.w {
		drawString(offset(p.d, 0, 2), p.w, warningString, style)
		return
	}

	drawString(offset(p.d, p.w-warningLen, 2), -1, warningString, style)
}

func (p *ProgressDrawer) Draw() error {
	// pause/shuffle/repeat drawers don't clear the top row, so we need to do
	// it here
	clear(p.d, p.w, 1)
	p.DrawPause()
	p.DrawShuffle()
	p.DrawRepeat()

	p.DrawBar()
	p.DrawWarning()

	return nil
}

var _ Drawer = &ProgressDrawer{}
