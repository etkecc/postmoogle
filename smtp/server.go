package smtp

import (
	"crypto/tls"
	"net"
	"os"
	"time"

	"github.com/emersion/go-smtp"
	"gitlab.com/etke.cc/go/logger"
)

type Config struct {
	Domain string
	Port   string

	TLSCert     string
	TLSKey      string
	TLSPort     string
	TLSRequired bool

	LogLevel string
	MaxSize  int
	Bot      Bot
}

type Server struct {
	log  *logger.Logger
	msa  *smtp.Server
	errs chan error

	port    string
	tlsPort string
	tlsCfg  *tls.Config
}

// NewServer creates new SMTP server
func NewServer(cfg *Config) *Server {
	log := logger.New("smtp/msa.", cfg.LogLevel)
	sender := NewMTA(cfg.LogLevel)
	receiver := &msa{
		log:    log,
		bot:    cfg.Bot,
		domain: cfg.Domain,
	}
	receiver.bot.SetMTA(sender)

	s := smtp.NewServer(receiver)
	s.Domain = cfg.Domain
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = cfg.MaxSize * 1024 * 1024
	s.EnableREQUIRETLS = cfg.TLSRequired
	if log.GetLevel() == "DEBUG" || log.GetLevel() == "TRACE" {
		s.Debug = os.Stdout
	}

	server := &Server{
		msa:     s,
		log:     log,
		port:    cfg.Port,
		tlsPort: cfg.TLSPort,
	}
	server.loadTLSConfig(cfg.TLSCert, cfg.TLSKey)
	return server
}

// Start SMTP server
func (s *Server) Start() error {
	s.errs = make(chan error, 1)
	go s.listen(s.port, nil)
	if s.tlsCfg != nil {
		go s.listen(s.tlsPort, s.tlsCfg)
	}

	return <-s.errs
}

// Stop SMTP server
func (s *Server) Stop() {
	err := s.msa.Close()
	if err != nil {
		s.log.Error("cannot stop SMTP server properly: %v", err)
	}
	s.log.Info("SMTP server has been stopped")
}

func (s *Server) listen(port string, tlsCfg *tls.Config) {
	var l net.Listener
	var err error
	if tlsCfg != nil {
		l, err = tls.Listen("tcp", ":"+port, tlsCfg)
	} else {
		l, err = net.Listen("tcp", ":"+port)
	}
	if err != nil {
		s.log.Error("cannot start listener on %s: %v", port, err)
		s.errs <- err
		return
	}

	s.log.Info("Starting SMTP server on port %s", port)

	err = s.msa.Serve(l)
	if err != nil {
		s.log.Error("cannot start SMTP server on %s: %v", port, err)
		s.errs <- err
		close(s.errs)
	}
}

func (s *Server) loadTLSConfig(cert, key string) {
	if cert == "" || key == "" {
		s.log.Warn("SSL certificate is not provided")
		return
	}

	tlsCert, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		s.log.Error("cannot load SSL certificate: %v", err)
		return
	}
	s.tlsCfg = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	s.msa.TLSConfig = s.tlsCfg
}
