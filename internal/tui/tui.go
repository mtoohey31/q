package tui

import (
	"fmt"
	"image"
	"log"
	"math"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"

	"github.com/gdamore/tcell/v2"
)

type tui struct {
	// constants
	Cmd
	cmd.Globals
	logger *log.Logger

	// resources
	screen tcell.Screen
	conn   protocol.Conn

	// data state
	protocol.State

	// ui state
	shuffleRune        rune
	pauseRuneMap       map[protocol.PauseState]rune
	repeatRuneStyleMap map[protocol.RepeatState]runeStylePair

	topR      image.Rectangle
	bottomR   image.Rectangle
	queueR    image.Rectangle
	queryR    image.Rectangle
	coverMaxR image.Rectangle
	infoMaxR  image.Rectangle
	progressR image.Rectangle
	barR      image.Rectangle
	modeR     image.Rectangle
	errorR    image.Rectangle

	visibleErr error

	mode mode

	clipboardPath  string
	queueFocusIdx  int
	queueScrollIdx int

	queryMouseIdx  int
	queryFocusIdx  int
	queryScrollIdx int
	queryString    string
	queryResults   protocol.QueryResults
}

// newTUI creates a new tui. conn should not yet have had its initial message
// read when it is passed to this function. This function may block
// indefinitely if the server does not send an initial message over conn. If
// this function returns an error, conn will not be closed; it is the caller's
// responsibility to close it in that scenario, if they no longer intend to use
// it.
func newTUI(cmd Cmd, g cmd.Globals, logger *log.Logger, conn protocol.Conn) (*tui, error) {
	t := &tui{
		Cmd:     cmd,
		Globals: g,
		logger:  logger,
		conn:    conn,
	}

	m, err := conn.Receive()
	if err != nil {
		return nil, fmt.Errorf("failed to receive initial message: %w", err)
	}

	var ok bool
	t.State, ok = m.(protocol.State)
	if !ok {
		return nil, fmt.Errorf("initial message was of unexpected type %T", m)
	}

	if t.State.Version != protocol.Version {
		return nil, fmt.Errorf(`server version "%s" does not match tui version "%s"`,
			t.State.Version, protocol.Version)
	}

	t.screen, err = tcell.NewScreen()
	if err != nil {
		return nil, fmt.Errorf("failed to create screen: %w", err)
	}

	if err := t.screen.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize screen: %w", err)
	}

	t.initIndicatorRunes()

	return t, nil
}

func (t *tui) close() error {
	t.screen.Fini()
	if err := t.conn.Close(); err != nil {
		return fmt.Errorf("failed to close connection: %w", err)
	}

	return nil
}

func (t *tui) resize(w, h int) error {
	t.topR, t.bottomR = vSplitFixedBottom(image.Rect(0, 0, w, h), 3)

	t.queueR, t.queryR = hSplitRatio(t.topR, 1.0/3)
	t.queueFocus(t.queueFocusIdx) // make sure scrolloff is still ok

	t.drawQuery()

	for c := t.queryR.Min.Add(image.Pt(-1, 0)); c.Y < t.bottomR.Min.Y; c.Y++ {
		t.draw(c, '│', tcell.StyleDefault)
	}
	for c := t.bottomR.Min.Add(image.Pt(0, -1)); c.X < t.bottomR.Max.X; c.X++ {
		t.draw(c, '─', tcell.StyleDefault)
	}
	t.draw(image.Pt(t.queryR.Min.X-1, t.bottomR.Min.Y-1), '┴', tcell.StyleDefault)

	t.coverMaxR, _ = hSplitRatio(t.bottomR, 1.0/4)
	// the call below also sets a bunch of other stuff since they
	// get reset each time cover is redrawn
	if err := t.drawBottom(
		t.NowPlaying.Cover,
		// force a clear and redraw of the image, since we resized;
		// the non-nil zero-value image shouldn't be equal to the
		// previous image, so a clear will happen
		image.Image(&image.RGBA{}),
	); err != nil {
		return fmt.Errorf("failed to draw bottom: %w", err)
	}

	return nil
}

func (t *tui) loop() (err error) {
	var wg sync.WaitGroup
	defer wg.Wait()
	defer func() {
		closeErr := t.close()

		if err == nil {
			err = closeErr
		}
	}()

	screenEvCh := make(chan tcell.Event)
	wg.Add(1)
	go func() {
		defer wg.Done()
		t.screen.ChannelEvents(screenEvCh, nil)
	}()

	// spawn message receive routine

	// buffered so that we don't get stuck trying to send messages to the
	// server, but it's not receiving because it's trying to send and we're not
	// listening
	serverMessageCh := make(chan protocol.Message, 128)
	// buffered for the same reason as above, and also so that the final
	// net.ErrClosed can be sent without deadlocking on shutdown, though it
	// won't ever be read
	serverErrorCh := make(chan error, 128)
	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			m, err := t.conn.Receive()
			if err != nil {
				// even if it's net.ErrClosed, this is still unexpected and
				// fatal, because if there's no server left we need to exit
				serverErrorCh <- err
				return
			}

			serverMessageCh <- m
		}
	}()

	var errTimeoutCancel chan struct{}
	defer func() {
		if errTimeoutCancel != nil {
			close(errTimeoutCancel)
			errTimeoutCancel = nil
		}
	}()
	clearErrCh := make(chan struct{})
	setNewErr := func(err error) {
		t.logger.Print(err)

		// if there's an existing timeout routine running, stop it
		if errTimeoutCancel != nil {
			close(errTimeoutCancel)
		}

		// set the new error
		t.visibleErr = err
		t.drawError()

		// start a new timeout routine
		errTimeoutCancel = make(chan struct{})
		wg.Add(1)
		go func() {
			defer wg.Done()

			select {
			case <-time.After(time.Second * 3):
				clearErrCh <- struct{}{}
			case <-errTimeoutCancel:
			}
		}()
	}

	for {
		select {
		case err := <-serverErrorCh:
			return fmt.Errorf("server connection lost: %w", err)

		case <-clearErrCh:
			t.visibleErr = nil
			errTimeoutCancel = nil
			t.drawError()

		case m := <-serverMessageCh:
			switch m := m.(type) {
			case protocol.Error:
				setNewErr(m)

			case protocol.ProgressState:
				t.Progress = m
				t.drawBar()

			case protocol.NowPlayingState:
				old := t.NowPlaying.Cover
				t.NowPlaying = m
				if err := t.drawBottom(m.Cover, old); err != nil {
					return fmt.Errorf("failed to draw bottom: %w", err)
				}

			case protocol.QueueState:
				t.Queue = m
				// clamps and redraws
				t.queueFocus(t.queueFocusIdx)

			case protocol.PauseState:
				t.Pause = m
				t.drawPause()

			case protocol.ShuffleState:
				t.Shuffle = m
				t.drawShuffle()

			case protocol.RepeatState:
				t.Repeat = m
				t.drawRepeat()

			case protocol.QueryResults:
				t.queryResults = m
				t.drawQuery()

			case protocol.Removed:
				t.clipboardPath = string(m)

			default:
				setNewErr(fmt.Errorf("unhandled message type: %T", m))
			}

		case ev := <-screenEvCh:
			switch ev := ev.(type) {
			case *tcell.EventError:
				return fmt.Errorf("got error event: %w", ev)

			case *tcell.EventResize:
				if err := t.resize(ev.Size()); err != nil {
					return err
				}

			case *tcell.EventKey:
				err = nil

				switch t.mode {
				case modeNormal:
					switch ev.Key() {
					case tcell.KeyCtrlC:
						return nil

					case tcell.KeyLeft:
						err = t.conn.Send(protocol.Seek(t.Progress.Current - time.Second*5))

					case tcell.KeyRight:
						err = t.conn.Send(protocol.Seek(t.Progress.Current + time.Second*5))

					case tcell.KeyUp:
						t.queueShiftFocus(-1)

					case tcell.KeyDown:
						t.queueShiftFocus(1)

					case tcell.KeyCtrlU, tcell.KeyCtrlB:
						t.queueShiftFocus(-10)

					case tcell.KeyCtrlD, tcell.KeyCtrlF:
						t.queueShiftFocus(10)

					case tcell.KeyPgUp:
						t.queueShiftFocus(-t.queueR.Dy())

					case tcell.KeyPgDn:
						t.queueShiftFocus(t.queueR.Dy())

					case tcell.KeyHome:
						t.queueFocus(0)

					case tcell.KeyEnd:
						t.queueFocus(math.MaxInt)

					case tcell.KeyEnter:
						err = t.conn.Send(protocol.Jump(t.queueFocusIdx))
						t.queueFocus(0)

					case tcell.KeyRune:
						switch ev.Rune() {
						case ' ':
							err = t.conn.Send(protocol.PauseState(!t.Pause))

						case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
							err = t.conn.Send(protocol.Seek(float64(t.Progress.Total) *
								(float64(ev.Rune()-'0') / 10)))

						case 'k':
							t.queueShiftFocus(-1)

						case 'j':
							t.queueShiftFocus(1)

						case 'g':
							t.queueFocus(0)

						case 'G':
							t.queueFocus(math.MaxInt)

						case 'd', 'x':
							err = t.conn.Send(protocol.Remove(t.queueFocusIdx))

						case 'D', 'X':
							err = t.conn.Send(protocol.RemoveAll{})

						case 'l':
							err = t.conn.Send(protocol.Later(t.queueFocusIdx))

						case 'S':
							err = t.conn.Send(protocol.Reshuffle{})

						case 's':
							err = t.conn.Send(!t.Shuffle)

						case 'r':
							err = t.conn.Send(t.Repeat.Next())

						case 'R':
							err = t.conn.Send(t.Repeat.Prev())

						case 'n':
							err = t.conn.Send(protocol.Skip(1))

						case 'N':
							err = t.conn.Send(protocol.Skip(-1))

						case 'i', 'I', '/':
							t.mode = modeInsert
							t.drawMode()
							t.drawQuery()

						case 'p':
							if t.clipboardPath == "" {
								// TODO: surface a warning or error here to
								// notify the user that nothing happened because
								// they don't have anything in the clipboard
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: t.queueFocusIdx + 1,
								Path:  t.clipboardPath,
							})
							t.clipboardPath = ""

						case 'P':
							if t.clipboardPath == "" {
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: t.queueFocusIdx,
								Path:  t.clipboardPath,
							})
							t.clipboardPath = ""

						case 'q', 'Q':
							return nil
						}
					}

				case modeInsert:
					oldQueryString := t.queryString

					switch ev.Key() {
					case tcell.KeyCtrlC, tcell.KeyESC:
						t.mode = modeNormal
						t.drawMode()

					case tcell.KeyEnter, tcell.KeyDown:
						t.mode = modeSelect
						t.drawMode()

					case tcell.KeyHome, tcell.KeyCtrlA:
						t.queryMouseIdx = 0

					case tcell.KeyEnd, tcell.KeyCtrlE:
						t.queryMouseIdx = len(t.queryString)

					case tcell.KeyLeft:
						if ev.Modifiers()&tcell.ModAlt != 0 || ev.Modifiers()&tcell.ModCtrl != 0 {
							endIdx := t.queryMouseIdx
							if endIdx > 0 && t.queryString[endIdx-1] == ' ' {
								endIdx--
							}
							t.queryMouseIdx = strings.LastIndexByte(t.queryString[:endIdx], ' ') + 1
						} else {
							_, size := utf8.DecodeLastRuneInString(t.queryString[:t.queryMouseIdx])
							t.queryMouseIdx -= size
						}

					case tcell.KeyRight:
						if ev.Modifiers()&tcell.ModAlt != 0 || ev.Modifiers()&tcell.ModCtrl != 0 {
							startIdx := t.queryMouseIdx
							if startIdx < len(t.queryString) && t.queryString[startIdx] == ' ' {
								startIdx++
							}
							spaceIdx := strings.IndexByte(t.queryString[startIdx:], ' ')
							if spaceIdx != -1 {
								t.queryMouseIdx = startIdx + spaceIdx
							} else {
								t.queryMouseIdx = len(t.queryString)
							}
						} else {
							_, size := utf8.DecodeRuneInString(t.queryString[t.queryMouseIdx:])
							t.queryMouseIdx += size
						}

					case tcell.KeyCtrlW:
						endIdx := t.queryMouseIdx
						if r, size := utf8.DecodeLastRuneInString(t.queryString[:endIdx]); r == ' ' {
							endIdx -= size
						}
						startIdx := strings.LastIndexByte(t.queryString[:endIdx], ' ') + 1
						t.queryString = t.queryString[:startIdx] + t.queryString[t.queryMouseIdx:]
						t.queryMouseIdx = startIdx

					case tcell.KeyBackspace2:
						_, size := utf8.DecodeLastRuneInString(t.queryString[:t.queryMouseIdx])
						t.queryString = t.queryString[:t.queryMouseIdx-size] + t.queryString[t.queryMouseIdx:]
						t.queryMouseIdx -= size

					case tcell.KeyDelete:
						_, size := utf8.DecodeRuneInString(t.queryString[t.queryMouseIdx:])
						t.queryString = t.queryString[:t.queryMouseIdx] + t.queryString[t.queryMouseIdx+size:]

					case tcell.KeyRune:
						r := ev.Rune()
						t.queryString = strings.Join([]string{
							t.queryString[:t.queryMouseIdx],
							string(r),
							t.queryString[t.queryMouseIdx:],
						}, "")
						t.queryMouseIdx += len(string(r))
					}

					if t.queryString != oldQueryString {
						if t.queryString == "" {
							t.queryResults = nil
						} else {
							err = t.conn.Send(protocol.Query(t.queryString))
						}
					}
					t.drawQuery()

				case modeSelect:
					switch ev.Key() {
					case tcell.KeyCtrlC, tcell.KeyESC:
						t.mode = modeNormal
						t.drawMode()
						t.queryFocusIdx = 0
						t.drawQuery()

					case tcell.KeyUp:
						if t.queryFocusIdx > 0 {
							t.queryShiftFocus(-1)
						} else {
							t.mode = modeInsert
							t.drawMode()
							t.drawQuery()
						}

					case tcell.KeyDown:
						t.queryShiftFocus(1)

					case tcell.KeyCtrlU, tcell.KeyCtrlB:
						t.queryShiftFocus(-10)

					case tcell.KeyCtrlD, tcell.KeyCtrlF:
						t.queryShiftFocus(10)

					case tcell.KeyPgUp:
						t.queryShiftFocus(-t.queryR.Dy() + 1)

					case tcell.KeyPgDn:
						t.queryShiftFocus(t.queryR.Dy() - 1)

					case tcell.KeyHome:
						t.queryFocus(0)

					case tcell.KeyEnd:
						t.queryFocus(math.MaxInt)

					case tcell.KeyEnter:
						if len(t.queryResults) == 0 {
							break
						}

						err = t.conn.Send(protocol.Insert{
							Index: 0,
							Path:  t.queryResults[t.queryFocusIdx],
						})

					case tcell.KeyRune:
						switch ev.Rune() {
						case 'k':
							if t.queryFocusIdx > 0 {
								t.queryShiftFocus(-1)
							} else {
								t.mode = modeInsert
								t.drawMode()
								t.drawQuery()
							}

						case 'j':
							t.queryShiftFocus(1)

						case 'g':
							t.queryFocus(0)

						case 'G':
							t.queryFocus(math.MaxInt)

						case 'i', ' ':
							if len(t.queryResults) == 0 {
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: t.queueFocusIdx,
								Path:  t.queryResults[t.queryFocusIdx],
							})

						case 'I':
							if len(t.queryResults) == 0 {
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: 0,
								Path:  t.queryResults[t.queryFocusIdx],
							})

						case 'a':
							if len(t.queryResults) == 0 {
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: t.queueFocusIdx + 1,
								Path:  t.queryResults[t.queryFocusIdx],
							})

						case 'A':
							if len(t.queryResults) == 0 {
								break
							}

							err = t.conn.Send(protocol.Insert{
								Index: len(t.Queue),
								Path:  t.queryResults[t.queryFocusIdx],
							})
						}
					}

				default:
					panic(fmt.Sprintf(`invalid mode "%d"`, t.mode))
				}

				if err != nil {
					return err
				}

			default:
				t.logger.Printf("unhandled event type: %T", ev)
			}
		}

		if !t.screen.HasPendingEvent() {
			t.screen.Show()
		}
	}
}
