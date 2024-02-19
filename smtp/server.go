package smtp

import (
	"context"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"

	"gitlab.com/etke.cc/postmoogle/email"
)

const (
	// NoUserCode SMTP code
	NoUserCode = 550
	// BannedCode SMTP code
	BannedCode = 554
)

var (
	// NoUserEnhancedCode enhanced SMTP code
	NoUserEnhancedCode = smtp.EnhancedCode{5, 5, 0}
	// BannedEnhancedCode enhanced SMTP code
	BannedEnhancedCode = smtp.EnhancedCode{5, 5, 4}
	// ErrBanned returned to banned hosts
	ErrBanned = &smtp.SMTPError{
		Code:         BannedCode,
		EnhancedCode: BannedEnhancedCode,
		Message:      "please, don't bother me anymore, kupo.",
	}
	// ErrNoUser returned when no such mailbox found
	ErrNoUser = &smtp.SMTPError{
		Code:         NoUserCode,
		EnhancedCode: NoUserEnhancedCode,
		Message:      "no such user here, kupo.",
	}
)

type mailServer struct {
	bot     matrixbot
	log     *zerolog.Logger
	domains []string
	sender  MailSender
}

func (m *mailServer) NewSession(con *smtp.Conn) (smtp.Session, error) {
	ctx := sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone())
	return &session{
		log:      m.log,
		bot:      m.bot,
		domains:  m.domains,
		sendmail: m.sender.Send,
		conn:     con,
		ctx:      ctx,
		privkey:  m.bot.GetDKIMprivkey(ctx),
	}, nil
}

// ReceiveEmail - incoming mail into matrix room
func (m *mailServer) ReceiveEmail(ctx context.Context, eml *email.Email) error {
	return m.bot.IncomingEmail(ctx, eml)
}
