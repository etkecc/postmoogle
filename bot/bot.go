package bot

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// Bot represents matrix bot
type Bot struct {
	prefix string
	domain string
	log    *logger.Logger
	lp     *linkpearl.Linkpearl
}

// New creates a new matrix bot
func New(lp *linkpearl.Linkpearl, log *logger.Logger, prefix, domain string) *Bot {
	return &Bot{
		prefix: prefix,
		domain: domain,
		log:    log,
		lp:     lp,
	}
}

// Error message to the log and matrix room
func (b *Bot) Error(ctx context.Context, roomID id.RoomID, message string, args ...interface{}) {
	b.log.Error(message, args...)

	if sentry.HasHubOnContext(ctx) {
		sentry.GetHubFromContext(ctx).CaptureException(fmt.Errorf(message, args...))
	} else {
		sentry.CaptureException(fmt.Errorf(message, args...))
	}
	if roomID != "" {
		// nolint // if something goes wrong here nobody can help...
		b.lp.Send(roomID, &event.MessageEventContent{
			MsgType: event.MsgNotice,
			Body:    "ERROR: " + fmt.Sprintf(message, args...),
		})
	}
}

// Start performs matrix /sync
func (b *Bot) Start() error {
	if err := b.migrate(); err != nil {
		return err
	}
	if err := b.lp.GetClient().SetPresence(event.PresenceOnline); err != nil {
		return err
	}

	b.initSync()
	b.log.Info("Postmoogle has been started")
	return b.lp.GetClient().Sync()
}

// Stop the bot
func (b *Bot) Stop() {
	err := b.lp.GetClient().SetPresence(event.PresenceOffline)
	if err != nil {
		b.log.Error("cannot set presence = offline: %v", err)
	}
	b.lp.GetClient().StopSync()
}
