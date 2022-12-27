package protocol

import (
	"fmt"
	"io"
)

// Listener listens for and accepts client connections.
type Listener interface {
	// Listen begins listening for connections.
	//
	// This function should only be called once. Calling Listen, then Close,
	// then Listen again is invalid. Instead, a new Listener should be created.
	Listen() error

	// Accept blocks and accepts the next client connection.
	//
	// It should return an error satsifying errors.Is(err, net.ErrClosed) when
	// the Listener is closed.
	Accept() (Conn, error)

	fmt.Stringer
	io.Closer
}
