package smtp

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/jhillyerd/enmime"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/validator"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// incomingSession represents an SMTP-submission session receiving emails from remote servers
type incomingSession struct {
	log          *logger.Logger
	getRoomID    func(string) (id.RoomID, bool)
	getFilters   func(id.RoomID) utils.IncomingFilteringOptions
	receiveEmail func(context.Context, *utils.Email) error
	greylisted   func(net.Addr) bool
	ban          func(net.Addr)
	domains      []string

	ctx  context.Context
	addr net.Addr
	to   string
	from string
}

func (s *incomingSession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !utils.AddressValid(from) {
		s.log.Debug("address %s is invalid", from)
		s.ban(s.addr)
		return ErrBanned
	}
	s.from = from
	s.log.Debug("mail from %s, options: %+v", from, opts)
	return nil
}

func (s *incomingSession) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	s.to = to
	var domainok bool
	for _, domain := range s.domains {
		if utils.Hostname(to) == domain {
			domainok = true
			break
		}
	}
	if !domainok {
		s.log.Debug("wrong domain of %s", to)
		s.ban(s.addr)
		return ErrBanned
	}

	roomID, ok := s.getRoomID(utils.Mailbox(to))
	if !ok {
		s.log.Debug("mapping for %s not found", to)
		s.ban(s.addr)
		return ErrBanned
	}

	validations := s.getFilters(roomID)
	if !validateEmail(s.from, s.to, s.log, validations) {
		s.ban(s.addr)
		return ErrBanned
	}

	s.log.Debug("mail to %s", to)
	return nil
}

func (s *incomingSession) Data(r io.Reader) error {
	if s.greylisted(s.addr) {
		return &smtp.SMTPError{
			Code:         451,
			EnhancedCode: smtp.EnhancedCode{4, 5, 1},
			Message:      "You have been greylisted, try again a bit later.",
		}
	}
	parser := enmime.NewParser()
	eml, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}

	files := parseAttachments(eml.Attachments, s.log)

	email := utils.NewEmail(
		eml.GetHeader("Message-Id"),
		eml.GetHeader("In-Reply-To"),
		eml.GetHeader("References"),
		eml.GetHeader("Subject"),
		s.from,
		s.to,
		eml.Text,
		eml.HTML,
		files)

	return s.receiveEmail(s.ctx, email)
}
func (s *incomingSession) Reset()        {}
func (s *incomingSession) Logout() error { return nil }

// outgoingSession represents an SMTP-submission session sending emails from external scripts, using postmoogle as SMTP server
type outgoingSession struct {
	log      *logger.Logger
	sendmail func(string, string, string) error
	privkey  string
	domains  []string

	ctx  context.Context
	to   string
	from string
}

func (s *outgoingSession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !utils.AddressValid(from) {
		return errors.New("please, provide email address")
	}
	return nil
}

func (s *outgoingSession) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	s.to = to

	s.log.Debug("mail to %s", to)
	return nil
}

func (s *outgoingSession) Data(r io.Reader) error {
	parser := enmime.NewParser()
	eml, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}

	files := parseAttachments(eml.Attachments, s.log)

	email := utils.NewEmail(
		eml.GetHeader("Message-Id"),
		eml.GetHeader("In-Reply-To"),
		eml.GetHeader("References"),
		eml.GetHeader("Subject"),
		s.from,
		s.to,
		eml.Text,
		eml.HTML,
		files)

	return s.sendmail(email.From, email.To, email.Compose(s.privkey))
}
func (s *outgoingSession) Reset()        {}
func (s *outgoingSession) Logout() error { return nil }

func validateEmail(from, to string, log *logger.Logger, options utils.IncomingFilteringOptions) bool {
	enforce := validator.Enforce{
		Email: true,
		MX:    options.SpamcheckMX(),
		SMTP:  options.SpamcheckMX(),
	}
	v := validator.New(options.Spamlist(), enforce, to, log)

	return v.Email(from)
}

func parseAttachments(parts []*enmime.Part, log *logger.Logger) []*utils.File {
	files := make([]*utils.File, 0, len(parts))
	for _, attachment := range parts {
		for _, err := range attachment.Errors {
			log.Warn("attachment error: %v", err)
		}
		file := utils.NewFile(attachment.FileName, attachment.Content)
		files = append(files, file)
	}

	return files
}
