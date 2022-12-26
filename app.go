package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"mtoohey.com/q/internal/draw"
	"mtoohey.com/q/internal/query"
	"mtoohey.com/q/internal/track"
	"mtoohey.com/q/internal/types"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"
	"github.com/gdamore/tcell/v2"
)

type app struct {
	// constant configuration options
	sampleRate beep.SampleRate

	// internal state
	paused bool
	repeat types.Repeat
	// if nil, shuffle is disabled; if non-nil, when repeat is enabled, the
	// value is the index within the queue of the first song that has already
	// been played on this repetition
	shuffleIdx    *int
	clipboard     *track.Track
	queue         []*track.Track
	queueFocusIdx int
	warning       error

	// drawers
	rootDrawer       draw.DrawSetScoper
	queueDrawer      *draw.QueueDrawer
	bottomDrawer     draw.Drawer
	coverDrawer      draw.DynWDrawClearer
	switchableDrawer *draw.SwitchableDrawer
	searchDrawer     *draw.SearchDrawer
	progressDrawer   *draw.ProgressDrawer

	// external resources
	streamer         beep.Streamer
	streamSeekCloser beep.StreamSeekCloser
	format           beep.Format
	screen           tcell.Screen
}

func (a *app) fatalf(err error, format string, args ...any) {
	err = fmt.Errorf(fmt.Sprintf("%s: %%w", format), append(args, err)...)
	if err := a.screen.PostEvent(tcell.NewEventError(err)); err != nil {
		// if we drop an error event and fail to exit with it properly,
		// panicking is better than ignoring the error
		a.screen.Fini()
		panic(err)
	}
}

// fatalfIf returns whether err was non-nil.
func (a *app) fatalfIf(err error, format string, args ...any) bool {
	if err != nil {
		a.fatalf(err, format, args...)
		return true
	}

	return false
}

// warning wraps an error to indicate that it should be considered non-fatal
type warning struct {
	err error
}

func (w *warning) Error() string {
	return w.err.Error()
}

func (a *app) warnf(err error, format string, args ...any) {
	err = fmt.Errorf(fmt.Sprintf("%s: %%w", format), append(args, err)...)
	// if we drop a warning, we don't need to panic like with fatalf
	_ = a.screen.PostEvent(tcell.NewEventInterrupt(warning{err}))
}

func (a *app) warnfIf(err error, format string, args ...any) {
	if err != nil {
		a.warnf(err, format, args...)
	}
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
		var err error

		silenceFrom, ok = a.streamer.Stream(samples)
		if !ok {
			// if the streamer failed, warn and set err so that we'll skip
			// below
			err = a.streamer.Err()
			a.warnfIf(err, "streamer failed")
		}

		// if the streamer didn't fill samples, try skipping to the next song
		if silenceFrom < len(samples) {
			// if we ran into an error with the current streamer, drop it from
			// the queue while calling skip
			r := a.repeat
			if !ok && err != nil {
				r = types.RepeatNone
			}

			a.skipLocked(r)
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

func newApp(options appOptions) (a *app, err error) {
	a = &app{
		sampleRate: beep.SampleRate(options.SampleRate),
		repeat:     options.Repeat,
	}

	pathSet := map[string]struct{}{}
	paths := []string{}
	for _, q := range options.InitialQueries {
		newPaths, err := query.Query(options.MusicDir, q)
		if err != nil {
			return nil, fmt.Errorf(`failed to execute query "%s": %w`, q, err)
		}
		for _, p := range newPaths {
			if _, ok := pathSet[p]; !ok {
				paths = append(paths, p)
				pathSet[p] = struct{}{}
			}
		}
	}

	if options.Shuffle {
		shuffleIdx := len(paths)
		a.shuffleIdx = &shuffleIdx
	}

	a.screen, err = tcell.NewScreen()
	if err != nil {
		return nil, err
	}

	if err := a.screen.Init(); err != nil {
		return nil, err
	}
	defer func() {
		// if we encounter a failure starting the app, we should revert the
		// screen before returning
		if err != nil {
			a.screen.Fini()
		}
	}()

	a.queue = make([]*track.Track, len(paths))
	for i, path := range paths {
		a.queue[i] = &track.Track{Path: path}
	}

	if len(a.queue) > 0 {
		// we're not actually playing yet, so we can call this even though the
		// speaker isn't locked
		a.playQueueTopLocked()
	}

	a.progressDrawer = draw.NewProgressDrawer(func(r rune) bool {
		return a.screen.CanDisplay(r, false)
	})
	a.progressDrawer.StreamSeekCloser = &a.streamSeekCloser
	a.progressDrawer.Format = &a.format
	a.progressDrawer.Paused = &a.paused
	a.progressDrawer.Repeat = &a.repeat
	a.progressDrawer.ShuffleIdx = &a.shuffleIdx
	a.progressDrawer.Warning = &a.warning

	a.queueDrawer = &draw.QueueDrawer{
		Queue:         &a.queue,
		QueueFocusIdx: &a.queueFocusIdx,
		ScrollOff:     options.ScrollOff,
	}

	a.coverDrawer = &draw.CoverDynWDrawer{
		Queue: &a.queue,
		AbsHeight: func() int {
			_, h := a.screen.Size()
			return h
		},
	}

	a.searchDrawer = &draw.SearchDrawer{
		Screen:   a.screen,
		MusicDir: options.MusicDir,
	}

	initialTab := options.InitialTab
	if len(a.queue) == 0 {
		initialTab = types.TabSearch
	}
	a.switchableDrawer = &draw.SwitchableDrawer{
		Drawers: []draw.Drawer{
			types.TabMetadata: &draw.MetadataDrawer{Queue: &a.queue},
			types.TabLyrics:   &draw.LyricsDrawer{Queue: &a.queue},
			types.TabSearch:   a.searchDrawer,
			// TODO: visualizer
		},
		Tab: initialTab,
	}

	a.bottomDrawer = &draw.HorizDynLimitRatioSplitDrawer{
		Ratio: 1.0 / 4,
		Left:  a.coverDrawer,
		Right: &draw.HorizDynLimitRatioSplitDrawer{
			Ratio: 1.0 / 3,
			Left:  &draw.InfoDynWDrawer{Queue: &a.queue},
			Right: a.progressDrawer,
		},
	}

	a.rootDrawer = &draw.VertFixedBotSplitDrawer{
		BottomH: 3,
		Top: &draw.HorizRatioSplitDrawer{
			Ratio: 1.0 / 3,
			Left:  a.queueDrawer,
			Right: a.switchableDrawer,
		},
		Bottom: a.bottomDrawer,
	}

	return a, nil
}

func (a *app) loop() error {
	// we don't draw to start, because we get a resize event on startup, which
	// causes the first draw

	for {
		switch ev := a.screen.PollEvent().(type) {
		case *tcell.EventInterrupt:
			if w, ok := ev.Data().(warning); ok {
				a.warning = w.err
				go func(e error) {
					// if the message is still the same one in 1 second, clear it
					time.Sleep(time.Second)
					if a.warning == e {
						a.warning = nil
						a.progressDrawer.DrawWarning()
						a.screen.Show()
					}
				}(w.err)
			}

		case *tcell.EventError:
			return ev

		case *tcell.EventResize:
			w, h := ev.Size()
			a.rootDrawer.SetScope(func(x, y int, r rune, s tcell.Style) {
				a.screen.SetContent(x, y, r, nil, s)
			}, w, h)
			a.fatalfIf(a.coverDrawer.Clear(), "cover clear failed")
			if !a.fatalfIf(a.rootDrawer.Draw(), "root draw failed") &&
				a.streamSeekCloser != nil && !a.paused {
				a.progressDrawer.SpawnProgressDrawers(a.screen.Show)
			}

		case *tcell.EventKey:
			switch ev.Key() {
			case tcell.KeyCtrlC:
				return nil

			case tcell.KeyLeft:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.Left(), "search left failed")
				} else {
					a.seekBy(time.Second * -5)
				}

			case tcell.KeyRight:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.Right(), "search right failed")
				} else {
					a.seekBy(time.Second * 5)
				}

			case tcell.KeyHome, tcell.KeyCtrlA:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.ToStart(), "search to start failed")
				}

			case tcell.KeyEnd, tcell.KeyCtrlE:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.ToEnd(), "search to end failed")
				}

			case tcell.KeyCtrlW:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.KillWord(), "search kill word failed")
				}

			case tcell.KeyDown:
				if a.searching() {
					a.fatalfIf(a.searchDrawer.ShiftFocus(1), "search shift focus failed")
				} else {
					a.queueFocusShift(1)
				}

			case tcell.KeyUp:
				if a.searching() {
					a.fatalfIf(a.searchDrawer.ShiftFocus(-1), "search shift focus failed")
				} else {
					a.queueFocusShift(-1)
				}

			case tcell.KeyCtrlD, tcell.KeyCtrlF:
				a.queueFocusShift(10)

			case tcell.KeyCtrlU, tcell.KeyCtrlB:
				a.queueFocusShift(-10)

			case tcell.KeyPgDn:
				a.queueFocusShift(a.queueDrawer.Height())

			case tcell.KeyPgUp:
				a.queueFocusShift(-a.queueDrawer.Height())

			case tcell.KeyEnter:
				if a.typing() && a.searchDrawer.ResultsIdx == 0 {
					a.fatalfIf(a.searchDrawer.ShiftFocus(1), "search shift focus failed")
				} else if !a.searching() {
					a.jumpFocused()
				}

			case tcell.KeyTAB:
				a.fatalfIf(a.switchableDrawer.Cycle(), "cycle failed")

			case tcell.KeyBackspace2:
				if !a.typing() {
					break
				}
				a.fatalfIf(a.searchDrawer.Backspace(), "search backspace failed")

			case tcell.KeyRune:
				if a.typing() {
					a.fatalfIf(a.searchDrawer.Insert(ev.Rune()), "search insert failed")
					break
				} else if a.searching() {
					switch ev.Rune() {
					case 'a', 'A':
						if res := a.searchDrawer.FocusedResult(); res != "" {
							a.queue = append(a.queue, &track.Track{Path: res})
							if len(a.queue) == 1 {
								a.playQueueTop()
								a.fatalfIf(a.bottomDrawer.Draw(), "bottom draw failed")
							}
							a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
						}

					case 'i':
						// TODO: make the location that the track gets inserted
						// dependent on the focused index

						res := a.searchDrawer.FocusedResult()
						if res == "" {
							break
						}

						if len(a.queue) > 0 {
							a.queue = append([]*track.Track{a.queue[0], {Path: res}}, a.queue[1:]...)
							if a.shuffleIdx != nil {
								*a.shuffleIdx++
							}
							a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
							break
						}

						fallthrough
					case 'I':
						res := a.searchDrawer.FocusedResult()
						if res == "" {
							break
						}

						a.queue = append([]*track.Track{{Path: res}}, a.queue...)
						if a.shuffleIdx != nil {
							*a.shuffleIdx++
						}
						wasPlaying := !a.paused && a.streamer != nil
						a.playQueueTop()
						if !wasPlaying {
							a.paused = false
							a.progressDrawer.SpawnProgressDrawers(a.screen.Show)
						}
						a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
						a.fatalfIf(a.bottomDrawer.Draw(), "bottom draw failed")
					}
				}

				switch ev.Rune() {
				case ' ':
					a.cyclePause()

				case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
					a.seekPercent(float32(ev.Rune()-'0') / 10)

				case 'j':
					a.queueFocusShift(1)

				case 'k':
					a.queueFocusShift(-1)

				case 'g':
					a.queueFocusShift(math.MinInt)

				case 'G':
					a.queueFocusShift(math.MaxInt)

				case 'd', 'x':
					a.removeFocused()

				case 'D', 'X':
					a.clearQueue()

				case 'S':
					a.reshuffle()

				case 's':
					a.cycleShuffle()

				case 'r':
					a.cycleRepeat()

				case 'p', 'P':
					a.paste(ev.Rune() == 'P')

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

func (a *app) typing() bool {
	return a.switchableDrawer.Tab == types.TabSearch && a.searchDrawer.ResultsIdx == 0
}

func (a *app) searching() bool {
	return a.switchableDrawer.Tab == types.TabSearch
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
		a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
	}
}

func (a *app) paste(before bool) {
	if a.clipboard == nil {
		return
	}

	if !before {
		a.queueFocusIdx++
	}
	if a.shuffleIdx != nil {
		if *a.shuffleIdx >= a.queueFocusIdx {
			*a.shuffleIdx++
		}
	}
	a.queue = append(a.queue[:a.queueFocusIdx],
		append([]*track.Track{a.clipboard}, a.queue[a.queueFocusIdx:]...)...)
	a.clipboard = nil

	if a.queueFocusIdx == 0 {
		a.playQueueTop()
		a.fatalfIf(a.bottomDrawer.Draw(), "bottom draw failed")
		a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabMetadata), "metadata draw failed")
		a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabLyrics), "lyrics draw failed")
	}
	a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
}

func (a *app) removeFocused() {
	if len(a.queue) == 0 {
		return
	}

	a.clipboard = a.queue[a.queueFocusIdx]

	if a.shuffleIdx != nil && *a.shuffleIdx > a.queueFocusIdx {
		*a.shuffleIdx--
	}

	a.queue = append(a.queue[:a.queueFocusIdx], a.queue[a.queueFocusIdx+1:]...)

	if a.queueFocusIdx == 0 {
		a.playQueueTop()
		a.warnfIf(a.bottomDrawer.Draw(), "bottom draw failed")
		a.warnfIf(a.switchableDrawer.DrawIfVisible(types.TabMetadata), "metadata draw failed")
		a.warnfIf(a.switchableDrawer.DrawIfVisible(types.TabLyrics), "lyrics draw failed")
	}

	if a.queueFocusIdx >= len(a.queue) {
		a.queueFocusIdx = len(a.queue) - 1
	}

	a.warnfIf(a.queueDrawer.Draw(), "queue draw failed")
}

func (a *app) clearQueue() {
	if a.shuffleIdx != nil {
		*a.shuffleIdx = 0
	}
	a.queueFocusIdx = 0
	a.queue = nil

	a.playQueueTop()
	a.warnfIf(a.bottomDrawer.Draw(), "bottom draw failed")
	a.warnfIf(a.switchableDrawer.DrawIfVisible(types.TabMetadata), "metadata draw failed")
	a.warnfIf(a.switchableDrawer.DrawIfVisible(types.TabLyrics), "lyrics draw failed")

	a.warnfIf(a.queueDrawer.Draw(), "queue draw failed")
}

func (a *app) jumpFocused() {
	if a.queueFocusIdx == 0 {
		wasPaused := a.paused
		if a.streamSeekCloser != nil {
			speaker.Lock()
			if a.paused {
				a.paused = false
			}
			a.warnfIf(a.streamSeekCloser.Seek(0), "failed to seek ssc")
			speaker.Unlock()
		}
		if wasPaused {
			a.progressDrawer.DrawPause()
			a.progressDrawer.SpawnProgressDrawers(a.screen.Show)
		}
		a.progressDrawer.DrawBar()

		return
	}

	switch a.repeat {
	case types.RepeatNone:
		a.queue = a.queue[a.queueFocusIdx:]
	case types.RepeatTrack, types.RepeatQueue:
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

	if a.paused {
		speaker.Lock()
		a.paused = false
		speaker.Unlock()
		a.progressDrawer.DrawPause()
		a.progressDrawer.SpawnProgressDrawers(a.screen.Show)
	}

	a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
	a.fatalfIf(a.bottomDrawer.Draw(), "bottom draw failed")
	a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabMetadata), "metadata draw failed")
	a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabLyrics), "lyrics draw failed")
}

func (a *app) cyclePause() {
	speaker.Lock()
	a.paused = !a.paused
	speaker.Unlock()
	a.progressDrawer.DrawPause()
	if a.paused {
		a.progressDrawer.CancelProgressDrawers()
		a.progressDrawer.DrawBar()
	} else if len(a.queue) > 0 {
		a.progressDrawer.SpawnProgressDrawers(a.screen.Show)
	}
}

func (a *app) cycleRepeat() {
	a.repeat = (a.repeat + 1) % 3
	a.progressDrawer.DrawRepeat()
}

func (a *app) cycleShuffle() {
	if a.shuffleIdx == nil {
		shuffleIdx := len(a.queue)
		a.shuffleIdx = &shuffleIdx
	} else {
		a.shuffleIdx = nil
	}
	a.progressDrawer.DrawShuffle()
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

	a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
}

func (a *app) skip() {
	speaker.Lock()
	a.skipLocked(a.repeat)
	speaker.Unlock()
}

func (a *app) skipLocked(r types.Repeat) {
	if len(a.queue) == 0 {
		return
	}

	if r == types.RepeatTrack || (r == types.RepeatQueue && len(a.queue) == 1) {
		if a.streamSeekCloser == nil {
			return
		}

		if err := a.streamSeekCloser.Seek(0); err != nil {
			a.warnf(err, "failed to seek ssc")
			a.skipLocked(types.RepeatNone)
		}

		return
	}

	// update queue
	if r == types.RepeatNone {
		a.queue = a.queue[1:]
		if a.queueFocusIdx >= len(a.queue) {
			a.queueFocusIdx = len(a.queue) - 1
		}
	} else {
		if a.shuffleIdx == nil {
			a.queue = append(a.queue[1:], a.queue[0])
		} else {
			// we've played through the whole thing once, reset
			if *a.shuffleIdx == 0 {
				*a.shuffleIdx = len(a.queue)
			}

			var r int
			if *a.shuffleIdx == 1 && len(a.queue) > 2 {
				// special case: we don't want to move the current track to the
				// start again and play it twice in a row
				r = *a.shuffleIdx + 1 + rand.Intn(len(a.queue)-*a.shuffleIdx)
			} else {
				r = *a.shuffleIdx + rand.Intn(len(a.queue)-*a.shuffleIdx+1)
			}
			a.queue = append(a.queue[1:r], append([]*track.Track{a.queue[0]}, a.queue[r:]...)...)
			*a.shuffleIdx--
		}
	}

	a.playQueueTopLocked()

	a.fatalfIf(a.queueDrawer.Draw(), "queue draw failed")
	a.fatalfIf(a.bottomDrawer.Draw(), "bottom draw failed")
	a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabMetadata), "metadata draw failed")
	a.fatalfIf(a.switchableDrawer.DrawIfVisible(types.TabLyrics), "lyrics draw failed")
}

func (a *app) playQueueTop() {
	speaker.Lock()
	a.playQueueTopLocked()
	speaker.Unlock()
}

func (a *app) playQueueTopLocked() {
	if a.streamSeekCloser != nil {
		a.warnfIf(a.streamSeekCloser.Close(), "failed to close ssc")
	}

	if len(a.queue) > 0 {
		var err error
		a.streamSeekCloser, a.format, err = a.queue[0].Decode()
		if err != nil {
			a.warnf(err, "failed to decode queue[0]")
			a.skipLocked(a.repeat)
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
		a.progressDrawer.CancelProgressDrawers()
	}
}

func (a *app) seekBy(d time.Duration) {
	speaker.Lock()
	newP := a.streamSeekCloser.Position() + a.format.SampleRate.N(d)
	if newP > a.streamSeekCloser.Len() {
		newP = a.streamSeekCloser.Len() - 1
	} else if newP < 0 {
		newP = 0
	}
	a.warnfIf(a.streamSeekCloser.Seek(newP), "failed to seekBy")
	speaker.Unlock()

	a.progressDrawer.DrawBar()
}

func (a *app) seekPercent(p float32) {
	if p < 0 || p > 1 {
		panic(fmt.Sprintf("seekPercent called with invalid percent %f", p))
	}
	speaker.Lock()
	a.warnfIf(a.streamSeekCloser.Seek(int(float32(a.streamSeekCloser.Len())*p)), "failed to seekPercent")
	speaker.Unlock()

	a.progressDrawer.DrawBar()
}
