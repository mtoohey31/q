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
	s.logger.Printf("recieved message of type %T: %#v", m, m)

	switch m := m.(type) {
	case protocol.PauseState:
		speaker.Lock()
		s.paused = m
		speaker.Unlock()
		s.broadcast(s.paused)

	case protocol.RepeatState:
		s.repeat.Store(m)
		s.broadcast(s.repeat)

	case protocol.ShuffleState:
		s.shuffle.Store(bool(m))
		s.broadcast(m)

	case protocol.Skip:
		if m < 0 {
			respond(protocol.Error("reverse skip not implemented"))
			break
		}

		speaker.Lock()
		// special case: when skipping zero places, we jump to the start of the
		// now playing song
		if m == 0 {
			if err := s.streamer.Seek(util.Clamp(
				0,
				s.format.SampleRate.N(time.Duration(m)),
				s.streamer.Len(),
			)); err != nil {
				s.broadcastErr(fmt.Errorf("seek failed: %w", err))
				s.queueMu.Lock()
				s.skipLocked(true)
				s.queueMu.Unlock()
			}
			s.broadcastProgress()
			speaker.Unlock()
			break
		}

		// TODO: implement this in a less horrible, terrible, very bad, no good
		// way
		s.queueMu.Lock()
		for i := 0; i < int(m); i++ {
			s.skipLocked(false) // broadcasts new now playing... each time...
		}
		s.queueMu.Unlock()
		speaker.Unlock()

	case protocol.Seek:
		speaker.Lock()
		if err := s.streamer.Seek(util.Clamp(
			0,
			s.format.SampleRate.N(time.Duration(m)),
			s.streamer.Len(),
		)); err != nil {
			s.broadcastErr(fmt.Errorf("seek failed: %w", err))
			s.queueMu.Lock()
			s.skipLocked(true)
			s.queueMu.Unlock()
		}
		s.broadcastProgress()
		speaker.Unlock()

	case protocol.Remove:
		s.queueMu.Lock()

		removeIdx := int(m)
		if removeIdx >= len(s.queue) || 0 > removeIdx {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for remove request: %d", removeIdx)))
			return
		}

		s.queue = append(s.queue[:removeIdx], s.queue[removeIdx+1:]...)
		if removeIdx < s.shuffleIdx {
			s.decShuffleIdxLocked()
		}

		if removeIdx == 0 {
			speaker.Lock()
			s.playQueueTopLocked() // broadcasts new now playing
			speaker.Unlock()
		}

		s.queueMu.Unlock()

		s.broadcastQueue()

	case protocol.RemoveAll:
		s.queueMu.Lock()

		s.queue = nil
		s.shuffleIdx = 0

		speaker.Lock()
		s.playQueueTopLocked() // broadcasts new now playing
		speaker.Unlock()

		s.queueMu.Unlock()

		s.broadcastQueue()

	case protocol.Insert:
		s.queueMu.Lock()

		if m.Index > len(s.queue) || 0 > m.Index {
			s.queueMu.Unlock()
			respond(protocol.Error(fmt.Sprintf("invalid index for insert request: %d", m.Index)))
			return
		}

		s.queue = append(s.queue[:m.Index], append([]*track.Track{{Path: m.Path}}, s.queue[m.Index:]...)...)
		if m.Index <= s.shuffleIdx {
			s.shuffleIdx++
		}

		if m.Index == 0 {
			speaker.Lock()
			s.playQueueTopLocked() // broadcasts new now playing
			speaker.Unlock()
		}

		s.queueMu.Unlock()

		s.broadcastQueue()

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

		if len(s.queue) <= 2 {
			// we don't re-shuffle the now playing song, so we're shuffling
			//s.queue[1:], and if that's only 1 long, it's not going to have
			// any effect

			return
		}

		shuffle(s.queue[1:])

		s.queueMu.Unlock()

		s.broadcastQueue()

	default:
		respond(protocol.Error(fmt.Sprintf("invalid request type: %T", m)))
	}

	return
}
