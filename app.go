package main

import (
	"fmt"
	"math/rand"
	"time"

	"mtoohey.com/q/internal/track"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell/v2"
)

type app struct {
	// constant configuration options
	sampleRate beep.SampleRate

	// internal state
	paused        bool
	queue         []*track.Track
	repeat        repeat
	queueFocusIdx int
	// if nil, shuffle is disabled; if non-nil, when repeat is enabled, the
	// value is the index within the queue of the first song that has already
	// been played on this repetition
	shuffleIdx *int

	// drawers
	rootDrawer     drawer
	progressDrawer *progressDrawer
	coverDrawer    *coverDynWDrawer
	queueDrawer    *queueDrawer

	// external resources
	streamer         beep.Streamer
	streamSeekCloser beep.StreamSeekCloser
	format           beep.Format
	screen           tcell.Screen
}

// TODO: add warning error type that only displays the error and doesn't crash
// the program

func (app *app) fatalf(err error, format string, a ...any) {
	err = fmt.Errorf(fmt.Sprintf("%s: %%w", format), append(a, err)...)
	app.screen.PostEvent(tcell.NewEventError(err))
}

// Err implements beep.Streamer
func (a *app) Err() error {
	return nil // we never want the app to be drained
}

// Stream implements beep.Streamer
func (a *app) Stream(samples [][2]float64) (n int, ok bool) {
	silenceFrom := 0

	if !a.paused && a.streamer != nil {
		var ok bool
		silenceFrom, ok = a.streamer.Stream(samples)
		if !ok {
			// if the streamer failed, try posting an error
			if err := a.streamer.Err(); err != nil {
				a.fatalf(err, "streamer failed")
			}
		}

		// if the streamer didn't fill samples, try skipping to the next song
		if silenceFrom < len(samples) {
			a.skipLocked()
			// recursively continue streaming after the skip to avoid silence,
			// if there's no now-playing song after the skip, the recurisve
			// call will realize this and fill the rest of samples with silence
			n, _ := a.Stream(samples[silenceFrom:])
			silenceFrom += n
		}
	}

	for i := silenceFrom; i < len(samples); i++ {
		samples[i] = [2]float64{}
	}

	return len(samples), true
}

func newApp(options appOptions) (*app, error) {
	a := &app{
		sampleRate: beep.SampleRate(options.SampleRate),
		repeat:     options.Repeat,
	}
	if options.Shuffle {
		shuffleIdx := len(options.Paths)
		a.shuffleIdx = &shuffleIdx
	}

	var err error
	a.screen, err = tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	if err := a.screen.Init(); err != nil {
		return nil, err
	}

	a.queue = make([]*track.Track, len(options.Paths))
	for i, path := range options.Paths {
		a.queue[i] = &track.Track{Path: path}
	}

	if len(a.queue) > 0 {
		// we're not actually playing yet, so we can call this even though the
		// speaker isn't locked
		a.playQueueTopLocked()
	}

	a.progressDrawer = newProgressDrawer(func(r rune) bool {
		return a.screen.CanDisplay(r, false)
	})
	a.progressDrawer.streamSeekCloser = &a.streamSeekCloser
	a.progressDrawer.format = &a.format
	a.progressDrawer.paused = &a.paused
	a.progressDrawer.repeat = &a.repeat
	a.progressDrawer.shuffleIdx = &a.shuffleIdx

	a.coverDrawer = &coverDynWDrawer{
		queue: &a.queue,
		absHeight: func() int {
			_, h := a.screen.Size()
			return h
		},
	}

	a.queueDrawer = &queueDrawer{
		queue:         &a.queue,
		queueFocusIdx: &a.queueFocusIdx,
	}

	a.rootDrawer = &verticalFixedBottomSplitDrawer{
		bottomH: 3,
		top: &horizRatioSplitDrawer{
			ratio: 1.0 / 3,
			left:  a.queueDrawer,
			right: &fillDrawer{' '}, // TODO: switchable between lyrics, visualizer, full metadata, search
		},
		bottom: &horizDynLimitRatioSplitDrawer{
			ratio: 1.0 / 4,
			left:  a.coverDrawer,
			right: &horizDynLimitRatioSplitDrawer{
				ratio: 1.0 / 3,
				left:  &infoDynWDrawer{&a.queue},
				right: a.progressDrawer,
			},
		},
	}

	return a, nil
}

func (a *app) draw() {
	w, h := a.screen.Size()
	err := a.rootDrawer.draw(func(x, y int, r rune, s tcell.Style) {
		a.screen.SetContent(x, y, r, nil, s)
	}, w, h)
	if err != nil {
		a.fatalf(err, "drawing failed")
	}
}

func (a *app) loop() error {
	// we don't draw to start, because we get a resize event on startup, which
	// causes the first draw
	sawInitialResize := false

	for {
		switch ev := a.screen.PollEvent().(type) {
		case *tcell.EventError:
			return fmt.Errorf("error event: %w", ev)

		case *tcell.EventResize:
			a.coverDrawer.clear()
			a.draw()
			if !sawInitialResize {
				sawInitialResize = true
				a.progressDrawer.spawnProgressDrawers(a.screen.Show)
			}

		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyCtrlC:
				return nil

			case tcell.KeyLeft:
				a.seekBy(time.Second*5, false)

			case tcell.KeyRight:
				a.seekBy(time.Second*5, true)

			case tcell.KeyDown:
				a.queueFocusShift(1)

			case tcell.KeyUp:
				a.queueFocusShift(-1)

			case tcell.KeyEnter:
				a.jumpFocused()

			case tcell.KeyRune:
				switch ev.Rune() {
				case ' ':
					a.cyclePause()

				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
					a.seekPercent(float32(ev.Rune()-'0') / 10)

				case 'j':
					a.queueFocusShift(1)

				case 'k':
					a.queueFocusShift(-1)

				case 'd', 'x':
					a.removeFocused()

				case 'S':
					a.reshuffle()

				case 's':
					a.cycleShuffle()

				case 'r':
					a.cycleRepeat()

				case 'n':
					a.skip()

				// case 'N':
				// a.skipBack()
				// TODO: implement this; will require keeping a separate record
				// of the past queue when:
				//
				// - we are not repeating the queue, regardless of shuffle
				// - we are shuffling and repeating the queue

				case 'q', 'Q':
					return nil
				}
			}
		}

		a.screen.Show()
	}
}

func (a *app) queueFocusShift(i int) {
	newIdx := a.queueFocusIdx + i
	if newIdx >= len(a.queue) {
		newIdx = len(a.queue) - 1
	} else if newIdx < 0 {
		newIdx = 0
	}

	a.queueFocusIdx = newIdx

	if i != 0 {
		a.queueDrawer.drawPrev()
	}
}

func (a *app) removeFocused() {
	if a.shuffleIdx != nil && *a.shuffleIdx > a.queueFocusIdx {
		*a.shuffleIdx--
	}

	a.queue = append(a.queue[:a.queueFocusIdx], a.queue[a.queueFocusIdx+1:]...)

	if a.queueFocusIdx == 0 {
		a.playQueueTop()
		// TODO: redraw bottom bar here
	}

	if a.queueFocusIdx >= len(a.queue) {
		a.queueFocusIdx = len(a.queue) - 1
	}

	a.queueDrawer.drawPrev()
}

func (a *app) jumpFocused() {
	if a.queueFocusIdx == 0 {
		if a.streamSeekCloser != nil {
			if err := a.streamSeekCloser.Seek(0); err != nil {
				a.fatalf(err, "failed to seek ssc")
			}
		}

		return
	}

	switch a.repeat {
	case repeatNone:
		a.queue = a.queue[a.queueFocusIdx:]
		a.queueFocusIdx = 0
	case repeatTrack, repeatQueue:
		if a.shuffleIdx != nil {
			if *a.shuffleIdx >= a.queueFocusIdx {
				shuffle(a.queue[:a.queueFocusIdx])
				*a.shuffleIdx -= a.queueFocusIdx
			} else {
				shuffle(a.queue[:*a.shuffleIdx])
				shuffle(a.queue[*a.shuffleIdx:a.queueFocusIdx])
				*a.shuffleIdx -= a.queueFocusIdx - len(a.queue)
			}
		}

		a.queue = append(a.queue[a.queueFocusIdx:], a.queue[:a.queueFocusIdx]...)
	}

	a.queueFocusIdx = 0

	a.playQueueTop()
	a.queueDrawer.drawPrev()
	// TODO: draw bottom bar
}

func (a *app) cyclePause() {
	speaker.Lock()
	a.paused = !a.paused
	speaker.Unlock()
	a.progressDrawer.drawPause()
	if a.paused {
		a.progressDrawer.cancelProgressDrawers()
		a.progressDrawer.drawBar()
	} else if len(a.queue) > 0 {
		a.progressDrawer.spawnProgressDrawers(a.screen.Show)
	}
}

func (a *app) cycleRepeat() {
	a.repeat = (a.repeat + 1) % 3
	a.progressDrawer.drawRepeat()
}

func (a *app) cycleShuffle() {
	if a.shuffleIdx == nil {
		shuffleIdx := len(a.queue)
		a.shuffleIdx = &shuffleIdx
	} else {
		a.shuffleIdx = nil
	}
	a.progressDrawer.drawShuffle()
}

func shuffle[T any](s []T) {
	rand.Shuffle(len(s), func(i, j int) {
		s[i], s[j] = s[j], s[i]
	})
}

// reshuffle reorders the queue based on the following rules:
//
//   - if shuffle is disabled, the whole queue, excluding the current song, is
//     randomly re-ordered
//   - if shuffle is enabled, both a.queue[1:*a.shuffleIdx] and
//     a.queue[*a.shuffleIdx:] are independently re-ordered
func (a *app) reshuffle() {
	// we need at least 3 songs for shuffle to have any affect
	if len(a.queue) < 3 {
		return
	}

	if a.shuffleIdx == nil {
		shuffle(a.queue[1:])
	} else {
		shuffle(a.queue[1:*a.shuffleIdx])
		shuffle(a.queue[*a.shuffleIdx:])
	}

	a.queueDrawer.drawPrev()
	a.screen.Show()
}

func (a *app) skip() {
	speaker.Lock()
	a.skipLocked()
	speaker.Unlock()
}

func (a *app) skipLocked() {
	if len(a.queue) == 0 {
		return
	}

	if a.repeat == repeatTrack || (a.repeat == repeatQueue && len(a.queue) == 1) {
		if err := a.streamSeekCloser.Seek(0); err != nil {
			a.fatalf(err, "failed to seek ssc")
		}

		return
	}

	// update queue
	if a.repeat == repeatNone {
		a.queue = a.queue[1:]
		if a.queueFocusIdx >= len(a.queue) {
			a.queueFocusIdx = len(a.queue) - 1
		}
	} else {
		if a.shuffleIdx == nil {
			a.queue = append(a.queue[1:], a.queue[0])
		} else {
			var r int
			if *a.shuffleIdx == 1 {
				// special case: we don't want to move the current track to the
				// start again and play it twice in a row
				r = *a.shuffleIdx + 1 + rand.Intn(len(a.queue)-*a.shuffleIdx)
			} else {
				r = *a.shuffleIdx + rand.Intn(len(a.queue)-*a.shuffleIdx+1)
			}
			a.queue = append(a.queue[1:r], append([]*track.Track{a.queue[0]}, a.queue[r:]...)...)
			*a.shuffleIdx--
			// we've played through the whole thing once, reset
			if *a.shuffleIdx == 0 {
				*a.shuffleIdx = len(a.queue)
			}
		}
	}

	a.playQueueTopLocked()

	// TODO: only re-draw the queue and the stuff in the bottom bar
	a.draw()
	// a.queueDrawer.draw()
	// a.coverDrawer.dynWDraw()
	// a.progressDrawer.drawPause()
	// a.progressDrawer.drawBar()
	a.screen.Show()
}

// callers are required to verify that queue[0] exists
func (a *app) playQueueTop() {
	speaker.Lock()
	a.playQueueTopLocked()
	speaker.Unlock()
}

// callers are required to verify that queue[0] exists
func (a *app) playQueueTopLocked() {
	if a.streamSeekCloser != nil {
		if err := a.streamSeekCloser.Close(); err != nil {
			a.fatalf(err, "failed to close ssc")
		}
	}

	if len(a.queue) > 0 {
		var err error
		a.streamSeekCloser, a.format, err = a.queue[0].Decode()
		if err != nil {
			a.fatalf(err, "failed to decode queue[0]")
			a.skipLocked()
			return
		}

		if a.format.SampleRate == a.sampleRate {
			a.streamer = a.streamSeekCloser
		} else {
			a.streamer = beep.Resample(4, a.format.SampleRate, a.sampleRate, a.streamSeekCloser)
		}
	} else {
		a.streamSeekCloser = nil
		a.streamer = nil
		a.progressDrawer.cancelProgressDrawers()
	}
}

func (a *app) seekBy(d time.Duration, reverse bool) {
	speaker.Lock()
	newP := a.streamSeekCloser.Position() + a.format.SampleRate.N(d)
	if newP > a.streamSeekCloser.Len() {
		newP = a.streamSeekCloser.Len() - 1
	}
	err := a.streamSeekCloser.Seek(newP)
	speaker.Unlock()
	if err != nil {
		a.fatalf(err, "failed to seekBy")
	}

	a.progressDrawer.drawBar()
	a.screen.Show()
}

func (a *app) seekPercent(p float32) {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("seekPercent called with invalid percent %f", p))
	}
	speaker.Lock()
	err := a.streamSeekCloser.Seek(int(float32(a.streamSeekCloser.Len()) * p))
	speaker.Unlock()
	if err != nil {
		a.fatalf(err, "failed to seekPercent")
	}

	a.progressDrawer.drawBar()
	a.screen.Show()
}

func (a *app) close() {
	a.screen.Fini()
}
