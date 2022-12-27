package server

import (
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/faiface/beep"
	"mtoohey.com/q/internal/protocol"
)

// Constants contains constant values that cannot be changed while the server
// is running.
type Constants struct {
	// SampleRate is the Sample rate to use for the player as a whole. Audio
	// files with different sample rates will be resampled.
	SampleRate beep.SampleRate `short:"t" default:"44100" help:"Sample rate to use for the player as a whole. Audio files with different sample rates will be resampled."`
	// MusicDir is the directory containing music files.
	MusicDir string `short:"m" default:"." type:"path" help:"Directory containing music files."`
	// UnixSocket is the path of the socket to bind to. No socket is used if
	// this flag is not provided.
	UnixSocket string `short:"u" help:"Path of the unix socket to bind to. No socket is used if this flag is not provided."`
}

// Cmd contains all possible options for a server.
type Cmd struct {
	// Constants contains constant values.
	Constants

	// Shuffle is the initial shuffle state.
	Shuffle protocol.ShuffleState `short:"s" negatable:"true" default:"true" help:"Initial shuffle state."`
	// Repeat is the initial repeat state.
	Repeat protocol.RepeatState `short:"r" default:"queue" help:"Initial repeat state."`

	// InitialQueries are queries whose results will become the initial queue.
	InitialQueries []string `arg:"" optional:"true" help:"Queries whose results will become the initial queue."`
}

func (c Cmd) Run() (err error) {
	s, err := NewServer(c, log.Default())
	if err != nil {
		return err
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

	err = s.Serve()
	return
}
