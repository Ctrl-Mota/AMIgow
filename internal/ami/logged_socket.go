package ami

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	dialTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	handshakeTimeout  = 15 * time.Second
	steadyReadTimeout = 90 * time.Second
	keepAlivePeriod   = 30 * time.Second
)

// LoggedSocket é o dono do net.Conn de uma conexão AMI. Só o dispatcher
// escreve e só o reader lê; Close pode ser chamado de qualquer goroutine.
type LoggedSocket struct {
	id          string
	conn        net.Conn
	readTimeout atomic.Int64
	closeOnce   sync.Once
	closeErr    error
}

func dialLoggedSocket(ctx context.Context, address string, id string) (*LoggedSocket, error) {
	dialer := net.Dialer{Timeout: dialTimeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}

	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		tcp.SetKeepAlivePeriod(keepAlivePeriod)
	}

	socket := &LoggedSocket{id: id, conn: conn}
	socket.SetReadTimeout(handshakeTimeout)
	return socket, nil
}

func (ls *LoggedSocket) SetReadTimeout(timeout time.Duration) {
	ls.readTimeout.Store(int64(timeout))
}

func (ls *LoggedSocket) Read(p []byte) (int, error) {
	timeout := time.Duration(ls.readTimeout.Load())
	if timeout > 0 {
		if err := ls.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
			return 0, err
		}
	}
	return ls.conn.Read(p)
}

func (ls *LoggedSocket) Send(message string) error {
	if err := ls.conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		return err
	}
	_, err := io.WriteString(ls.conn, message)
	return err
}

func (ls *LoggedSocket) Close() error {
	ls.closeOnce.Do(func() {
		ls.closeErr = ls.conn.Close()
	})
	return ls.closeErr
}
