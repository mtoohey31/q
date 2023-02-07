package server

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/query"
	"mtoohey.com/q/internal/server/channelconn"
	"mtoohey.com/q/internal/server/unixsocketconn"
	"mtoohey.com/q/internal/track"

	"github.com/faiface/beep"
)

type Server struct {
	// constants
	cmd.Globals
	logger *log.Logger

	// state
	// pausedMu protects pause. speaker also needs to be locked when we modify
	// pause, but there are cases (such as when broadcasting an updated status)
	// where we want to read paused without locking the speaker, so we have
	// this too...
	pausedMu sync.RWMutex
	paused   protocol.PauseState
	repeat   atomic.Value
	shuffle  atomic.Bool
	// queueMu protects shuffleIdx and queue.
	queueMu sync.RWMutex
	// shuffleIdx is the index within the queue of the first song that was
	// played during this repeat of the queue. It may be 0 if no songs have yet
	// been finished on this repeat.
	shuffleIdx protocol.ShuffleIdxState
	queue      []*track.Track

	// resources
	// streamerMu protects format and streamer. Reads from, seeks of, and
	// reassignments of streamer also require the speaker to be locked.
	streamerMu sync.RWMutex
	streamer   beep.StreamSeekCloser
	format     beep.Format

	channelListener *channelconn.ChannelListener
	listeners       []protocol.Listener
	// clientsMu protects clients.
	clientsMu sync.RWMutex
	clients   []protocol.Conn
	// disconnected receives clients that have disconnected and should be
	// removed.
	disconnected chan protocol.Conn
	// closedMu protects closed.
	closedMu sync.Mutex
	// closed is nil initially, then open while we're serving, then closed
	// once Close is called
	closed chan struct{}
}

// NewServer creates a new server.
func NewServer(cmd Cmd, g cmd.Globals, logger *log.Logger) (*Server, error) {
	// This function directly accesses a lot of stuff that usually needs to
	// have a mutex locked, or be accessed in an atomic way. This is fine
	// because the server can only be accessed by the single routine that this
	// function is running in, so there is no danger of races or other issues.

	s := &Server{
		Globals: g,
		logger:  logger,
		paused:  false,
	}
	s.repeat.Store(cmd.Repeat)
	s.shuffle.Store(bool(cmd.Shuffle))

	pathSet := map[string]struct{}{}
	s.queue = []*track.Track{}
	for _, q := range cmd.InitialQueries {
		newPaths, err := query.Query(s.MusicDir, q)
		if err != nil {
			return nil, fmt.Errorf(`failed to execute query "%s": %w`, q, err)
		}

		for _, p := range newPaths {
			if _, ok := pathSet[p]; !ok {
				s.queue = append(s.queue, &track.Track{Path: p})
				pathSet[p] = struct{}{}
			}
		}
	}
	if cmd.Shuffle {
		shuffle(s.queue)
	}
	s.shuffleIdx = 0
	s.playQueueTopLocked()

	s.channelListener = channelconn.NewChannelListener()
	s.listeners = []protocol.Listener{s.channelListener}

	if s.UnixSocket != "" {
		s.listeners = append(s.listeners,
			&unixsocketconn.UnixSocketListener{
				SocketPath: s.UnixSocket,
			})
	}

	return s, nil
}

func (s *Server) Close() error {
	s.logger.Println("shutting down")

	s.closedMu.Lock()
	if s.closed == nil {
		s.closedMu.Unlock()
		return nil
	}
	select {
	case <-s.closed:
	default:
		close(s.closed)
	}
	s.closedMu.Unlock()

	return nil
}

// Stream requires speaker to be locked, which beep will do automatically.
func (s *Server) Stream(samples [][2]float64) (n int, ok bool) {
	s.streamerMu.Lock()
	n, ok = s.streamLocked(samples)
	s.streamerMu.Unlock()
	return
}

// streamLocked requires speaker and streamerMu to be locked.
func (s *Server) streamLocked(samples [][2]float64) (n int, ok bool) {
	silenceFrom := 0

	s.pausedMu.RLock()
	paused := s.paused
	s.pausedMu.RUnlock()

	if !paused && s.streamer != nil {
		var ok bool
		var err error

		silenceFrom, ok = s.streamer.Stream(samples)
		if !ok {
			// if the streamer failed, warn and set err so that we'll skip
			// below
			if err = s.streamer.Err(); err != nil {
				s.broadcastErr(fmt.Errorf("streamer failed: %w", err))
			}
		}

		// if the streamer didn't fill samples, try skipping to the next song
		if silenceFrom < len(samples) {
			// if we ran into an error with the current streamer, drop it from
			// the queue, regardless of the repeat setting
			s.queueMu.Lock()
			s.skipLocked(err != nil)
			s.queueMu.Unlock()

			// recursively continue streaming after the skip to avoid silence,
			// if there's no now-playing song after the skip, the recurisve
			// call will realize this and fill the rest of samples with silence
			n, _ := s.streamLocked(samples[silenceFrom:])
			silenceFrom += n
		}
	}

	for i := silenceFrom; i < len(samples); i++ {
		samples[i] = [2]float64{}
	}

	return len(samples), true
}

func (*Server) Err() error {
	// we never want the server to be drained from the speaker causing its
	// playback to end, so we always return nil here, just as we always return
	// true in (*server).Stream.

	return nil
}

func (s *Server) decShuffleIdxLocked() {
	newIdx := s.shuffleIdx - 1
	if newIdx < 0 {
		newIdx = protocol.ShuffleIdxState(len(s.queue)) - 1
	}
	s.shuffleIdx = newIdx
}

// skipLocked moves to the next song, if there is one. speaker, queue, and
// streamer should be locked before a call to this method.
//
// drop indicates whether the previous song should be forcibly dropped from the
// queue (such as in the case of an error reading from the current streamer).
func (s *Server) skipLocked(drop bool) {
	// if there's nothing in the queue, there's nothing we can do regardless of
	// any other states.
	if len(s.queue) == 0 {
		return
	}

	// load repeat once and use it for the rest of the function
	repeat := s.repeat.Load().(protocol.RepeatState)

	// if we're not required to drop the current song, and if we're repeating
	// either the current track, or the whole queue but there's only one song
	// in the queue...
	if !drop && (repeat == protocol.RepeatStateTrack ||
		(repeat == protocol.RepeatStateQueue && len(s.queue) == 1)) {

		if err := s.streamer.Seek(0); err != nil {
			s.broadcastErr(fmt.Errorf("seek failed: %w", err))
			s.skipLocked(true)
			return
		}

		s.broadcast(s.getProgressLocked())

		return
	}

	// if we make it this far, the queue is going to change unless there are
	// errors

	if drop || repeat == protocol.RepeatStateNone {
		s.queue = s.queue[1:]
	} else if s.shuffle.Load() {
		// !drop && s.repeat == protocol.RepeatStateQueue since both drop and
		// protocol.RepeatStateNone are handled immediately above, and
		// protocol.RepeatStateTrack is handled further above.

		// len(s.queue) >= 2 also because the 0 case is handled at the very top
		// and the 1 case is handled in by that further above case.

		var insertIdx int
		switch s.shuffleIdx {
		case 0, 1:
			// special case: we should try to avoid putting the current track
			// next and playing it twice in a row if shuffleIdx is 0 or 1, and
			// we also have to ensure in the case where it's 0 that insertIdx
			// won't be 0 leading to invalid indexing below

			insertIdx = rand(2, len(s.queue)+1)
		default:
			insertIdx = rand(int(s.shuffleIdx), len(s.queue)+1)
		}

		s.queue = append(s.queue[1:insertIdx], append([]*track.Track{s.queue[0]}, s.queue[insertIdx:]...)...)
	} else {
		// !s.shuffle

		s.queue = append(s.queue[1:], s.queue[0])
	}
	s.decShuffleIdxLocked()

	s.playQueueTopLocked()

	s.broadcastQueue()
}

// playQueueTopLocked closes the current streamer, then begins decoding the
// item at s.queue[0], if one exists. If len(s.queue) == 0 the current streamer
// is assigned to nil. speaker, queue, and streamer should be locked before
// this method is called.
func (s *Server) playQueueTopLocked() {
	if s.streamer != nil {
		if err := s.streamer.Close(); err != nil {
			s.broadcastErr(fmt.Errorf("failed to close previous streamer: %w", err))
		}
	}

	if len(s.queue) == 0 {
		s.streamer = nil
		s.format = beep.Format{}
		s.broadcastNowPlayingLocked()

		return
	}

	var streamer beep.StreamSeekCloser
	var err error
	streamer, s.format, err = s.queue[0].Decode()
	if err != nil {
		s.streamer, s.format = nil, beep.Format{}
		s.broadcastErr(fmt.Errorf("failed to decode queue[0]: %w", err))
		s.skipLocked(true)
		return
	}
	if s.format.SampleRate == s.SampleRate {
		// if the raw streamer's sample rate is equal to the current sample
		// rate, just use it directly without resampling
		s.streamer = streamer
	} else {
		s.streamer = resampleSeekCloser(s.format.SampleRate, s.SampleRate, streamer)
	}

	s.broadcastNowPlayingLocked()
}
