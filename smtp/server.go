package smtp

import (
	"context"
	"io"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/trysmtp"

	"gitlab.com/etke.cc/postmoogle/utils"
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
	log     *logger.Logger
	domains []string
}

// Login used for outgoing mail submissions only (when you use postmoogle as smtp server in your scripts)
func (m *mailServer) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	m.log.Debug("Login state=%+v username=%+v", state, username)
	if m.bot.IsBanned(state.RemoteAddr) {
		return nil, ErrBanned
	}

	if !utils.AddressValid(username) {
		m.log.Debug("address %s is invalid", username)
		m.bot.Ban(state.RemoteAddr)
		return nil, ErrBanned
	}

	if !m.bot.AllowAuth(username, password) {
		m.log.Debug("username=%s or password=<redacted> is invalid", username)
		m.bot.Ban(state.RemoteAddr)
		return nil, ErrBanned
	}

	return &outgoingSession{
		ctx:      sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		sendmail: m.SendEmail,
		privkey:  m.bot.GetDKIMprivkey(),
		from:     username,
		log:      m.log,
		domains:  m.domains,
		tos:      []string{},
	}, nil
}

// AnonymousLogin used for incoming mail submissions only
func (m *mailServer) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	m.log.Debug("AnonymousLogin state=%+v", state)
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
		log:          m.log,
		domains:      m.domains,
		addr:         state.RemoteAddr,
		tos:          []string{},
	}, nil
}

// SendEmail to external mail server
func (m *mailServer) SendEmail(from, to, data string) error {
	m.log.Debug("Sending email from %s to %s", from, to)
	conn, err := trysmtp.Connect(from, to)
	if err != nil {
		m.log.Error("cannot connect to SMTP server of %s: %v", to, err)
		return err
	}
	defer conn.Close()

	var w io.WriteCloser
	w, err = conn.Data()
	if err != nil {
		m.log.Error("cannot send DATA command: %v", err)
		return err
	}
	defer w.Close()
	m.log.Debug("sending DATA:\n%s", data)
	_, err = strings.NewReader(data).WriteTo(w)
	if err != nil {
		m.log.Debug("cannot write DATA: %v", err)
		return err
	}

	m.log.Debug("email has been sent")
	return nil
}

// ReceiveEmail - incoming mail into matrix room
func (m *mailServer) ReceiveEmail(ctx context.Context, email *utils.Email) error {
	return m.bot.IncomingEmail(ctx, email)
}
