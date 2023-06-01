package smtp

import (
	"context"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"

	"gitlab.com/etke.cc/postmoogle/email"
)

var (
	// ErrBanned returned to banned hosts
	ErrBanned = &smtp.SMTPError{
		Code:         554,
		EnhancedCode: smtp.EnhancedCode{5, 5, 4},
		Message:      "please, don't bother me anymore, kupo.",
	}
	// ErrNoUser returned when no such mailbox found
	ErrNoUser = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 5, 0},
		Message:      "no such user here, kupo.",
	}
)

type mailServer struct {
	bot     matrixbot
	log     *zerolog.Logger
	domains []string
	sender  MailSender
}

// Login used for outgoing mail submissions only (when you use postmoogle as smtp server in your scripts)
func (m *mailServer) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	m.log.Debug().Str("username", username).Any("state", state).Msg("Login")
	if m.bot.IsBanned(state.RemoteAddr) {
		return nil, ErrBanned
	}

	if !email.AddressValid(username) {
		m.log.Debug().Str("address", username).Msg("address is invalid")
		m.bot.Ban(state.RemoteAddr)
		return nil, ErrBanned
	}

	roomID, allow := m.bot.AllowAuth(username, password)
	if !allow {
		m.log.Debug().Str("username", username).Msg("username or password is invalid")
		m.bot.Ban(state.RemoteAddr)
		return nil, ErrBanned
	}

	return &outgoingSession{
		ctx:       sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		sendmail:  m.sender.Send,
		privkey:   m.bot.GetDKIMprivkey(),
		from:      username,
		log:       m.log,
		domains:   m.domains,
		getRoomID: m.bot.GetMapping,
		fromRoom:  roomID,
		tos:       []string{},
	}, nil
}

// AnonymousLogin used for incoming mail submissions only
func (m *mailServer) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	m.log.Debug().Any("state", state).Msg("AnonymousLogin")
	if m.bot.IsBanned(state.RemoteAddr) {
		return nil, ErrBanned
	}

	return &incomingSession{
		ctx:          sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		getRoomID:    m.bot.GetMapping,
		getFilters:   m.bot.GetIFOptions,
		receiveEmail: m.ReceiveEmail,
		ban:          m.bot.Ban,
		greylisted:   m.bot.IsGreylisted,
		trusted:      m.bot.IsTrusted,
		log:          m.log,
		domains:      m.domains,
		addr:         state.RemoteAddr,
		tos:          []string{},
	}, nil
}

// ReceiveEmail - incoming mail into matrix room
func (m *mailServer) ReceiveEmail(ctx context.Context, eml *email.Email) error {
	return m.bot.IncomingEmail(ctx, eml)
}
