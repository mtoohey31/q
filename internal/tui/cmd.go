package tui

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"

	"mtoohey.com/q/internal/protocol"
	"mtoohey.com/q/internal/server"
	"mtoohey.com/q/internal/server/unixsocketconn"
)

type Cmd struct {
	server.Cmd
	// ScrollOff is the lines of padding from the cursor to the edge of the
	// screen when scrolling.
	ScrollOff int `short:"o" default:"7" help:"Lines of padding from the cursor to the edge of the screen when scrolling."`
	// ServerLogPath is the path of the file the server's logs should be output
	// to if an internal server must be started.
	ServerLogPath string `short:"l" type:"path" help:"The path of the file the server's logs should be output to if an internal server must be started."`
}

func (c Cmd) Run() (err error) {
	var conn protocol.Conn
	var wg sync.WaitGroup
	defer wg.Wait()

	if c.UnixSocket != "" {
		_, err = os.Stat(c.UnixSocket)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// we got an unexpected error, bail
			return fmt.Errorf("unexpected error while checking whether socket exists: %w", err)
		}

		if err != nil {
			// then errors.Is(err, os.ErrNotExist), since otherwise we would've
			// hit the first condition. in this case, the socket doesn't exist,
			// so we should create our own server which will bind to it
			out := io.Discard
			if c.ServerLogPath != "" {
				f, err := os.OpenFile(c.ServerLogPath, os.O_RDWR|os.O_CREATE, 0o600)
				if err != nil {
					return fmt.Errorf("failed to create server log file: %w", err)
				}
				out = f
			}

			s, err := server.NewServer(c.Cmd, log.New(out, "", log.LstdFlags))
			if err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}

			// make sure we only close once
			var closeOnce sync.Once
			doClose := func() {
				closeErr := s.Close()
				if err == nil {
					err = closeErr
				}
			}
			defer closeOnce.Do(doClose)

			// ensure the server's resources get cleaned up even if we get killed
			sigCh := make(chan os.Signal, 2)
			go func() {
				<-sigCh
				closeOnce.Do(doClose)
			}()
			signal.Notify(sigCh, os.Interrupt)
			defer signal.Stop(sigCh)

			// start the server
			wg.Add(1)
			go func() {
				defer wg.Done()
				serveErr := s.Serve()

				if err == nil {
					err = serveErr
				}
			}()

			conn = s.ChannelConn()
		} else {
			// then the socket already exists, so let's try and connect to it

			conn, err = unixsocketconn.NewUnixSocketClientConn(c.UnixSocket)
			if err != nil {
				return fmt.Errorf("failed to connect to existing socket: %w", err)
			}
		}
	}

	tui, err := newTUI(c, conn)
	if err != nil {
		// this is our responsibility, but ignore the error because we already
		// have an error to report
		_ = conn.Close()
		return fmt.Errorf("failed to create tui: %w", err)
	}

	err = tui.loop()
	return
}
