package smtp

import (
	"context"
	"os"
	"time"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
)

// msa is mail submission agent, implements smtp.Backend
type msa struct {
	log    *logger.Logger
	domain string
	client Client
}

func (b *msa) newSession() *msasession {
	return &msasession{
		ctx:    sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		log:    b.log,
		domain: b.domain,
		client: b.client,
	}
}

func (b *msa) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return nil, smtp.ErrAuthUnsupported
}

func (b *msa) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return b.newSession(), nil
}

func Start(domain, port, loglevel string, maxSize int, client Client) error {
	log := logger.New("smtp.", loglevel)
	sender := NewMTA(loglevel)
	receiver := &msa{
		log:    log,
		domain: domain,
		client: client,
	}
	receiver.client.SetMTA(sender)
	s := smtp.NewServer(receiver)
	s.Addr = ":" + port
	s.Domain = domain
	s.ReadTimeout = 10 * time.Second
	s.WriteTimeout = 10 * time.Second
	s.MaxMessageBytes = maxSize * 1024 * 1024
	if log.GetLevel() == "DEBUG" || log.GetLevel() == "TRACE" {
		s.Debug = os.Stdout
	}

	log.Info("Starting SMTP server on %s:%s", domain, port)
	return s.ListenAndServe()
}
