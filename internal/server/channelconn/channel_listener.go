package channelconn

import (
	"fmt"
	"net"
	"sync"

	"mtoohey.com/q/internal/protocol"
)

// ChannelListener listens for channel connections.
type ChannelListener struct {
	// closedMu protects closed
	closedMu sync.Mutex
	// closed gets closed when this listener is closed.
	closed chan struct{}

	// connMu protects conn.
	connMu sync.RWMutex
	// connCh is used to send connections from a call to Conn to a waiting call
	// to Accept.
	connCh chan *ChannelConn
}

func (cl *ChannelListener) Close() error {
	cl.closedMu.Lock()

	select {
	case <-cl.closed:
		// already closed, do nothing
		cl.closedMu.Unlock()

	default:
		// should stop calls to Accept and Conn soon
		close(cl.closed)
		cl.closedMu.Unlock()

		// wait for Accept and Conn calls to end
		cl.connMu.Lock()

		close(cl.connCh)
		cl.connCh = nil

		cl.connMu.Unlock()
	}

	return nil
}

func (cl *ChannelListener) Listen() error { return nil }

func (cl *ChannelListener) Accept() (protocol.Conn, error) {
	cl.connMu.RLock()

	select {
	case <-cl.closed:
		cl.connMu.RUnlock()
		return nil, fmt.Errorf("channel listener closed: %w", net.ErrClosed)

	case conn, ok := <-cl.connCh: // will block forever once cl.connCh is nil
		cl.connMu.RUnlock()
		if !ok {
			return nil, fmt.Errorf("channel listener closed: %w", net.ErrClosed)
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
// while this functions is blocked waiting for an accept call, it will return
// nil.
func (cl *ChannelListener) Conn() *ChannelConn {
	cl.connMu.RLock()

	clientSend := make(chan protocol.Message)
	serverSend := make(chan protocol.Message)

	select {
	case <-cl.closed:
		cl.connMu.RUnlock()
		return nil

	case cl.connCh <- &ChannelConn{
		closed:  make(chan struct{}),
		receive: clientSend,
		send:    serverSend,
	}:

		cl.connMu.RUnlock()

		return &ChannelConn{
			closed:  make(chan struct{}),
			receive: serverSend,
			send:    clientSend,
		}
	}
}

func NewChannelListener() *ChannelListener {
	return &ChannelListener{
		connCh: make(chan *ChannelConn),
		closed: make(chan struct{}),
	}
}

var _ protocol.Listener = &ChannelListener{}
