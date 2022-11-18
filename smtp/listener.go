package smtp

import (
	"net"

	"gitlab.com/etke.cc/go/logger"
)

// Listener that rejects connections from banned hosts
type Listener struct {
	log      *logger.Logger
	listener net.Listener
	isBanned func(net.Addr) bool
}

func NewListener(actual net.Listener, isBanned func(net.Addr) bool, log *logger.Logger) *Listener {
	return &Listener{
		log:      log,
		listener: actual,
		isBanned: isBanned,
	}
}

// Accept waits for and returns the next connection to the listener.
func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return conn, err
	}
	if l.isBanned(conn.RemoteAddr()) {
		conn.Close()
		l.log.Info("rejected connection from %q (already banned)", conn.RemoteAddr())
		// Due to go-smtp design, any error returned here will crash whole server,
		// thus we have to forge a connection
		return &net.TCPConn{}, nil
	}

	return conn, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *Listener) Close() error {
	return l.listener.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}
