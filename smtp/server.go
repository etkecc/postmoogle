package smtp

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/go/trysmtp"

	"gitlab.com/etke.cc/postmoogle/utils"
)

type mailServer struct {
	bot     matrixbot
	log     *logger.Logger
	domains []string
}

// Login used for outgoing mail submissions only (when you use postmoogle as smtp server in your scripts)
func (m *mailServer) Login(state *smtp.ConnectionState, username, password string) (smtp.Session, error) {
	m.log.Debug("Login state=%+v username=%+v", state, username)
	if !utils.AddressValid(username) {
		return nil, errors.New("please, provide an email address")
	}

	if !m.bot.AllowAuth(username, password) {
		return nil, errors.New("email or password is invalid")
	}

	return &outgoingSession{
		ctx:      sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		sendmail: m.SendEmail,
		privkey:  m.bot.GetDKIMprivkey(),
		from:     username,
		log:      m.log,
		domains:  m.domains,
	}, nil
}

// AnonymousLogin used for incoming mail submissions only
func (m *mailServer) AnonymousLogin(state *smtp.ConnectionState) (smtp.Session, error) {
	m.log.Debug("AnonymousLogin state=%+v", state)
	return &incomingSession{
		ctx:          sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone()),
		getRoomID:    m.bot.GetMapping,
		getFilters:   m.bot.GetIFOptions,
		receiveEmail: m.ReceiveEmail,
		log:          m.log,
		domains:      m.domains,
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
