package smtp

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"time"

	"github.com/emersion/go-smtp"
	"gitlab.com/etke.cc/go/logger"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

type Config struct {
	Domains []string
	Port    string

	TLSCert     string
	TLSKey      string
	TLSPort     string
	TLSRequired bool

	LogLevel string
	MaxSize  int
	Bot      matrixbot
}

type Manager struct {
	log  *logger.Logger
	smtp *smtp.Server
	errs chan error

	port    string
	tlsPort string
	tlsCfg  *tls.Config
}

type matrixbot interface {
	AllowAuth(string, string) bool
	GetMapping(string) (id.RoomID, bool)
	GetIFOptions(id.RoomID) utils.IncomingFilteringOptions
	IncomingEmail(context.Context, *utils.Email) error
	SetSendmail(func(string, string, string) error)
	GetDKIMprivkey() string
}

// NewManager creates new SMTP server manager
func NewManager(cfg *Config) *Manager {
	log := logger.New("smtp.", cfg.LogLevel)
	mailsrv := &mailServer{
		log:     log,
		bot:     cfg.Bot,
		domains: cfg.Domains,
	}
	cfg.Bot.SetSendmail(mailsrv.SendEmail)

	s := smtp.NewServer(mailsrv)
	s.Domain = cfg.Domains[0]
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = cfg.MaxSize * 1024 * 1024
	s.AllowInsecureAuth = !cfg.TLSRequired
	s.EnableREQUIRETLS = cfg.TLSRequired
	s.EnableSMTPUTF8 = true
	if log.GetLevel() == "DEBUG" || log.GetLevel() == "TRACE" {
		s.Debug = os.Stdout
	}

	m := &Manager{
		smtp:    s,
		log:     log,
		port:    cfg.Port,
		tlsPort: cfg.TLSPort,
	}
	m.loadTLSConfig(cfg.TLSCert, cfg.TLSKey)
	return m
}

// Start SMTP server
func (m *Manager) Start() error {
	m.errs = make(chan error, 1)
	go m.listen(m.port, nil)
	if m.tlsCfg != nil {
		go m.listen(m.tlsPort, m.tlsCfg)
	}

	return <-m.errs
}

// Stop SMTP server
func (m *Manager) Stop() {
	err := m.smtp.Close()
	if err != nil {
		m.log.Error("cannot stop SMTP server properly: %v", err)
	}
	m.log.Info("SMTP server has been stopped")
}

func (m *Manager) listen(port string, tlsCfg *tls.Config) {
	var l net.Listener
	var err error
	if tlsCfg != nil {
		l, err = tls.Listen("tcp", ":"+port, tlsCfg)
	} else {
		l, err = net.Listen("tcp", ":"+port)
	}
	if err != nil {
		m.log.Error("cannot start listener on %s: %v", port, err)
		m.errs <- err
		return
	}

	m.log.Info("Starting SMTP server on port %s", port)

	err = m.smtp.Serve(l)
	if err != nil {
		m.log.Error("cannot start SMTP server on %s: %v", port, err)
		m.errs <- err
		close(m.errs)
	}
}

func (m *Manager) loadTLSConfig(cert, key string) {
	if cert == "" || key == "" {
		m.log.Warn("SSL certificate is not provided")
		return
	}

	tlsCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		m.log.Error("cannot load SSL certificate: %v", err)
		return
	}
	m.tlsCfg = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	m.smtp.TLSConfig = m.tlsCfg
}
