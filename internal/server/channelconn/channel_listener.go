package channelconn

import (
	"fmt"
	"net"
	"sync/atomic"

	"mtoohey.com/q/internal/protocol"
)

// ChannelListener listens for channel connections.
type ChannelListener struct {
	// closed indicates that the connection has been closed.
	closed atomic.Bool
	// closedCh gets closed when this listener is closed.
	closedCh chan struct{}

	// connCh is used to send connections from a call to Conn to a waiting call
	// to Accept.
	connCh chan *ChannelConn
}

func (cl *ChannelListener) Close() error {
	if cl.closed.Swap(true) {
		// Already closed, don't need to do anything.
		return nil
	}

	// Stop all calls to Accept and Conn.
	close(cl.closedCh)

	// Clean this up.
	close(cl.connCh)

	return nil
}

func (cl *ChannelListener) Listen() error { return nil }

var errListenerClosed = fmt.Errorf("channel listener closed: %w", net.ErrClosed)

func (cl *ChannelListener) Accept() (protocol.Conn, error) {
	if cl.closed.Load() {
		return nil, errListenerClosed
	}

	select {
	case <-cl.closedCh:
		return nil, errListenerClosed

	case conn, ok := <-cl.connCh:
		if !ok {
			return nil, errListenerClosed
		}

		return conn, nil
	}
}

func (cl *ChannelListener) String() string {
	return fmt.Sprintf("channel conn %v", cl.connCh)
}

// Conn returns a new connection and returns the opposite end of the connection
// through an ongoing to call to this Listener's Accept method. This function
// will block if there is no ongoing Accept call. If the listener is closed
// while this function is blocked waiting for an accept call, it will return
// nil.
func (cl *ChannelListener) Conn() *ChannelConn {
	if cl.closed.Load() {
		return nil
	}

	closed := &atomic.Bool{}
	closedCh := make(chan struct{})
	clientSend := make(chan protocol.Message)
	serverSend := make(chan protocol.Message)

	select {
	case <-cl.closedCh:
		return nil

	case cl.connCh <- &ChannelConn{
		closed:   closed,
		closedCh: closedCh,
		receive:  clientSend,
		send:     serverSend,
	}:

		return &ChannelConn{
			closed:   closed,
			closedCh: closedCh,
			receive:  serverSend,
			send:     clientSend,
		}
	}
}

func NewChannelListener() *ChannelListener {
	return &ChannelListener{
		connCh:   make(chan *ChannelConn),
		closedCh: make(chan struct{}),
	}
}

var _ protocol.Listener = &ChannelListener{}
