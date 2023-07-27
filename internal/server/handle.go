package server

import (
	"fmt"
	"time"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/query"
	"mtoohey.com/q/internal/track"
	"mtoohey.com/q/internal/util"

	"github.com/faiface/beep/speaker"
)

// handle handles a single incoming message.
func (s *Server) handle(m protocol.Message, respond func(protocol.Message)) {
	s.logger.Printf("received message of type %T: %#v", m, m)

	switch m := m.(type) {
	case protocol.PauseState:
		speaker.Lock()
		s.pausedMu.Lock()
		s.paused = m
		s.pausedMu.Unlock()
		speaker.Unlock()

		s.broadcast(m)

	case protocol.RepeatState:
		s.queueMu.Lock()
		s.queue.Repeat = m
		s.queueMu.Unlock()

		s.broadcast(m)

	case protocol.ShuffleState:
		s.queueMu.Lock()
		s.queue.Shuffle = m
		s.queueMu.Unlock()

		s.broadcast(m)

	case protocol.Skip:
		speaker.Lock()
		s.queueMu.Lock()
		s.streamerMu.Lock()

		s.skipLocked(int(m))

		s.streamerMu.Unlock()
		s.queueMu.Unlock()
		speaker.Unlock()

	case protocol.Seek:
		speaker.Lock()
		s.streamerMu.Lock()
		if err := s.streamer.Seek(util.Clamp(
			0,
			s.format.SampleRate.N(time.Duration(m)),
			s.streamer.Len()-1,
		)); err != nil {
			s.broadcastErr(fmt.Errorf("seek failed: %w", err))
			s.queueMu.Lock()
			s.dropTopLocked()
			s.queueMu.Unlock()
		}
		s.streamerMu.Unlock()
		speaker.Unlock()
		// must come after streamerMu.unlock because this needs to RLock
		// streamerMu.
		s.broadcastProgress()

	case protocol.Remove:
		s.queueMu.Lock()
		removed, ok := s.queue.Remove(uint(m))
		if !ok {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for remove request: %d", m)))
			return
		}

		if m == 0 {
			speaker.Lock()
			s.streamerMu.Lock()
			s.playQueueTopLocked() // broadcasts new now playing
			s.streamerMu.Unlock()
			speaker.Unlock()
		}

		newQueue := s.getQueueLocked()
		s.queueMu.Unlock()

		go s.broadcast(newQueue)
		go respond(protocol.Removed(removed.Path))

	case protocol.RemoveAll:
		s.queueMu.Lock()
		s.queue.Clear()

		speaker.Lock()
		s.streamerMu.Lock()
		s.playQueueTopLocked() // broadcasts new now playing
		s.streamerMu.Unlock()
		speaker.Unlock()

		s.queueMu.Unlock()

		s.broadcast(protocol.QueueState{})

	case protocol.Insert:
		s.queueMu.Lock()
		if !s.queue.Insert(&track.Track{Path: m.Path}, uint(m.Index)) {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for insert request: %d", m.Index)))
			return
		}

		if m.Index == 0 {
			speaker.Lock()
			s.streamerMu.Lock()
			s.playQueueTopLocked() // broadcasts new now playing
			s.streamerMu.Unlock()
			speaker.Unlock()
		}

		newQueue := s.getQueueLocked()
		s.queueMu.Unlock()

		s.broadcast(newQueue)

	case protocol.Query:
		paths, err := query.Query(s.MusicDir, string(m))

		var resp any
		if err != nil {
			resp = protocol.Error(fmt.Sprintf("failed to execute query: %s", err))
		} else {
			resp = protocol.QueryResults(paths)
		}
		respond(resp)

	case protocol.Reshuffle:
		s.queueMu.Lock()

		if s.queue.Len() <= 2 {
			// we don't re-shuffle the now playing song, so we're shuffling
			//s.queue[1:], and if that's only 1 long, it's not going to have
			// any effect

			return
		}

		s.queue.ReshuffleAfterHead()

		newQueue := s.getQueueLocked()
		s.queueMu.Unlock()

		s.broadcast(newQueue)

	case protocol.Later:
		s.queueMu.Lock()

		if !s.queue.Later(uint(m)) {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for later request: %d", m)))
			return
		}

		if m == 0 {
			speaker.Lock()
			s.streamerMu.Lock()
			s.playQueueTopLocked() // broadcasts new now playing
			s.streamerMu.Unlock()
			speaker.Unlock()
		}

		newQueue := s.getQueueLocked()
		s.queueMu.Unlock()

		go s.broadcast(newQueue)

	case protocol.Jump:
		s.queueMu.Lock()

		if uint(m) >= s.queue.Len() {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for jump request: %d", m)))
			return
		}

		oldShuffle := s.queue.Shuffle
		s.queue.Shuffle = false
		s.queue.Skip(int(m))
		s.queue.Shuffle = oldShuffle

		speaker.Lock()
		s.streamerMu.Lock()
		s.playQueueTopLocked() // broadcasts new now playing
		s.streamerMu.Unlock()
		speaker.Unlock()

		newQueue := s.getQueueLocked()
		s.queueMu.Unlock()

		go s.broadcast(newQueue)

	default:
		respond(protocol.Error(fmt.Sprintf("invalid request type: %T", m)))
	}
}
