package main

import (
	"errors"
	"fmt"
	"image"
	"math"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"mtoohey.com/q/internal/termimage"
	"mtoohey.com/q/internal/track"

	"github.com/faiface/beep"
	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
)

// TODO: figure out a better system then caching previous values of drawers and
// heights and widths cause that's ugly

type drawFunc func(x, y int, r rune, s tcell.Style)

// offset returns a new drawFunc final based on prev that is offset by xOff and
// yOff.
func offset(prev drawFunc, xOff, yOff int) (final drawFunc) {
	return func(x, y int, r rune, s tcell.Style) {
		prev(x+xOff, y+yOff, r, s)
	}
}

// fill uses d to fill from x 0..w to y 0..h (both non-inclusive).
func fill(d drawFunc, w, h int, r rune) {
	for x := 0; x < w; x++ {
		for y := 0; y < h; y++ {
			d(x, y, r, tcell.StyleDefault)
		}
	}
}

// clear uses d to clear from x 0..w to y 0..h (both non-inclusive).
func clear(d drawFunc, w, h int) {
	fill(d, w, h, ' ')
}

func drawString(d drawFunc, maxW int, s string, style tcell.Style) int {
	x := 0
	for r, rl := utf8.DecodeRuneInString(s); len(s) > 0; r, rl = utf8.DecodeRuneInString(s) {
		w := runewidth.RuneWidth(r)
		if maxW >= 0 {
			if x+w >= maxW && !(x+w == maxW && len(s) == rl) {
				for ; x < maxW; x++ {
					d(x, 0, '…', style)
				}
				return x
			}
		}
		d(x, 0, r, style)

		x += w
		s = s[rl:]
	}
	return x
}

// drawer draws to the screen, taking up a fixed box.
type drawer interface {
	// draw fills the box from x value 0..w and y value 0..h
	// (both non-inclusive) by calling d. It must fill the entire box, the
	// caller makes no guarantees about the prior contents.
	draw(d drawFunc, w, h int) error
}

// horizRatioSplitDrawer draws left and right next to each other, along with
// lines between the two, and a line at the bottom.
//
// The bottom line is this type's responsibility because it has to be drawn
// with knowledge of where the horizontal split is since there is a joining '┴'
// in the bottom line.
type horizRatioSplitDrawer struct {
	ratio       float32
	left, right drawer
}

func (hrs *horizRatioSplitDrawer) draw(d drawFunc, w, h int) error {
	leftW := int(float32(w) * hrs.ratio)
	// h-1 leaves space for the horizontal line
	if err := hrs.left.draw(d, leftW, h-1); err != nil {
		return err
	}

	// draw line
	for y := 0; y < h-1; y++ {
		d(leftW, y, '│', tcell.StyleDefault)
	}
	for x := 0; x < w; x++ {
		if x == leftW {
			continue
		}

		d(x, h-1, '─', tcell.StyleDefault)
	}
	d(leftW, h-1, '┴', tcell.StyleDefault)

	rightX := leftW + 1 // leave space for vertical line
	rightW := w - rightX
	return hrs.right.draw(offset(d, rightX, 0), rightW, h-1)
}

// verticalFixedBottomSplitDrawer draws top and bottom respectively above and
// below each other, providing bottom with a height of bottomH and top with
// the remaining vertical space. It draws no lines.
type verticalFixedBottomSplitDrawer struct {
	bottomH     int
	top, bottom drawer
}

func (vfbs *verticalFixedBottomSplitDrawer) draw(d drawFunc, w, h int) error {
	topH := h - vfbs.bottomH
	if err := vfbs.top.draw(d, w, topH); err != nil {
		return err
	}

	return vfbs.bottom.draw(offset(d, 0, topH), w, vfbs.bottomH)
}

// drawer draws to the screen, taking up a fixed height but variable width.
type dynWDrawer interface {
	// dynWDraw fills the box from x value 0..w where w <= maxW and y value
	// 0..h (both non-inclusive) by calling d. It is the caller's responsibility
	// to fill the range w..maxW.
	dynWDraw(d drawFunc, maxW, h int) (w int, err error)
}

// coverDynWDrawer draws the cover of a song. It breaks all the rules cause it
// writes directly to stdout instead of calling the drawFunc, and it gets its
// position absolutely using absHeight then does math assuming it's in the
// bottom left corner...
type coverDynWDrawer struct {
	queue *[]*track.Track

	// absHeight fetch the full height of the screen
	absHeight func() int

	// prevCover is the cover that was drawn last time. This is used to
	// determine whether we need to re-send the image.
	prevCover image.Image
	// prevW is the width that was occupied when prevCover was drawn.
	prevW *int
}

func (c *coverDynWDrawer) clear() error {
	err := termimage.ClearImages(os.Stdout)
	if err != nil {
		if errors.Is(err, termimage.TerminalUnsupported) {
			return nil
		}

		return err
	}

	c.prevCover = nil
	z := 0
	c.prevW = &z

	return nil
}

func (c *coverDynWDrawer) dynWDraw(d drawFunc, maxW, h int) (w int, err error) {
	if len(*c.queue) == 0 {
		return -1, nil
	}

	cover, err := (*c.queue)[0].Cover()
	if err != nil {
		return 0, err
	}

	if cover == nil {
		if err := c.clear(); err != nil {
			return 0, err
		}

		return -1, nil
	}

	if cover != nil && cover == c.prevCover {
		return *c.prevW, nil
	}

	coverBounds := cover.Bounds()
	coverAspectRatio := float64(coverBounds.Dx()) / float64(coverBounds.Dy())
	cellAspectRatio := 2.28
	w = int(math.Round(float64(h) * coverAspectRatio * cellAspectRatio))
	imageH := h
	if w > maxW {
		w = maxW
		imageH = int(math.Round(float64(w) / coverAspectRatio / cellAspectRatio))
	}

	err = termimage.WriteImage(os.Stdout, cover, image.Rect(0, c.absHeight()-h, w, c.absHeight()-h+imageH))
	if err != nil {
		if errors.Is(err, termimage.TerminalUnsupported) {
			return -1, nil
		}

		return 0, err
	}

	clear(d, w, h)

	c.prevCover = cover
	c.prevW = &w

	return w, nil
}

// horizDynLimitRatioSplitDrawer first draws left will maxW set to w*ratio,
// then draws right in whatever the remaining space is.
type horizDynLimitRatioSplitDrawer struct {
	ratio      float32
	lastRightX *int
	left       dynWDrawer
	right      drawer
}

func (hdlrs *horizDynLimitRatioSplitDrawer) draw(d drawFunc, w, h int) error {
	leftMaxW := int(float32(w) * hdlrs.ratio)
	leftW, err := hdlrs.left.dynWDraw(d, leftMaxW, h)
	if err != nil {
		return err
	}

	for y := 0; y < h; y++ {
		d(leftW, y, ' ', tcell.StyleDefault)
	}

	rightX := leftW + 1
	hdlrs.lastRightX = &rightX
	return hdlrs.right.draw(offset(d, rightX, 0), w-rightX, h)
}

// fillDrawer fills its whole box with r.
type fillDrawer struct {
	r rune
}

func (f *fillDrawer) draw(d drawFunc, w, h int) error {
	fill(d, w, h, f.r)

	return nil
}

func (f *fillDrawer) dynWDraw(d drawFunc, maxW, h int) (w int, err error) {
	fill(d, maxW, h, f.r)

	return maxW, nil
}

type queueDrawer struct {
	queue         *[]*track.Track
	queueFocusIdx *int

	prevD        drawFunc
	prevW, prevH *int
}

func (q *queueDrawer) draw(d drawFunc, w, h int) error {
	q.prevD = d
	q.prevW = &w
	q.prevH = &h

	return q.drawPrev()
}

func (q *queueDrawer) drawPrev() error {
	d, w, h := q.prevD, *q.prevW, *q.prevH

	if len(*q.queue) == 0 {
		s := "queue empty"
		textX := h / 2
		clear(d, w, textX)
		drawString(offset(d, (w-len(s))/2, textX), w, s, tcell.StyleDefault.
			Dim(true).Italic(true).Foreground(tcell.ColorGray))
		clear(offset(d, 0, textX+1), w, h-textX-1)
	}

	y := 0
	for ; y < h && y < len(*q.queue); y++ {
		description, err := (*q.queue)[y].Description()
		if err != nil {
			return fmt.Errorf("failed to get description of queue[%d]: %w", y, err)
		}

		style := tcell.StyleDefault
		if y == *q.queueFocusIdx {
			style = style.Background(tcell.ColorAqua).Foreground(tcell.ColorBlack)
		}
		d(0, y, ' ', style)
		x := drawString(offset(d, 1, y), w-2, description, style)
		for x++; x < w; x++ {
			d(x, y, ' ', style)
		}
	}
	clear(offset(d, 0, y), w, h-y)

	return nil
}

type infoDynWDrawer struct {
	queue *[]*track.Track
}

func (i *infoDynWDrawer) dynWDraw(d drawFunc, maxW, h int) (w int, err error) {
	if len(*i.queue) == 0 {
		return -1, nil
	}

	title, err := (*i.queue)[0].Title()
	if err != nil {
		return -1, fmt.Errorf("failed to get title of queue[0]: %w", err)
	}

	artist, err := (*i.queue)[0].Artist()
	if err != nil {
		return -1, fmt.Errorf("failed to get artist for queue[0]: %w", err)
	}

	tW := drawString(d, maxW, title, tcell.StyleDefault)
	aW := drawString(offset(d, 0, 1), maxW, artist, tcell.StyleDefault.Dim(true))

	w = tW
	if aW > w {
		w = aW
	}

	clear(offset(d, tW, 0), w-tW, 1)
	clear(offset(d, aW, 1), w-aW, 1)
	clear(offset(d, 0, 2), w, 1)

	return w, nil
}

type runeStylePair struct {
	r rune
	s tcell.Style
}

// progress draw assumes h is always 3
type progressDrawer struct {
	streamSeekCloser *beep.StreamSeekCloser
	format           *beep.Format
	// TODO: consider wrapping these in a state struct or something
	paused     *bool
	repeat     *repeat
	shuffleIdx **int

	pausedRMap  map[bool]rune
	repeatRSMap map[repeat]runeStylePair
	shuffleR    rune

	prevDrawer drawFunc
	prevW      *int

	cancelBarDrawer  chan struct{}
	cancelTimeDrawer chan struct{}
}

func newProgressDrawer(canDisplay func(rune) bool) *progressDrawer {
	p := progressDrawer{
		pausedRMap: map[bool]rune{false: '>', true: '>'},
		repeatRSMap: map[repeat]runeStylePair{
			repeatNone:  {'r', tcell.StyleDefault.Dim(true)},
			repeatQueue: {'r', tcell.StyleDefault},
			repeatTrack: {'r', tcell.StyleDefault.Underline(true)},
		},
		shuffleR: 's',
	}

	if canDisplay('') && canDisplay('契') {
		p.pausedRMap = map[bool]rune{false: '', true: '契'}
	}

	if canDisplay('稜') && canDisplay('凌') && canDisplay('綾') {
		p.repeatRSMap = map[repeat]runeStylePair{
			repeatNone:  {'稜', tcell.StyleDefault.Dim(true)},
			repeatQueue: {'凌', tcell.StyleDefault},
			repeatTrack: {'綾', tcell.StyleDefault},
		}
	}

	if canDisplay('列') {
		p.shuffleR = '列'
	}

	return &p
}

func (p *progressDrawer) drawPause() {
	if p.prevDrawer == nil {
		return
	}

	s := tcell.StyleDefault
	if *p.paused || *p.streamSeekCloser == nil {
		s = s.Dim(true)
	}
	p.prevDrawer((*p.prevW)/2, 0, p.pausedRMap[*p.paused], s)
}

func (p *progressDrawer) drawRepeat() {
	if p.prevDrawer == nil {
		return
	}

	p.prevDrawer((*p.prevW)/2+5, 0, p.repeatRSMap[*p.repeat].r, p.repeatRSMap[*p.repeat].s)
}

func (p *progressDrawer) drawShuffle() {
	if p.prevDrawer == nil {
		return
	}

	s := tcell.StyleDefault
	if *p.shuffleIdx == nil {
		s = s.Dim(true)
	}
	p.prevDrawer((*p.prevW)/2-5, 0, p.shuffleR, s)
}

func (p *progressDrawer) widths() (dW, barW int) {
	// the total duration of the current song
	totalD := (*p.format).SampleRate.D((*p.streamSeekCloser).Len()).Truncate(time.Second)

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

	return dW, (*p.prevW) - (2*dW + 2)
}

func (p *progressDrawer) drawBar() {
	if *p.streamSeekCloser == nil || p.prevDrawer == nil {
		return
	}

	currD := (*p.format).SampleRate.D((*p.streamSeekCloser).Position())
	currS := currD.Truncate(time.Second).String()
	totalD := (*p.format).SampleRate.D((*p.streamSeekCloser).Len())
	totalS := totalD.Truncate(time.Second).String()

	dW, barW := p.widths()
	currS = strings.Repeat(" ", dW-len(currS)) + currS
	totalS = strings.Repeat(" ", dW-len(totalS)) + totalS

	drawString(offset(p.prevDrawer, 0, 1), -1, currS, tcell.StyleDefault)
	p.prevDrawer(dW, 1, '|', tcell.StyleDefault)

	progress := float64(currD) / float64(totalD)
	barCompleteW := int(progress * float64(barW))
	drawString(offset(p.prevDrawer, dW+1, 1), -1, strings.Repeat("█", barCompleteW)+
		strings.Repeat(" ", barW-barCompleteW), tcell.StyleDefault)

	p.prevDrawer(*p.prevW-dW-1, 1, '|', tcell.StyleDefault)
	drawString(offset(p.prevDrawer, *p.prevW-dW, 1), -1, totalS, tcell.StyleDefault)
}

func (p *progressDrawer) spawnProgressDrawers(show func()) {
	if p.cancelBarDrawer != nil {
		// drawer already running
		return
	}

	p.cancelBarDrawer = make(chan struct{})
	p.cancelTimeDrawer = make(chan struct{})

	// bar drawer
	go func() {
		for {
			if p.prevW != nil {
				p.drawBar()
				show()
			}

			// the time it takes to draw one section of the bar
			_, barW := p.widths()
			redrawTime := (p.format).SampleRate.D((*p.streamSeekCloser).Len() / barW)

			select {
			case <-time.After(redrawTime):
			case <-p.cancelBarDrawer:
				return
			}
		}
	}()

	// time drawer
	go func() {
		for {
			if p.prevW != nil {
				p.drawBar()
				show()
			}

			select {
			case <-time.After(time.Second):
			case <-p.cancelTimeDrawer:
				return
			}
		}
	}()
}

func (p *progressDrawer) cancelProgressDrawers() {
	if p.cancelBarDrawer != nil {
		close(p.cancelBarDrawer)
		p.cancelBarDrawer = nil
	}
	if p.cancelTimeDrawer != nil {
		close(p.cancelTimeDrawer)
		p.cancelTimeDrawer = nil
	}
}

func (p *progressDrawer) draw(d drawFunc, w, _ int) error {
	p.prevDrawer = d
	p.prevW = &w

	clear(d, w, 3)

	p.drawPause()
	p.drawShuffle()
	p.drawRepeat()
	p.drawBar()

	return nil
}
