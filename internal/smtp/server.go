package smtp

import (
	"context"

	"github.com/emersion/go-smtp"
	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"

	"github.com/etkecc/postmoogle/internal/email"
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
