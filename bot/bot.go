package bot

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Bot represents matrix bot
type Bot struct {
	prefix                  string
	domain                  string
	allowedUsers            []*regexp.Regexp
	allowedAdmins           []*regexp.Regexp
	commands                commandList
	rooms                   sync.Map
	mta                     utils.MTA
	log                     *logger.Logger
	lp                      *linkpearl.Linkpearl
	mu                      map[id.RoomID]*sync.Mutex
	handledMembershipEvents sync.Map
}

// New creates a new matrix bot
func New(
	lp *linkpearl.Linkpearl,
	log *logger.Logger,
	prefix string,
	domain string,
	admins []string,
) (*Bot, error) {
	b := &Bot{
		prefix: prefix,
		domain: domain,
		rooms:  sync.Map{},
		log:    log,
		lp:     lp,
		mu:     map[id.RoomID]*sync.Mutex{},
	}
	users, err := b.initBotUsers()
	if err != nil {
		return nil, err
	}
	allowedUsers, uerr := parseMXIDpatterns(users, "")
	if uerr != nil {
		return nil, uerr
	}
	b.allowedUsers = allowedUsers

	allowedAdmins, aerr := parseMXIDpatterns(admins, "")
	if aerr != nil {
		return nil, aerr
	}
	b.allowedAdmins = allowedAdmins

	b.commands = b.initCommands()

	return b, nil
}

// Error message to the log and matrix room
func (b *Bot) Error(ctx context.Context, roomID id.RoomID, message string, args ...interface{}) {
	b.log.Error(message, args...)
	err := fmt.Errorf(message, args...)

	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		sentry.GetHubFromContext(ctx).CaptureException(err)
	}
	if roomID != "" {
		b.SendError(ctx, roomID, err.Error())
	}
}

// SendError sends an error message to the matrix room
func (b *Bot) SendError(ctx context.Context, roomID id.RoomID, message string) {
	b.SendNotice(ctx, roomID, "ERROR: "+message)
}

// SendNotice sends a notice message to the matrix room
func (b *Bot) SendNotice(ctx context.Context, roomID id.RoomID, message string) {
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

// Stop the bot
func (b *Bot) Stop() {
	err := b.lp.GetClient().SetPresence(event.PresenceOffline)
	if err != nil {
		b.log.Error("cannot set presence = offline: %v", err)
	}
	b.lp.GetClient().StopSync()
}
