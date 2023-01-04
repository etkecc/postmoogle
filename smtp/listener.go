package smtp

import (
	"net"

	"gitlab.com/etke.cc/go/logger"
)

// Listener that rejects connections from banned hosts
type Listener struct {
	log      *logger.Logger
	done     chan struct{}
	listener net.Listener
	isBanned func(net.Addr) bool
}

func NewListener(actual net.Listener, isBanned func(net.Addr) bool, log *logger.Logger) *Listener {
	return &Listener{
		log:      log,
		done:     make(chan struct{}, 1),
		listener: actual,
		isBanned: isBanned,
	}
}

// Accept waits for and returns the next connection to the listener.
func (l *Listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.listener.Accept()
		if err != nil {
			select {
			case <-l.done:
				return conn, err
			default:
				l.log.Warn("cannot accept connection: %v", err)
				continue
			}
		}
		if l.isBanned(conn.RemoteAddr()) {
			conn.Close()
			l.log.Info("rejected connection from %q (already banned)", conn.RemoteAddr())
			continue
		}

		l.log.Info("accepted connection from %q", conn.RemoteAddr())
		return conn, nil
	}
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *Listener) Close() error {
	close(l.done)
	return l.listener.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr {
	return l.listener.Addr()
}
