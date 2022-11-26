package smtp

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"strconv"

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
	trusted      func(net.Addr) bool
	ban          func(net.Addr)
	domains      []string
	roomID       id.RoomID

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

	var ok bool
	s.roomID, ok = s.getRoomID(utils.Mailbox(to))
	if !ok {
		s.log.Debug("mapping for %s not found", to)
		return ErrNoUser
	}

	s.log.Debug("mail to %s", to)
	return nil
}

// getAddr gets real address of incoming email serder,
// including special case of trusted proxy
func (s *incomingSession) getAddr(envelope *enmime.Envelope) net.Addr {
	if !s.trusted(s.addr) {
		return s.addr
	}

	addrHeader := envelope.GetHeader("X-Real-Addr")
	if addrHeader == "" {
		return s.addr
	}

	host, portString, _ := net.SplitHostPort(addrHeader) //nolint:errcheck
	if host == "" {
		return s.addr
	}

	var port int
	port, _ = strconv.Atoi(portString) //nolint:errcheck

	realAddr := &net.TCPAddr{IP: net.ParseIP(host), Port: port}
	s.log.Info("real address: %s", realAddr.String())
	return realAddr
}

func (s *incomingSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		s.log.Error("cannot read DATA: %v", err)
		return err
	}
	reader := bytes.NewReader(data)
	parser := enmime.NewParser()
	envelope, err := parser.ReadEnvelope(reader)
	if err != nil {
		return err
	}
	addr := s.getAddr(envelope)
	reader.Seek(0, io.SeekStart) //nolint:errcheck
	validations := s.getFilters(s.roomID)
	if !validateIncoming(s.from, s.tos[0], addr, s.log, validations) {
		s.ban(addr)
		return ErrBanned
	}
	if s.greylisted(addr) {
		return &smtp.SMTPError{
			Code:         451,
			EnhancedCode: smtp.EnhancedCode{4, 5, 1},
			Message:      "You have been greylisted, try again a bit later.",
		}
	}
	if validations.SpamcheckDKIM() {
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
