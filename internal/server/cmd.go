package server

import (
	"log"
	"os"
	"os/signal"
	"sync"

	"mtoohey.com/q/internal/cmd"
	"mtoohey.com/q/internal/protocol"
)

// Cmd contains all possible options for a server.
type Cmd struct {
	// Shuffle is the initial shuffle state.
	Shuffle protocol.ShuffleState `short:"s" negatable:"true" default:"true" help:"Initial shuffle state."`
	// Repeat is the initial repeat state.
	Repeat protocol.RepeatState `short:"r" default:"queue" help:"Initial repeat state."`

	// InitialQueries are queries whose results will become the initial queue.
	InitialQueries []string `arg:"" optional:"true" help:"Queries whose results will become the initial queue."`
}

func (c Cmd) Run(g cmd.Globals) (err error) {
	s, err := NewServer(c, g, log.Default())
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
