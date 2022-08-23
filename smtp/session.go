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

	if utils.Hostname(to) != s.domain {
		s.log.Debug("wrong domain of %s", to)
		return smtp.ErrAuthRequired
	}

	_, ok := s.client.GetMapping(s.ctx, utils.Mailbox(to))
	if !ok {
		s.log.Debug("mapping for %s not found", to)
		return smtp.ErrAuthRequired
	}

	s.to = to
	s.log.Debug("mail to %s", to)
	return nil
}

func (s *session) parseAttachments(parts []*enmime.Part) []*utils.File {
	files := make([]*utils.File, 0, len(parts))
	for _, attachment := range parts {
		for _, err := range attachment.Errors {
			s.log.Warn("attachment error: %v", err)
		}
		file := utils.NewFile(attachment.FileName, attachment.ContentType, attachment.Content)
		files = append(files, file)
	}

	return files
}

func (s *session) Data(r io.Reader) error {
	parser := enmime.NewParser()
	eml, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}

	attachments := s.parseAttachments(eml.Attachments)
	inlines := s.parseAttachments(eml.Inlines)
	files := make([]*utils.File, 0, len(attachments)+len(inlines))
	files = append(files, attachments...)
	files = append(files, inlines...)
	return s.client.Send(s.ctx, s.from, s.to, eml.GetHeader("Subject"), eml.Text, eml.HTML, files)
}

func (s *session) Reset() {}

func (s *session) Logout() error {
	return nil
}
