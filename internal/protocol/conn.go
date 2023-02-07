package protocol

import (
	"fmt"
	"io"
)

// Conn represents a connection.
type Conn interface {
	// Receive blocks and accepts a message.
	//
	// It should return an error satisfying errors.Is(err, net.ErrClosed) when
	// the operation cannot be completed because the connection was closed.
	Receive() (Message, error)

	// Send blocks and transmits a message.
	//
	// It should return an error satisfying errors.Is(err, net.ErrClosed) when
	// the operation cannot be completed because the connection was closed.
	Send(Message) error

	fmt.Stringer
	io.Closer
}
