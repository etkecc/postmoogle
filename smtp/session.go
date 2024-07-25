package smtp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/url"
	"slices"
	"strconv"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/jhillyerd/enmime"
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/go/validator/v2"
	"gitlab.com/etke.cc/postmoogle/email"
	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/id"
)

const (
	// Incoming is the direction of the email
	Incoming = "incoming"
	// Outgoing is the direction of the email
	Outoing = "outgoing"
)

// ensure that session implements smtp.AuthSession
var _ smtp.AuthSession = (*session)(nil)

type session struct {
	log      *zerolog.Logger
	bot      matrixbot
	ctx      context.Context //nolint:containedctx // that's session
	conn     *smtp.Conn
	domains  []string
	sendmail func(string, string, string, *url.URL) error

	dir      string
	tos      []string
	from     string
	roomID   id.RoomID
	privkey  string
	fromRoom id.RoomID
}

// AuthMechanisms returns the list of supported authentication mechanisms
func (s *session) AuthMechanisms() []string {
	return []string{sasl.Plain}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	if !slices.Contains(s.AuthMechanisms(), mech) {
		addr := s.conn.Conn().RemoteAddr()
		s.log.Info().Str("addr", addr.String()).Msg("banning due to invalid auth mechanism")
		s.bot.BanAuth(s.ctx, addr)
		return nil, ErrBanned
	}

	return NewPlainAuthServer(s.ctx, s.bot, s.conn, func(identity, username, password string) error {
		return s.authPlain(identity, username, password)
	}), nil
}

func (s *session) authPlain(_, username, password string) error {
	addr := s.conn.Conn().RemoteAddr()
	if !email.AddressValid(username) {
		s.log.Debug().Str("address", username).Msg("address is invalid")
		s.bot.BanAuth(s.ctx, addr)
		return ErrBanned
	}
	roomID, allow := s.bot.AllowAuth(s.ctx, username, password)
	if !allow {
		s.log.Debug().Str("username", username).Msg("username or password is invalid")
		s.bot.BanAuth(s.ctx, addr)
		return ErrBanned
	}

	s.dir = Outoing
	s.from = username
	s.fromRoom = roomID
	return nil
}

func (s *session) Mail(from string, _ *smtp.MailOptions) error {
	if s.dir == Outoing {
		return s.validateOutgoingMail(from)
	}

	// incoming mail
	if !email.AddressValid(from) {
		s.log.Debug().Str("from", from).Msg("address is invalid")
		s.bot.BanAuto(s.ctx, s.conn.Conn().RemoteAddr())
		return ErrBanned
	}
	s.from = email.Address(from)
	s.log.Debug().Str("from", from).Msg("incoming mail")
	return nil
}

func (s *session) Rcpt(to string, _ *smtp.RcptOptions) error {
	s.tos = append(s.tos, to)
	s.log.Debug().Str("to", to).Msg("mail")
	if s.dir == Outoing {
		return nil
	}
	if err := s.validateIncomingRcpt(to); err != nil {
		return err
	}

	if s.bot.GetIFOptions(s.ctx, s.roomID).SpamcheckRBL() {
		s.log.Info().Msg("checking dns blacklists...")
		if listed, reasons := CheckDNSBLs(s.ctx, s.log, s.conn.Conn().RemoteAddr()); listed {
			s.log.Info().Strs("reasons", reasons).Msg("rejected incoming email (DNS Blacklist)")
			if len(reasons) > 0 {
				return extendErrRBL(reasons)
			}
			return ErrRBL
		}
	}

	return nil
}

func (s *session) Data(r io.Reader) error {
	if s.dir == Outoing {
		return s.outgoingData(r)
	}
	return s.incomingData(r)
}

func (s *session) Reset() {}

func (s *session) Logout() error {
	return nil
}

func (s *session) outgoingData(r io.Reader) error {
	parser := enmime.NewParser()
	envelope, err := parser.ReadEnvelope(r)
	if err != nil {
		return err
	}
	eml := email.FromEnvelope(s.tos[0], envelope)
	for _, to := range s.tos {
		eml.RcptTo = to
		err := s.sendmail(eml.From, to, eml.Compose(s.privkey), s.bot.GetRelayConfig(s.ctx, s.fromRoom))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *session) incomingData(r io.Reader) error {
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
	validations := s.bot.GetIFOptions(s.ctx, s.roomID)
	if !validateIncoming(s.from, envelope.GetHeader("Return-Path"), addr, s.log, validations) {
		s.bot.BanAuth(s.ctx, addr)
		return ErrBanned
	}
	if s.bot.IsGreylisted(s.ctx, addr) {
		return ErrGreylisted
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
		err := s.bot.IncomingEmail(s.ctx, eml)
		if err != nil {
			return err
		}
	}
	return nil
}

// validateOutgoingMail checks if the sender is allowed to send mail
func (s *session) validateOutgoingMail(from string) error {
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

	roomID, ok := s.bot.GetMapping(s.ctx, utils.Mailbox(from))
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

// validateIncomingRcpt checks if the recipient is allowed to receive mail
func (s *session) validateIncomingRcpt(to string) error {
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
	s.roomID, ok = s.bot.GetMapping(s.ctx, utils.Mailbox(to))
	if !ok {
		s.log.Debug().Str("to", to).Msg("mapping not found")
		return ErrNoUser
	}

	s.log.Debug().Str("to", to).Msg("mail")
	return nil
}

// getAddr gets real address of incoming email serder,
// including special case of trusted proxy
func (s *session) getAddr(envelope *enmime.Envelope) net.Addr {
	remoteAddr := s.conn.Conn().RemoteAddr()
	if !s.bot.IsTrusted(remoteAddr) {
		return remoteAddr
	}

	addrHeader := envelope.GetHeader("X-Real-Addr")
	if addrHeader == "" {
		return remoteAddr
	}

	host, portString, _ := net.SplitHostPort(addrHeader) //nolint:errcheck // it is real addr
	if host == "" {
		return remoteAddr
	}

	var port int
	port, _ = strconv.Atoi(portString) //nolint:errcheck // it's a real addr
	realAddr := &net.TCPAddr{IP: net.ParseIP(host), Port: port}
	s.log.Info().Str("addr", realAddr.String()).Msg("real address")
	return realAddr
}

func validateIncoming(from, returnPath string, senderAddr net.Addr, log *zerolog.Logger, options email.IncomingFilteringOptions) bool {
	var sender net.IP
	switch netaddr := senderAddr.(type) {
	case *net.TCPAddr:
		sender = netaddr.IP
	default:
		host, _, _ := net.SplitHostPort(senderAddr.String()) //nolint:errcheck // interface constraints
		sender = net.ParseIP(host)
	}

	vcfg := &validator.Config{
		Log: log.Warn().Msgf,
		Email: validator.Email{
			Enforce:  true,
			Spamlist: options.Spamlist(),
			MX:       options.SpamcheckMX(),
			SPF:      options.SpamcheckSPF(),
			SMTP:     options.SpamcheckSMTP(),
			From:     from,
		},
	}
	v := validator.New(vcfg)

	return v.Email(from, returnPath, sender)
}
