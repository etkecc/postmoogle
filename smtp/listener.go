package smtp

import (
	"crypto/tls"
	"net"
	"sync"

	"gitlab.com/etke.cc/go/logger"
)

// Listener that rejects connections from banned hosts
type Listener struct {
	log      *logger.Logger
	done     chan struct{}
	tls      *tls.Config
	tlsMu    sync.Mutex
	listener net.Listener
	isBanned func(net.Addr) bool
}

func NewListener(port string, tlsConfig *tls.Config, isBanned func(net.Addr) bool, log *logger.Logger) (*Listener, error) {
	actual, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, err
	}

	return &Listener{
		log:      log,
		done:     make(chan struct{}, 1),
		tls:      tlsConfig,
		listener: actual,
		isBanned: isBanned,
	}, nil
}

func (l *Listener) SetTLSConfig(cfg *tls.Config) {
	l.tlsMu.Lock()
	l.tls = cfg
	l.tlsMu.Unlock()
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

		if l.tls != nil {
			return l.acceptTLS(conn)
		}
		return conn, nil
	}
}

func (l *Listener) acceptTLS(conn net.Conn) (net.Conn, error) {
	l.tlsMu.Lock()
	defer l.tlsMu.Unlock()

	return tls.Server(conn, l.tls), nil
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
