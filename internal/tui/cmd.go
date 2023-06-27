package tui

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"sync"

	"mtoohey.com/q/internal/cmd"
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
	ServerLogPath *string `short:"l" help:"The path of the file the server's logs should be output to if an internal server must be started."`
	// TUILogPath is the path of the file the TUI's logs should be output to.
	TUILogPath *string `short:"L" help:"The path of a file the TUI's logs should be output to."`
}

func (c Cmd) Run(g cmd.Globals) (err error) {
	var conn protocol.Conn
	var wg sync.WaitGroup
	defer wg.Wait()

	// start out assuming we will have to start our own server
	startServer := true
	if g.UnixSocket != nil {
		_, err = os.Stat(*g.UnixSocket)
		if err == nil {
			// only change this assumption if the unix socket flag was
			// provided and the socket already exists
			startServer = false
		} else if !errors.Is(err, os.ErrNotExist) {
			// we got an unexpected error, bail
			return fmt.Errorf("unexpected error while checking whether socket exists: %w", err)
		}
	}

	var serverLogger *log.Logger
	if startServer {
		out := io.Discard
		if c.ServerLogPath != nil {
			f, err := os.OpenFile(*c.ServerLogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
			if err != nil {
				return fmt.Errorf("failed to create server log file: %w", err)
			}
			out = f
		}
		serverLogger = log.New(out, "", log.LstdFlags)

		var s *server.Server
		s, err = server.NewServer(c.Cmd, g, serverLogger)
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

		channelConn := s.ChannelConn()
		if channelConn == nil {
			// the server encountered an error while starting up and is no
			// longer listening for channel connections; we should return so
			// that the serveErr will get surfaced to the user
			return nil
		}

		conn = channelConn
	} else {
		// try to connect to the existing socket
		conn, err = unixsocketconn.NewUnixSocketClientConn(*g.UnixSocket)
		if err != nil {
			return fmt.Errorf("failed to connect to existing socket: %w", err)
		}
	}

	var tuiLogger *log.Logger
	if c.ServerLogPath == c.TUILogPath && serverLogger != nil {
		tuiLogger = serverLogger
	} else if c.TUILogPath != nil {
		f, err := os.OpenFile(*c.TUILogPath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			// This is our responsibility, but ignore the error because we
			// already have an error to report.
			_ = conn.Close()
			return fmt.Errorf("failed to create tui log file: %w", err)
		}
		tuiLogger = log.New(f, "", log.LstdFlags)
	} else {
		tuiLogger = log.New(io.Discard, "", log.LstdFlags)
	}

	tui, err := newTUI(c, g, tuiLogger, conn)
	if err != nil {
		// Same as above.
		_ = conn.Close()
		return fmt.Errorf("failed to create tui: %w", err)
	}

	err = tui.loop()
	return
}
