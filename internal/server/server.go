package server

import (
	"fmt"
	"log"
	"sync"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/query"
	"mtoohey.com/q/internal/server/channelconn"
	"mtoohey.com/q/internal/server/queue"
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
	// queueMu protects queue.
	queueMu sync.RWMutex
	// shuffleIdx is the index within the queue of the first song that was
	// played during this repeat of the queue. It may be 0 if no songs have yet
	// been finished on this repeat.
	queue queue.Queue[*track.Track]

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

	pathSet := map[string]struct{}{}
	trackList := []*track.Track{}
	for _, q := range cmd.InitialQueries {
		newPaths, err := query.Query(s.MusicDir, q)
		if err != nil {
			return nil, fmt.Errorf(`failed to execute query "%s": %w`, q, err)
		}

		for _, p := range newPaths {
			if _, ok := pathSet[p]; !ok {
				trackList = append(trackList, &track.Track{Path: p})
				pathSet[p] = struct{}{}
			}
		}
	}
	s.queue = queue.QueueFrom(trackList)
	if cmd.Shuffle {
		s.queue.Reshuffle()
	}
	s.queue.Repeat = cmd.Repeat
	s.queue.Shuffle = cmd.Shuffle
	s.playQueueTopLocked()

	s.channelListener = channelconn.NewChannelListener()
	s.listeners = []protocol.Listener{s.channelListener}

	if s.UnixSocket != nil {
		s.listeners = append(s.listeners,
			&unixsocketconn.UnixSocketListener{
				SocketPath: *s.UnixSocket,
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
			err = s.streamer.Err()
			if err != nil {
				go s.broadcastErr(fmt.Errorf("streamer failed: %w", err))
			}
		}

		// if the streamer didn't fill samples, try skipping to the next song
		if silenceFrom < len(samples) {
			// if we ran into an error with the current streamer, drop it from
			// the queue, regardless of the repeat setting
			s.queueMu.Lock()
			if err != nil {
				s.dropTopLocked()
			} else {
				s.skipLocked(1)
			}
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

// dropTopLocked removes the song currently at the top of the queue, and moves
// to the next one, if one exists. speaker, queue, and streamer should all be
// locked before a call to this method.
func (s *Server) dropTopLocked() {
	_, ok := s.queue.Remove(0)
	if !ok {
		return
	}

	s.playQueueTopLocked()
	go s.broadcast(s.getQueueLocked())
}

// skipLocked moves to the next song, if there is one. speaker, queue, and
// streamer should be locked before a call to this method.
func (s *Server) skipLocked(n int) {
	s.queue.Skip(n)

	s.playQueueTopLocked()
	go s.broadcast(s.getQueueLocked())
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

	head, ok := s.queue.Head()
	if !ok {
		s.streamer = nil
		s.format = beep.Format{}
		s.broadcastNowPlayingLocked()

		return
	}

	var streamer beep.StreamSeekCloser
	var err error
	streamer, s.format, err = head.Decode()
	if err != nil {
		s.streamer, s.format = nil, beep.Format{}
		s.broadcastErr(fmt.Errorf("failed to decode queue[0]: %w", err))
		s.dropTopLocked() // recursively calls playQueueTopLocked after dropping
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
