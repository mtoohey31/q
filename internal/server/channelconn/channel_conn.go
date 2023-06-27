package channelconn

import (
	"fmt"
	"net"
	"sync/atomic"

	"mtoohey.com/q/internal/protocol"
)

// ChannelConn can be used to communicate over Go channels.
//
// This type is thread-safe: Receive, Send, and Close can be called on multiple
// goroutines simultaneously and should produce correct results. However,
// if Receive or Send are called after Close (or if they block until Close
// is called), they will return an error indicating that the connection was
// closed.
type ChannelConn struct {
	// closed indicates that the connection has been closed.
	closed *atomic.Bool
	// closedCh is gets closed when the connection is closed either by calling
	// Close or because the other end of the connection was closed.
	closedCh chan struct{}

	// receive receives messages. It may be closed by the other end to indicate
	// that the connection has been closed.
	receive <-chan protocol.Message
	// send can be used to send messages. It is nil when the connection is
	// closed.
	send chan<- protocol.Message
}

func (cc *ChannelConn) Close() error {
	if cc.closed.Swap(true) {
		// Already closed, don't need to do anything.
		return nil
	}

	// Stop all senders.
	close(cc.closedCh)

	// Clean up this no longer necessary channel. Recieve should be closed by
	// the other end.
	close(cc.send)

	return nil
}

var errConnClosed = fmt.Errorf("channel conn closed: %w", net.ErrClosed)

func (cc *ChannelConn) Receive() (protocol.Message, error) {
	if cc.closed.Load() {
		return nil, errConnClosed
	}

	select {
	case <-cc.closedCh:
		return nil, errConnClosed

	case m, ok := <-cc.receive:
		if !ok {
			// Then the other side was closed, so we should close too.
			_ = cc.Close() // Never returns an error.
			return nil, errConnClosed
		}

		return m, nil
	}
}

func (cc *ChannelConn) Send(m protocol.Message) error {
	if cc.closed.Load() {
		return errConnClosed
	}

	select {
	case <-cc.closedCh:
		return errConnClosed

	case cc.send <- m:
		return nil
	}
}

func (cc *ChannelConn) String() string {
	return fmt.Sprintf("channel conn %v", cc.receive)
}
