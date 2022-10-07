package smtp

import (
	"context"
	"errors"
	"io"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/jhillyerd/enmime"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/validator"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// msasession represents an SMTP-submission session.
// This can be used in 2 directions:
// - receiving emails from remote servers, in which case: `incoming = true`
// - sending emails from local users, in which case: `incoming = false`
type msasession struct {
	log    *logger.Logger
	bot    Bot
	mta    utils.MTA
	domain string

	ctx      context.Context
	incoming bool
	to       string
	from     string
}

func (s *msasession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !utils.AddressValid(from) {
		return errors.New("please, provide email address")
	}
	if s.incoming {
		s.from = from
		s.log.Debug("mail from %s, options: %+v", from, opts)
	}
	return nil
}

func (s *msasession) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	s.to = to

	if s.incoming {
		if utils.Hostname(to) != s.domain {
			s.log.Debug("wrong domain of %s", to)
			return smtp.ErrAuthRequired
		}

		roomID, ok := s.bot.GetMapping(utils.Mailbox(to))
		if !ok {
			s.log.Debug("mapping for %s not found", to)
			return smtp.ErrAuthRequired
		}

		validations := s.bot.GetOptions(roomID)
		if !s.validate(validations) {
			return smtp.ErrAuthRequired
		}
	}

	s.log.Debug("mail to %s", to)
	return nil
}

func (s *msasession) parseAttachments(parts []*enmime.Part) []*utils.File {
	files := make([]*utils.File, 0, len(parts))
	for _, attachment := range parts {
		for _, err := range attachment.Errors {
			s.log.Warn("attachment error: %v", err)
		}
		file := utils.NewFile(attachment.FileName, attachment.Content)
		files = append(files, file)
	}

	return files
}

func (s *msasession) validate(options utils.ValidationOptions) bool {
	spam := validator.Spam{
		Emails:     options.SpamEmails(),
		Hosts:      options.SpamHosts(),
		Localparts: options.SpamLocalparts(),
	}
	enforce := validator.Enforce{
		MX:   options.SecurityMX(),
		SMTP: options.SecuritySMTP(),
	}
	v := validator.New(spam, enforce, s.to, s.log)

	return v.Email(s.from)
}

func (s *msasession) Data(r io.Reader) error {
	parser := enmime.NewParser()
	eml, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}

	files := s.parseAttachments(eml.Attachments)

	email := utils.NewEmail(
		eml.GetHeader("Message-Id"),
		eml.GetHeader("In-Reply-To"),
		eml.GetHeader("Subject"),
		s.from,
		s.to,
		eml.Text,
		eml.HTML,
		files)

	return s.bot.Send2Matrix(s.ctx, email, s.incoming)
}

func (s *msasession) Reset() {}

func (s *msasession) Logout() error {
	return nil
}
