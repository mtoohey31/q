package unixsocketconn

import (
	"encoding/gob"
	"fmt"
	"net"
	"sync"

	"mtoohey.com/q/internal/protocol"
)

// UnixSocketListener listens for connections on a Unix socket.
type UnixSocketListener struct {
	// SocketPath is the path to the socket that should be listened on.
	SocketPath string

	// mu protects writes to ul.
	mu sync.Mutex
	// ul is the underlying listener.
	ul *net.UnixListener
}

func (usl *UnixSocketListener) Close() error {
	usl.mu.Lock()

	var err error
	if usl.ul != nil {
		err = usl.ul.Close()
		usl.ul = nil
	}

	usl.mu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to close unix socket listener: %w", err)
	}

	return nil
}

func (usl *UnixSocketListener) Listen() error {
	usl.mu.Lock()

	if usl.ul != nil {
		usl.mu.Unlock()
		return nil
	}

	var err error
	usl.ul, err = net.ListenUnix("unix", &net.UnixAddr{
		Name: usl.SocketPath,
		Net:  "unix",
	})
	if err != nil {
		return fmt.Errorf("listen unix failed: %w", err)
	}

	usl.mu.Unlock()

	return nil
}

func (usl *UnixSocketListener) Accept() (protocol.Conn, error) {
	if usl.ul == nil {
		return nil, net.ErrClosed
	}

	uc, err := usl.ul.AcceptUnix()
	if err != nil {
		// AcceptUnix returns an underlying value of type net.ErrClosed when
		// appropriate already, so we don't have to do any mapping
		return nil, fmt.Errorf("accept unix failed: %w", err)
	}

	return &unixSocketConn{
		uc:  uc,
		dec: gob.NewDecoder(uc),
		enc: gob.NewEncoder(uc),
	}, err
}

func (usl *UnixSocketListener) String() string {
	addrString := "<nil>"
	if usl.ul != nil {
		addrString = usl.ul.Addr().String()
	}

	return fmt.Sprintf("unix socket conn %s", addrString)
}

var _ protocol.Listener = &UnixSocketListener{}
