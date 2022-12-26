package draw

import (
	"image"
	"math"
	"strings"
	"time"

	"mtoohey.com/q/internal/types"

	"github.com/faiface/beep"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
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

func (p *ProgressDrawer) DrawPause(d drawFunc) {
	s := tcell.StyleDefault
	if *p.Paused || *p.StreamSeekCloser == nil {
		s = s.Dim(true)
	}
	d(p.Min.Add(image.Pt(p.Dx()/2, 0)), p.pausedRMap[*p.Paused], s)
}

func (p *ProgressDrawer) DrawRepeat(d drawFunc) {
	d(p.Min.Add(image.Pt(p.Dx()/2+5, 0)), p.repeatRSMap[*p.Repeat].r, p.repeatRSMap[*p.Repeat].s)
}

func (p *ProgressDrawer) DrawShuffle(d drawFunc) {
	s := tcell.StyleDefault
	if *p.ShuffleIdx == nil {
		s = s.Dim(true)
	}
	d(p.Min.Add(image.Pt(p.Dx()/2-5, 0)), p.shuffleR, s)
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

	return dW, p.Dx() - (2*dW + 2)
}

func (p *ProgressDrawer) DrawBar(d drawFunc) {
	if *p.StreamSeekCloser == nil {
		clear(d, image.Rect(p.Min.X, p.Min.Y+1, p.Max.X, p.Min.Y+2))

		return
	}

	currD := (*p.Format).SampleRate.D((*p.StreamSeekCloser).Position())
	currS := currD.Truncate(time.Second).String()
	totalD := (*p.Format).SampleRate.D((*p.StreamSeekCloser).Len())
	totalS := totalD.Truncate(time.Second).String()

	dW, barW := p.widths()
	currS = strings.Repeat(" ", dW-len(currS)) + currS
	totalS = strings.Repeat(" ", dW-len(totalS)) + totalS

	drawString(d, p.Min.Add(image.Pt(0, 1)), p.Max.X, currS, tcell.StyleDefault)
	d(p.Min.Add(image.Pt(dW, 1)), '|', tcell.StyleDefault)

	progress := float64(currD) / float64(totalD)
	barCompleteW := int(progress * float64(barW))
	drawString(d, p.Min.Add(image.Pt(dW+1, 1)), p.Max.X, strings.Repeat("█", barCompleteW), tcell.StyleDefault)

	if barCompleteW > barW {
		barCompleteW = barW
	}

	if barCompleteW != barW {
		// the fractional part of a box not drawn, should be between 0 and 1
		remainder := progress*float64(barW) - float64(barCompleteW)
		partialBoxes := [8]rune{' ', '▏', '▎', '▍', '▌', '▋', '▊', '▉'}
		d(image.Pt(p.Min.X+dW+1+barCompleteW, p.Min.Y+1), partialBoxes[int(remainder*8)], tcell.StyleDefault)

		drawString(d, p.Min.Add(image.Pt(dW+1+barCompleteW+1, 1)), p.Max.X,
			strings.Repeat(" ", barW-barCompleteW-1), tcell.StyleDefault)
	}

	d(image.Pt(p.Max.X-dW-1, p.Min.Y+1), '|', tcell.StyleDefault)
	drawString(d, image.Pt(p.Max.X-dW, p.Min.Y+1), p.Max.X, totalS, tcell.StyleDefault)
}

func (p *ProgressDrawer) SpawnProgressDrawers(d drawFunc, show func()) {
	if p.cancelDrawers != nil {
		// drawers already running
		return
	}

	p.cancelDrawers = make(chan struct{})

	// bar drawer
	go func() {
		for {
			p.DrawBar(d)
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
			p.DrawBar(d)
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

func (p *ProgressDrawer) DrawWarning(d drawFunc) {
	clear(d, image.Rectangle{
		Min: image.Point{
			X: p.Min.X,
			Y: p.Min.Y + 2,
		},
		Max: image.Point{
			X: p.Max.X,
			Y: p.Max.Y,
		},
	})

	if *p.Warning == nil {
		return
	}

	warningString := (*p.Warning).Error()
	warningLen := runewidth.StringWidth(warningString)

	style := tcell.StyleDefault.Background(tcell.ColorRed).Foreground(tcell.ColorBlack)

	if warningLen > p.Dx() {
		drawString(d, p.Min.Add(image.Pt(0, 2)), p.Max.X, warningString, style)
		return
	}

	drawString(d, p.Min.Add(image.Pt(p.Dx()-warningLen, 2)), -1, warningString, style)
}

func (p *ProgressDrawer) Draw(d drawFunc) error {
	// pause/shuffle/repeat drawers don't clear the top row, so we need to do
	// it here
	clear(d, image.Rectangle{
		Min: p.Min,
		Max: image.Point{
			X: p.Max.X,
			Y: p.Min.Y + 1,
		},
	})
	p.DrawPause(d)
	p.DrawShuffle(d)
	p.DrawRepeat(d)

	p.DrawBar(d)
	p.DrawWarning(d)

	return nil
}

var _ Drawer = &ProgressDrawer{}
