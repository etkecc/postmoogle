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
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/go/validator"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/email"
	"gitlab.com/etke.cc/postmoogle/utils"
)

// GraylistCode SMTP code
const GraylistCode = 451

var (
	// ErrInvalidEmail for invalid emails :)
	ErrInvalidEmail = errors.New("please, provide valid email address")
	// GraylistEnhancedCode is GraylistCode in enhanced code notation
	GraylistEnhancedCode = smtp.EnhancedCode{4, 5, 1}
)

// incomingSession represents an SMTP-submission session receiving emails from remote servers
type incomingSession struct {
	log          *zerolog.Logger
	getRoomID    func(context.Context, string) (id.RoomID, bool)
	getFilters   func(context.Context, id.RoomID) email.IncomingFilteringOptions
	receiveEmail func(context.Context, *email.Email) error
	greylisted   func(context.Context, net.Addr) bool
	trusted      func(net.Addr) bool
	ban          func(context.Context, net.Addr)
	domains      []string
	roomID       id.RoomID

	ctx  context.Context //nolint:containedctx // that's session
	addr net.Addr
	tos  []string
	from string
}

func (s *incomingSession) Mail(from string, opts smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !email.AddressValid(from) {
		s.log.Debug().Str("from", from).Msg("address is invalid")
		s.ban(s.ctx, s.addr)
		return ErrBanned
	}
	s.from = email.Address(from)
	s.log.Debug().Str("from", from).Any("options", opts).Msg("incoming mail")
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
		s.log.Debug().Str("to", to).Msg("wrong domain")
		return ErrNoUser
	}

	var ok bool
	s.roomID, ok = s.getRoomID(s.ctx, utils.Mailbox(to))
	if !ok {
		s.log.Debug().Str("to", to).Msg("mapping not found")
		return ErrNoUser
	}

	s.log.Debug().Str("to", to).Msg("mail")
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

	host, portString, _ := net.SplitHostPort(addrHeader) //nolint:errcheck // it is real addr
	if host == "" {
		return s.addr
	}

	var port int
	port, _ = strconv.Atoi(portString) //nolint:errcheck // it's a real addr

	realAddr := &net.TCPAddr{IP: net.ParseIP(host), Port: port}
	s.log.Info().Str("addr", realAddr.String()).Msg("real address")
	return realAddr
}

func (s *incomingSession) Data(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		s.log.Error().Err(err).Msg("cannot read DATA")
		return err
	}
	reader := bytes.NewReader(data)
	parser := enmime.NewParser()
	envelope, err := parser.ReadEnvelope(reader)
	if err != nil {
		return err
	}
	addr := s.getAddr(envelope)
	reader.Seek(0, io.SeekStart) //nolint:errcheck // becase we're sure that's ok
	validations := s.getFilters(s.ctx, s.roomID)
	if !validateIncoming(s.from, s.tos[0], addr, s.log, validations) {
		s.ban(s.ctx, addr)
		return ErrBanned
	}
	if s.greylisted(s.ctx, addr) {
		return &smtp.SMTPError{
			Code:         GraylistCode,
			EnhancedCode: GraylistEnhancedCode,
			Message:      "You have been greylisted, try again a bit later.",
		}
	}
	if validations.SpamcheckDKIM() {
		results, verr := dkim.Verify(reader)
		if verr != nil {
			s.log.Error().Err(verr).Msg("cannot verify DKIM")
			return verr
		}
		for _, result := range results {
			if result.Err != nil {
				s.log.Info().Str("domain", result.Domain).Err(result.Err).Msg("DKIM verification failed")
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
	log       *zerolog.Logger
	sendmail  func(string, string, string) error
	privkey   string
	domains   []string
	getRoomID func(context.Context, string) (id.RoomID, bool)

	ctx      context.Context //nolint:containedctx // that's session
	tos      []string
	from     string
	fromRoom id.RoomID
}

func (s *outgoingSession) Mail(from string, _ smtp.MailOptions) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("from", from)
	if !email.AddressValid(from) {
		return ErrInvalidEmail
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
		s.log.Debug().Str("from", from).Msg("wrong domain")
		return ErrNoUser
	}

	roomID, ok := s.getRoomID(s.ctx, utils.Mailbox(from))
	if !ok {
		s.log.Debug().Str("from", from).Msg("mapping not found")
		return ErrNoUser
	}
	if s.fromRoom != roomID {
		s.log.Warn().Str("from_roomID", s.fromRoom.String()).Str("roomID", roomID.String()).Msg("sender from different room tries to impersonate another mailbox")
		return ErrNoUser
	}
	return nil
}

func (s *outgoingSession) Rcpt(to string) error {
	sentry.GetHubFromContext(s.ctx).Scope().SetTag("to", to)
	s.tos = append(s.tos, to)

	s.log.Debug().Str("to", to).Msg("mail")
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

func validateIncoming(from, to string, senderAddr net.Addr, log *zerolog.Logger, options email.IncomingFilteringOptions) bool {
	var sender net.IP
	switch netaddr := senderAddr.(type) {
	case *net.TCPAddr:
		sender = netaddr.IP
	default:
		host, _, _ := net.SplitHostPort(senderAddr.String()) //nolint:errcheck // interface constraints
		sender = net.ParseIP(host)
	}

	enforce := validator.Enforce{
		Email: true,
		MX:    options.SpamcheckMX(),
		SPF:   options.SpamcheckSPF(),
		SMTP:  options.SpamcheckSMTP(),
	}
	v := validator.New(options.Spamlist(), enforce, to, &validatorLoggerWrapper{log: log})

	return v.Email(from, sender)
}
