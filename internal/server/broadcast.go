package server

import (
	"errors"
	"fmt"
	"net"

	"mtoohey.com/q/internal/protocol"
)

// broadcast sends the given message out to all clients.
func (s *Server) broadcast(m protocol.Message) {
	// we fetch clients once before the range expression begins, so even
	// if s.clients gets modified by someone while this is
	// happening, we'll only hit each client once. we could accidentally
	// send to a closed client, but that should return something that "Is"
	// net.ErrClosed, so we'll just end up logging the disconnect an extra
	// time. we might also miss sending the message to a recently added client,
	// but that's inevitable. an issue could potentially occur if we are adding
	// a client...

	s.clientsMu.RLock()
	clients := s.clients
	s.clientsMu.RUnlock()

	if len(clients) == 0 {
		// avoid the log message if we're not actually sending to anyone
		return
	}

	s.logger.Printf("broadcasting message: %#v", m)

	// BUG: if we are adding a client, and we send the current state before
	// adding it to s.clients (which is necessary because we need to ensure
	// that the initial message is sent first), the state may change between
	// when we fetch it for the initial message, and when the client gets added
	// to s.clients. if this happens, the client will have innacurate state
	// information. can we maybe fetch the initial state while signalling that
	// we need to record messages that the client will miss while it hasn't
	// been added to s.clients? the alternative is blocking all broadcasts
	// until after we've sent the first message, but that's not an option

	for _, client := range clients {
		if err := client.Send(m); err != nil {
			if errors.Is(err, net.ErrClosed) {
				s.logger.Printf("client disconnected: %s", client)
			} else {
				s.logger.Printf("send error for %s: %s", client, err)

				// try to close though ignore the error cause the client is
				// already broken, so there's a good chance this won't succeed
				_ = client.Close()
			}

			// drop the client
			s.disconnected <- client
		}
	}
}

// broadcastErr sends the given error to all clients.
func (s *Server) broadcastErr(err error) {
	s.logger.Println(err)
	s.broadcast(protocol.Error(err.Error()))
}

// getNowPlayingLocked returns the current now playing state. If errors are
// encountered, they will be broadcasted, but the most complete possible value
// will still be returned. queue should be locked.
func (s *Server) getNowPlayingLocked() protocol.NowPlayingState {
	if len(s.queue) == 0 {
		return protocol.NowPlayingState{}
	}

	title, err := s.queue[0].Title()
	if err != nil {
		s.broadcastErr(fmt.Errorf("failed to get queue[0] title: %w", err))
	}

	artist, err := s.queue[0].Artist()
	if err != nil {
		s.broadcastErr(fmt.Errorf("failed to get queue[0] artist: %w", err))
	}

	cover, err := s.queue[0].Cover()
	if err != nil {
		s.broadcastErr(fmt.Errorf("failed to get queue[0] cover: %w", err))
	}

	return protocol.NowPlayingState{
		Title:  title,
		Artist: artist,
		Cover:  cover,
	}
}

// broadcastNowPlayingLocked sends the now playing state to all clients.
// streamer and queue should be locked.
func (s *Server) broadcastNowPlayingLocked() {
	s.broadcast(s.getProgressLocked())
	s.broadcast(s.getNowPlayingLocked())
}

// getProgress retrieves the current progress.
func (s *Server) getProgress() protocol.ProgressState {
	s.streamerMu.RLock()
	p := s.getProgressLocked()
	s.streamerMu.RUnlock()
	return p
}

// getProgressLocked retrieves the current progress. streamer must be RLocked.
func (s *Server) getProgressLocked() protocol.ProgressState {
	if s.streamer == nil {
		return protocol.ProgressState{}
	}

	return protocol.ProgressState{
		Current: s.format.SampleRate.D(s.streamer.Position()),
		Total:   s.format.SampleRate.D(s.streamer.Len()),
	}
}

// broadcastProgress sends the now playing state to all clients.
func (s *Server) broadcastProgress() {
	s.broadcast(s.getProgress())
}

// getQueue retrieves the current QueueState.
func (s *Server) getQueue() protocol.QueueState {
	qs := make(protocol.QueueState, len(s.queue))
	for i, track := range s.queue {
		description, err := track.Description()
		if err != nil {
			s.broadcastErr(fmt.Errorf("failed to get queue[%d] description: %w", i, err))
			continue
		}
		qs[i] = protocol.QueueItem(description)
	}
	return qs
}

// broadcastQueue sends the current queue to all clients.
func (s *Server) broadcastQueue() {
	s.broadcast(s.getQueue())
}

// getQueue retrieves the current State.
func (s *Server) getState() protocol.State {
	s.pausedMu.RLock()
	paused := s.paused
	s.pausedMu.RUnlock()

	s.queueMu.RLock()
	nowPlaying := s.getNowPlayingLocked()
	s.queueMu.RUnlock()

	return protocol.State{
		NowPlaying: nowPlaying,
		Pause:      paused,
		Progress:   s.getProgress(),
		Repeat:     s.repeat.Load().(protocol.RepeatState),
		Shuffle:    protocol.ShuffleState(s.shuffle.Load()),
		Queue:      s.getQueue(),
		Version:    protocol.Version,
	}
}
