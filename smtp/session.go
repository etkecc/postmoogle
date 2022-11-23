package smtp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/jhillyerd/enmime"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/validator"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/email"
	"gitlab.com/etke.cc/postmoogle/utils"
)

// incomingSession represents an SMTP-submission session receiving emails from remote servers
type incomingSession struct {
	log          *logger.Logger
	getRoomID    func(string) (id.RoomID, bool)
	getFilters   func(id.RoomID) email.IncomingFilteringOptions
	receiveEmail func(context.Context, *email.Email) error
	greylisted   func(net.Addr) bool
	ban          func(net.Addr)
	domains      []string
	enforceDKIM  bool

	ctx  context.Context
	addr net.Addr
	tos  []string
	from string
}

func (s *incomingSession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !email.AddressValid(from) {
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
	s.tos = append(s.tos, to)
	hostname := utils.Hostname(to)
	var domainok bool
	for _, domain := range s.domains {
		if hostname == domain {
			domainok = true
			break
		}
	}
	if !domainok {
		s.log.Debug("wrong domain of %s", to)
		return ErrNoUser
	}

	roomID, ok := s.getRoomID(utils.Mailbox(to))
	if !ok {
		s.log.Debug("mapping for %s not found", to)
		return ErrNoUser
	}

	validations := s.getFilters(roomID)
	s.enforceDKIM = validations.SpamcheckDKIM()
	if !validateIncoming(s.from, to, s.addr, s.log, validations) {
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
	data, err := io.ReadAll(r)
	if err != nil {
		s.log.Error("cannot read DATA: %v", err)
		return err
	}
	reader := bytes.NewReader(data)
	if s.enforceDKIM {
		results, verr := dkim.Verify(reader)
		if verr != nil {
			s.log.Error("cannot verify DKIM: %v", verr)
			return verr
		}
		for _, result := range results {
			if result.Err != nil {
				s.log.Info("DKIM verification of %q failed: %v", result.Domain, result.Err)
				return result.Err
			}
		}
		reader.Seek(0, io.SeekStart) //nolint:errcheck
	}
	parser := enmime.NewParser()
	envelope, err := parser.ReadEnvelope(reader)
	if err != nil {
		return err
	}

	eml := email.FromEnvelope(s.tos[0], envelope)
	for _, to := range s.tos {
		eml.RcptTo = to
		err := s.receiveEmail(s.ctx, eml)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *incomingSession) Reset()        {}
func (s *incomingSession) Logout() error { return nil }

// outgoingSession represents an SMTP-submission session sending emails from external scripts, using postmoogle as SMTP server
type outgoingSession struct {
	log       *logger.Logger
	sendmail  func(string, string, string) error
	privkey   string
	domains   []string
	getRoomID func(string) (id.RoomID, bool)

	ctx      context.Context
	tos      []string
	from     string
	fromRoom id.RoomID
}

func (s *outgoingSession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !email.AddressValid(from) {
		return errors.New("please, provide email address")
	}
	hostname := utils.Hostname(from)
	var domainok bool
	for _, domain := range s.domains {
		if hostname == domain {
			domainok = true
			break
		}
	}
	if !domainok {
		s.log.Debug("wrong domain of %s", from)
		return ErrNoUser
	}

	roomID, ok := s.getRoomID(utils.Mailbox(from))
	if !ok {
		s.log.Debug("mapping for %s not found", from)
		return ErrNoUser
	}
	if s.fromRoom != roomID {
		s.log.Warn("sender from %q tries to impersonate %q", s.fromRoom, roomID)
		return ErrNoUser
	}
	return nil
}

func (s *outgoingSession) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	s.tos = append(s.tos, to)

	s.log.Debug("mail to %s", to)
	return nil
}

func (s *outgoingSession) Data(r io.Reader) error {
	parser := enmime.NewParser()
	envelope, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}
	eml := email.FromEnvelope(s.tos[0], envelope)
	for _, to := range s.tos {
		eml.RcptTo = to
		err := s.sendmail(eml.From, to, eml.Compose(s.privkey))
		if err != nil {
			return err
		}
	}

	return nil
}
func (s *outgoingSession) Reset()        {}
func (s *outgoingSession) Logout() error { return nil }

func validateIncoming(from, to string, senderAddr net.Addr, log *logger.Logger, options email.IncomingFilteringOptions) bool {
	var sender net.IP
	switch netaddr := senderAddr.(type) {
	case *net.TCPAddr:
		sender = netaddr.IP
	default:
		host, _, _ := net.SplitHostPort(senderAddr.String()) // nolint:errcheck
		sender = net.ParseIP(host)
	}

	enforce := validator.Enforce{
		Email: true,
		MX:    options.SpamcheckMX(),
		SPF:   options.SpamcheckSPF(),
		SMTP:  options.SpamcheckSMTP(),
	}
	v := validator.New(options.Spamlist(), enforce, to, log)

	return v.Email(from, sender)
}
