package unixsocketconn

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"

	"mtoohey.com/q/internal/protocol"
)

type unixSocketConn struct {
	uc  *net.UnixConn
	dec *gob.Decoder
	enc *gob.Encoder
}

func (usc *unixSocketConn) Close() error {
	if err := usc.uc.Close(); err != nil {
		return fmt.Errorf("unix socket close failed: %w", err)
	}

	return nil
}

func (usc *unixSocketConn) Receive() (protocol.Message, error) {
	var m any
	if err := usc.dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("unix socket conn closed: %w", net.ErrClosed)
		}

		return nil, fmt.Errorf("unix socket receive failed: %w", err)
	}

	return m, nil
}

func (usc *unixSocketConn) Send(m protocol.Message) error {
	if err := usc.enc.Encode(&m); err != nil {
		var eno syscall.Errno
		if errors.As(err, &eno); eno == 32 {
			return fmt.Errorf("unix socket conn closed: %w", net.ErrClosed)
		}

		return fmt.Errorf("unix socket send failed: %w", err)
	}

	return nil
}

func (usc *unixSocketConn) String() string {
	return fmt.Sprintf("unix socket conn %s", usc.uc.RemoteAddr())
}

func NewUnixSocketClientConn(unixSocket string) (protocol.Conn, error) {
	uc, err := net.DialUnix("unix", nil, &net.UnixAddr{
		Name: unixSocket,
		Net:  "unix",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial socket: %w", err)
	}

	return &unixSocketConn{
		uc:  uc,
		dec: gob.NewDecoder(uc),
		enc: gob.NewEncoder(uc),
	}, nil
}
