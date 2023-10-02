package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/go/fswatcher"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/email"
)

type Config struct {
	Domains []string
	Port    string

	TLSCerts    []string
	TLSKeys     []string
	TLSPort     string
	TLSRequired bool

	Logger  *zerolog.Logger
	MaxSize int
	Bot     matrixbot
	Callers []Caller
	Relay   *RelayConfig
}

type TLSConfig struct {
	Listener *Listener
	Config   *tls.Config
	Certs    []string
	Keys     []string
	Port     string
	Mu       sync.Mutex
}

type RelayConfig struct {
	Host     string
	Port     string
	Usename  string
	Password string
}

type Manager struct {
	log  *zerolog.Logger
	bot  matrixbot
	fsw  *fswatcher.Watcher
	smtp *smtp.Server
	errs chan error

	port string
	tls  TLSConfig
}

type matrixbot interface {
	AllowAuth(string, string) (id.RoomID, bool)
	IsGreylisted(net.Addr) bool
	IsBanned(net.Addr) bool
	IsTrusted(net.Addr) bool
	BanAuto(net.Addr)
	BanAuth(net.Addr)
	GetMapping(string) (id.RoomID, bool)
	GetIFOptions(id.RoomID) email.IncomingFilteringOptions
	IncomingEmail(context.Context, *email.Email) error
	GetDKIMprivkey() string
}

// Caller is Sendmail caller
type Caller interface {
	SetSendmail(func(string, string, string) error)
}

// NewManager creates new SMTP server manager
func NewManager(cfg *Config) *Manager {
	mailsrv := &mailServer{
		log:     cfg.Logger,
		bot:     cfg.Bot,
		domains: cfg.Domains,
		sender:  newClient(cfg.Relay, cfg.Logger),
	}
	for _, caller := range cfg.Callers {
		caller.SetSendmail(mailsrv.sender.Send)
	}

	s := smtp.NewServer(mailsrv)
	s.ErrorLog = loggerWrapper{func(s string, i ...any) { cfg.Logger.Error().Msgf(s, i...) }}
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = cfg.MaxSize * 1024 * 1024
	s.AllowInsecureAuth = !cfg.TLSRequired
	s.EnableREQUIRETLS = cfg.TLSRequired
	s.EnableSMTPUTF8 = true
	// set domain in greeting only in single-domain mode
	if len(cfg.Domains) == 1 {
		s.Domain = cfg.Domains[0]
	}
	loglevel := cfg.Logger.GetLevel()
	if loglevel == zerolog.InfoLevel || loglevel == zerolog.DebugLevel || loglevel == zerolog.TraceLevel {
		s.Debug = loggerWriter{func(s string) { cfg.Logger.Info().Msg(s) }}
	}

	fsw, err := fswatcher.New(append(cfg.TLSCerts, cfg.TLSKeys...), 0)
	if err != nil {
		cfg.Logger.Error().Err(err).Msg("cannot start FS watcher")
	}

	m := &Manager{
		smtp: s,
		bot:  cfg.Bot,
		log:  cfg.Logger,
		fsw:  fsw,
		port: cfg.Port,
		tls: TLSConfig{
			Certs: cfg.TLSCerts,
			Keys:  cfg.TLSKeys,
			Port:  cfg.TLSPort,
		},
	}

	m.tls.Mu.Lock()
	m.loadTLSConfig()
	m.tls.Mu.Unlock()

	if m.fsw != nil {
		go m.fsw.Start(func(_ fsnotify.Event) {
			m.tls.Mu.Lock()
			defer m.tls.Mu.Unlock()

			ok := m.loadTLSConfig()
			if ok {
				m.tls.Listener.SetTLSConfig(m.tls.Config)
			}
		})
	}
	return m
}

// Start SMTP server
func (m *Manager) Start() error {
	m.errs = make(chan error, 1)
	go m.listen(m.port, nil)
	if m.tls.Config != nil {
		go m.listen(m.tls.Port, m.tls.Config)
	}

	return <-m.errs
}

// Stop SMTP server
func (m *Manager) Stop() {
	err := m.fsw.Stop()
	if err != nil {
		m.log.Error().Err(err).Msg("cannot stop filesystem watcher properly")
	}

	err = m.smtp.Close()
	if err != nil {
		m.log.Error().Err(err).Msg("cannot stop SMTP server properly")
	}

	m.log.Info().Msg("SMTP server has been stopped")
}

func (m *Manager) listen(port string, tlsConfig *tls.Config) {
	lwrapper, err := NewListener(port, tlsConfig, m.bot.IsBanned, m.log)
	if err != nil {
		m.log.Error().Err(err).Str("port", port).Msg("cannot start listener")
		m.errs <- err
		return
	}
	if tlsConfig != nil {
		m.tls.Listener = lwrapper
	}
	m.log.Info().Str("port", port).Msg("Starting SMTP server")

	err = m.smtp.Serve(lwrapper)
	if err != nil {
		m.log.Error().Str("port", port).Err(err).Msg("cannot start SMTP server")
		m.errs <- err
		close(m.errs)
	}
}

// loadTLSConfig returns true if certs were loaded and false if not
func (m *Manager) loadTLSConfig() bool {
	m.log.Info().Msg("(re)loading TLS config")
	if len(m.tls.Certs) == 0 || len(m.tls.Keys) == 0 {
		m.log.Warn().Msg("SSL certificates are not provided")
		return false
	}

	certificates := make([]tls.Certificate, 0, len(m.tls.Certs))
	for i, path := range m.tls.Certs {
		tlsCert, err := tls.LoadX509KeyPair(path, m.tls.Keys[i])
		if err != nil {
			m.log.Error().Err(err).Msg("cannot load SSL certificate")
			continue
		}
		certificates = append(certificates, tlsCert)
	}
	if len(certificates) == 0 {
		return false
	}

	m.tls.Config = &tls.Config{Certificates: certificates} //nolint:gosec // it's email, even that config is too strict sometimes
	m.smtp.TLSConfig = m.tls.Config
	return true
}
