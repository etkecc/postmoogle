package smtp

import (
	"context"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
)

// msa is mail submission agent, implements smtp.Backend
type msa struct {
	log    *logger.Logger
	domain string
	bot    Bot
}

func (m *msa) newSession() *msasession {
	return &msasession{
		ctx:    sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		log:    m.log,
		bot:    m.bot,
		domain: m.domain,
	}
}

func (m *msa) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	return nil, smtp.ErrAuthUnsupported
}

func (m *msa) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return m.newSession(), nil
}
