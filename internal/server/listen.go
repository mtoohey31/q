package server

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/server/channelconn"

	"github.com/faiface/beep/speaker"
)

// ChannelConn returns a channel connection to this server. It should be called
// after Serve has begun, and may return nil if serve encounters an error while
// starting up.
func (s *Server) ChannelConn() *channelconn.ChannelConn {
	return s.channelListener.Conn()
}

func (s *Server) Serve() (err error) {
	s.logger.Println("beginning serve")

	s.closedMu.Lock()
	if s.closed != nil {
		s.closedMu.Unlock()
		return fmt.Errorf("serve already called")
	}
	s.closed = make(chan struct{})
	s.closedMu.Unlock()

	defer s.logger.Println("finished serve")

	if err := speaker.Init(s.SampleRate, s.SampleRate.N(time.Millisecond*10)); err != nil {
		close(s.closed)
		return fmt.Errorf("failed to initialize speaker: %w", err)
	}
	defer speaker.Close()

	speaker.Play(s)

	s.logger.Println("starting playback")

	clientCh := make(chan protocol.Conn)
	// buffered for the number of listeners so that the final accept errors
	// don't deadlock things
	acceptErrCh := make(chan error, len(s.listeners))

	s.disconnected = make(chan protocol.Conn)

	// wg for everything spawned by this function to make sure that everything
	// has exited before we return
	var wg sync.WaitGroup
	defer s.logger.Println("all goroutines exited")
	defer wg.Wait()
	defer s.logger.Println("waiting for goroutines to exit")

	s.logger.Println("starting listeners")

	// listener accept routines
	for _, l := range s.listeners {
		s.logger.Printf("starting %s", l)

		if err := l.Listen(); err != nil {
			close(s.closed)
			return fmt.Errorf("listen failed: %w", err)
		}

		s.logger.Printf("started %s", l)

		defer func(l protocol.Listener) {
			closeErr := l.Close()

			if err == nil {
				err = closeErr
			} else {
				s.logger.Printf("failed to close %s: %s", l, err)
			}
		}(l)

		wg.Add(1)
		go func(l protocol.Listener) {
			defer wg.Done()
			for {
				conn, err := l.Accept()
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						select {
						case <-s.closed:
							// return without sending error, this is expected
							return

						default:
						}
					}

					acceptErrCh <- err
					return
				}

				clientCh <- conn
			}
		}(l)
	}

	s.logger.Println("listeners started")

	// progress broadcast routine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-time.After(time.Second):
			case <-s.closed:
				return
			}

			if s.streamer != nil && !s.paused {
				s.broadcast(s.getProgress())
			}
		}
	}()

	s.logger.Println("starting accept loop")

	for {
		var c protocol.Conn

		select {
		case c = <-clientCh:
			// continue to below

		case err := <-acceptErrCh:
			close(s.closed)
			return err

		case c = <-s.disconnected:
			s.clientsMu.Lock()
			// this pointer comparison loop is necessary instead of just
			// accepting the index because the index may change between when it
			// gets sent down the channel and when it gets received here
			found := false
			for i, oc := range s.clients {
				if c == oc {
					s.clients = append(s.clients[:i], s.clients[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				s.logger.Printf("couldn't find disconnected client %s", c)
			}
			s.clientsMu.Unlock()
			continue

		case <-s.closed:
			s.clientsMu.Lock()
			defer s.clientsMu.Unlock()

			var firstErr error
			for _, client := range s.clients {
				if err := client.Close(); err != nil {
					if firstErr == nil {
						firstErr = err
					} else {
						s.logger.Printf("failed to close %s: %s", client, err)
					}
				}
			}
			return firstErr
		}

		s.logger.Printf("accepted connection from: %s", c)

		// move things into a goroutine immediately so we can keep accepting
		wg.Add(1)
		go func(c protocol.Conn) {
			defer wg.Done()
			// send initial state first before adding to s.clients so that
			// this is always the first message that a client gets
			if err := c.Send(s.getState()); err != nil {
				if errors.Is(err, net.ErrClosed) {
					s.logger.Printf("client %s disconnected before recieving initial message: %s", c, err)
				} else {
					s.logger.Printf("failed to send initial message to client %s: %s", c, err)

					// try to close though ignore the error cause the
					// connection is already broken, so there's a good chance
					// this won't succeed
					_ = c.Close()
				}

				// return without adding to the clients list
				return
			}

			s.clientsMu.Lock()
			s.clients = append(s.clients, c)
			s.clientsMu.Unlock()

			for {
				m, err := c.Receive()
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						s.logger.Printf("client disconnected: %s", c)
					} else {
						s.logger.Printf("receive error for %s: %s", c, err)

						// try to close though ignore the error cause the client is
						// already broken, so there's a good chance this won't succeed
						_ = c.Close()
					}

					// drop the client
					s.disconnected <- c
					return
				}

				err = nil
				s.handle(m, func(m protocol.Message) {
					s.logger.Printf("responding with message: %#v", m)
					err = c.Send(m)
				})
				if err != nil {
					if errors.Is(err, net.ErrClosed) {
						s.logger.Printf("client disconnected: %s", c)
					} else {
						s.logger.Printf("send error for %s: %s", c, err)

						// try to close though ignore the error cause the client is
						// already broken, so there's a good chance this won't succeed
						_ = c.Close()
					}

					// drop the client
					s.disconnected <- c
					return
				}
			}
		}(c)
	}
}
