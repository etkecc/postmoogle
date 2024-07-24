package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"sync"

	"github.com/rs/zerolog"
)

// Listener that rejects connections from banned hosts
type Listener struct {
	log      *zerolog.Logger
	done     chan struct{}
	tls      *tls.Config
	tlsMu    sync.Mutex
	listener net.Listener
	isBanned func(context.Context, net.Addr) bool
	banDNSBL func(context.Context, net.Addr)
}

func NewListener(
	port string,
	tlsConfig *tls.Config,
	isBanned func(context.Context, net.Addr) bool,
	banDNSBL func(context.Context, net.Addr),
	log *zerolog.Logger,
) (*Listener, error) {
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
		banDNSBL: banDNSBL,
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
				l.log.Warn().Err(err).Msg("cannot accept connection")
				continue
			}
		}
		ctx := context.Background()
		log := l.log.With().Str("addr", conn.RemoteAddr().String()).Logger()
		if l.isBanned(ctx, conn.RemoteAddr()) {
			conn.Close()
			log.Info().Msg("rejected connection (already banned)")
			continue
		}

		log.Info().Msg("checking dns blacklists...")
		if CheckDNSBLs(ctx, l.log, conn.RemoteAddr()) {
			//nolint:gocritic // TODO
			// conn.Close()
			// l.banDNSBL(ctx, conn.RemoteAddr())
			log.Info().Msg("should rejected connection (DNS Blacklist); but won't do it for now (for testing purposes)")
			continue
		}

		log.Info().Msg("accepted connection")

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
