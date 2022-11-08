package smtp

import (
	"context"
	"errors"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// msa is mail submission agent, implements smtp.Backend
type msa struct {
	log     *logger.Logger
	domains []string
	bot     Bot
	mta     utils.MTA
}

func (m *msa) newSession(from string, incoming bool) *msasession {
	return &msasession{
		ctx:      sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		mta:      m.mta,
		from:     from,
		incoming: incoming,
		log:      m.log,
		bot:      m.bot,
		domains:  m.domains,
	}
}

func (m *msa) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	if !utils.AddressValid(username) {
		return nil, errors.New("please, provide an email address")
	}

	if !m.bot.AllowAuth(username, password) {
		return nil, errors.New("email or password is invalid")
	}

	return m.newSession(username, false), nil
}

func (m *msa) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	return m.newSession("", true), nil
}
