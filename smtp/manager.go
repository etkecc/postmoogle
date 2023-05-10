package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"sync"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/fsnotify/fsnotify"
	"gitlab.com/etke.cc/go/fswatcher"
	"gitlab.com/etke.cc/go/logger"
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

	LogLevel string
	MaxSize  int
	Bot      matrixbot
	Callers  []Caller
	Relay    RelayConfig
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
	log  *logger.Logger
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
	Ban(net.Addr)
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
	log := logger.New("smtp.", cfg.LogLevel)

	smtpClient := newClient(&cfg.Relay, log)

	mailsrv := &mailServer{
		log:        log,
		bot:        cfg.Bot,
		domains:    cfg.Domains,
		mailSender: smtpClient,
	}
	for _, caller := range cfg.Callers {
		caller.SetSendmail(mailsrv.SendEmail)
	}

	s := smtp.NewServer(mailsrv)
	s.ErrorLog = loggerWrapper{func(s string, i ...interface{}) { log.Error(s, i...) }}
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
	if log.GetLevel() == "INFO" || log.GetLevel() == "DEBUG" || log.GetLevel() == "TRACE" {
		s.Debug = loggerWriter{func(s string) { log.Info(s) }}
	}

	fsw, err := fswatcher.New(append(cfg.TLSCerts, cfg.TLSKeys...), 0)
	if err != nil {
		log.Error("cannot start FS watcher: %v", err)
	}

	m := &Manager{
		smtp: s,
		bot:  cfg.Bot,
		log:  log,
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
		m.log.Error("cannot stop filesystem watcher properly: %v", err)
	}

	err = m.smtp.Close()
	if err != nil {
		m.log.Error("cannot stop SMTP server properly: %v", err)
	}

	m.log.Info("SMTP server has been stopped")
}

func (m *Manager) listen(port string, tlsConfig *tls.Config) {
	lwrapper, err := NewListener(port, tlsConfig, m.bot.IsBanned, m.log)
	if err != nil {
		m.log.Error("cannot start listener on %s: %v", port, err)
		m.errs <- err
		return
	}
	if tlsConfig != nil {
		m.tls.Listener = lwrapper
	}
	m.log.Info("Starting SMTP server on port %s", port)

	err = m.smtp.Serve(lwrapper)
	if err != nil {
		m.log.Error("cannot start SMTP server on %s: %v", port, err)
		m.errs <- err
		close(m.errs)
	}
}

// loadTLSConfig returns true if certs were loaded and false if not
func (m *Manager) loadTLSConfig() bool {
	m.log.Info("(re)loading TLS config")
	if len(m.tls.Certs) == 0 || len(m.tls.Keys) == 0 {
		m.log.Warn("SSL certificates are not provided")
		return false
	}

	certificates := make([]tls.Certificate, 0, len(m.tls.Certs))
	for i, path := range m.tls.Certs {
		tlsCert, err := tls.LoadX509KeyPair(path, m.tls.Keys[i])
		if err != nil {
			m.log.Error("cannot load SSL certificate: %v", err)
			continue
		}
		certificates = append(certificates, tlsCert)
	}
	if len(certificates) == 0 {
		return false
	}

	m.tls.Config = &tls.Config{Certificates: certificates}
	m.smtp.TLSConfig = m.tls.Config
	return true
}
