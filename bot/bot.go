package bot

import (
	"context"
	"fmt"
	"sync"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

// Bot represents matrix bot
type Bot struct {
	noowner                 bool
	federation              bool
	prefix                  string
	domain                  string
	rooms                   sync.Map
	log                     *logger.Logger
	lp                      *linkpearl.Linkpearl
	handledMembershipEvents sync.Map
}

// New creates a new matrix bot
func New(lp *linkpearl.Linkpearl, log *logger.Logger, prefix, domain string, noowner, federation bool) *Bot {
	return &Bot{
		noowner:    noowner,
		federation: federation,
		prefix:     prefix,
		domain:     domain,
		rooms:      sync.Map{},
		log:        log,
		lp:         lp,
	}
}

// Error message to the log and matrix room
func (b *Bot) Error(ctx context.Context, roomID id.RoomID, message string, args ...interface{}) {
	b.log.Error(message, args...)

	sentry.GetHubFromContext(ctx).CaptureException(fmt.Errorf(message, args...))
	if roomID != "" {
		// nolint // if something goes wrong here nobody can help...
		b.lp.Send(roomID, &event.MessageEventContent{
			MsgType: event.MsgNotice,
			Body:    "ERROR: " + fmt.Sprintf(message, args...),
		})
	}
}

// Notice sends a notice message to the matrix room
func (b *Bot) Notice(ctx context.Context, roomID id.RoomID, message string) {
	content := format.RenderMarkdown(message, true, true)
	content.MsgType = event.MsgNotice
	_, err := b.lp.Send(roomID, &content)
	if err != nil {
		sentry.GetHubFromContext(ctx).CaptureException(err)
	}
}

// Start performs matrix /sync
func (b *Bot) Start(statusMsg string) error {
	if err := b.migrate(); err != nil {
		return err
	}
	if err := b.syncRooms(); err != nil {
		return err
	}

	b.initSync()
	b.log.Info("Postmoogle has been started")
	return b.lp.Start(statusMsg)
}

// GetMappings returns mapping of mailbox = room
func (b *Bot) GetMapping(mailbox string) (id.RoomID, bool) {
	v, ok := b.rooms.Load(mailbox)
	if !ok {
		return "", ok
	}
	roomID, ok := v.(id.RoomID)
	if !ok {
		return "", ok
	}

	return roomID, ok
}

// Stop the bot
func (b *Bot) Stop() {
	err := b.lp.GetClient().SetPresence(event.PresenceOffline)
	if err != nil {
		b.log.Error("cannot set presence = offline: %v", err)
	}
	b.lp.GetClient().StopSync()
}
