package smtp

import (
	"context"
	"io"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/jhillyerd/enmime"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/postmoogle/utils"
)

type session struct {
	log    *logger.Logger
	domain string
	client Client

	ctx  context.Context
	to   string
	from string
}

func (s *session) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	s.from = from
	s.log.Debug("mail from %s, options: %+v", from, opts)
	return nil
}

func (s *session) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	mappings, err := s.client.GetMappings(s.ctx)
	if err != nil {
		s.log.Error("cannot get mappings: %v", err)
		return err
	}
	s.log.Debug("mappings: %v", mappings)
	_, ok := mappings[utils.Mailbox(to)]
	if !ok {
		s.log.Debug("mapping for %s not found", to)
		return smtp.ErrAuthRequired
	}

	if utils.Hostname(to) != s.domain {
		s.log.Debug("wrong domain of %s", to)
		return smtp.ErrAuthRequired
	}

	s.to = to
	s.log.Debug("mail to %s", to)
	return nil
}

func (s *session) Data(r io.Reader) error {
	parser := enmime.NewParser()
	env, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}
	text := env.Text
	if env.HTML != "" {
		text = env.HTML
	}
	return s.client.Send(s.from, s.to, env.GetHeader("Subject"), text)
}

func (s *session) Reset() {}

func (s *session) Logout() error {
	return nil
}
