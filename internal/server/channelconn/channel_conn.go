package channelconn

import (
	"fmt"
	"net"
	"sync"

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
	// closedMu protects closed.
	closedMu sync.Mutex
	// closedCh is gets closed when the connection is closed either by calling
	// Close or because the other end of the connection was closed.
	closed chan struct{}

	// receive recieves messages. It may be closed by the other end to indicate
	// that the connection has been closed.
	receive <-chan protocol.Message
	// sendMu protects send. In this context, sending to the channel is
	// considered a "read", and closing as well as assigning to nil is
	// considered a "write".
	sendMu sync.RWMutex
	// send can be used to send messages. It is nil when the connection is
	// closed.
	send chan<- protocol.Message
}

func (cc *ChannelConn) Close() error {
	cc.closedMu.Lock()

	select {
	case <-cc.closed:
		// already closed, do nothing
		cc.closedMu.Unlock()

	default:
		// this should cause all senders and receivers to stop blocking soon
		close(cc.closed)
		cc.closedMu.Unlock()

		// wait for all senders to stop
		cc.sendMu.Lock()

		// then close the channel and set the send channel to nil so that we
		// can tell that we shouldn't send anything else
		close(cc.send)
		cc.send = nil

		cc.sendMu.Unlock()
	}

	return nil
}

func (cc *ChannelConn) Receive() (protocol.Message, error) {
	select {
	case <-cc.closed:
		return nil, fmt.Errorf("channel conn closed: %w", net.ErrClosed)

	case m, ok := <-cc.receive:
		if !ok {
			_ = cc.Close() // never returns an error
			return nil, fmt.Errorf("channel conn closed: %w", net.ErrClosed)
		}

		return m, nil
	}
}

func (cc *ChannelConn) Send(m protocol.Message) error {
	cc.sendMu.RLock()

	if cc.send == nil {
		cc.sendMu.RUnlock()
		return fmt.Errorf("channel conn closed: %w", net.ErrClosed)
	}

	select {
	case <-cc.closed:
		cc.sendMu.RUnlock()
		return fmt.Errorf("channel conn closed: %w", net.ErrClosed)

	case cc.send <- m:
		cc.sendMu.RUnlock()
		return nil
	}
}

func (cc *ChannelConn) String() string {
	return fmt.Sprintf("channel conn %v", cc.receive)
}
