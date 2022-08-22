package smtp

import (
	"context"
	"os"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
)

type backend struct {
	log    *logger.Logger
	domain string
	client Client
}

func (b *backend) newSession() *session {
	return &session{
		ctx:    sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		log:    b.log,
		domain: b.domain,
		client: b.client,
	}
}

func (b *backend) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return nil, smtp.ErrAuthUnsupported
}

func (b *backend) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return b.newSession(), nil
}

func Start(domain, port, loglevel string, client Client) error {
	log := logger.New("smtp.", loglevel)
	be := &backend{
		log:    log,
		domain: domain,
		client: client,
	}
	s := smtp.NewServer(be)
	s.Addr = ":" + port
	s.Domain = domain
	s.AuthDisabled = true
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = 128 * 1024
	if log.GetLevel() == "DEBUG" || log.GetLevel() == "TRACE" {
		s.Debug = os.Stdout
	}

	log.Info("Starting SMTP server on %s:%s", domain, port)
	return s.ListenAndServe()
}
