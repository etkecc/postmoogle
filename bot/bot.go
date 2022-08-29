package bot

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"git.sr.ht/~xn/cache/v2"
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
	prefix                  string
	domain                  string
	allowedUsers            []*regexp.Regexp
	allowedAdmins           []*regexp.Regexp
	commands                commandList
	rooms                   sync.Map
	cfg                     cache.Cache[settings]
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
	noowner bool,
	allowedUsers []*regexp.Regexp,
	allowedAdmins []*regexp.Regexp,
) *Bot {
	b := &Bot{
		noowner:       noowner,
		prefix:        prefix,
		domain:        domain,
		allowedUsers:  allowedUsers,
		allowedAdmins: allowedAdmins,
		rooms:         sync.Map{},
		cfg:           cache.NewLRU[settings](1000),
		log:           log,
		lp:            lp,
		mu:            map[id.RoomID]*sync.Mutex{},
	}

	b.commands = b.buildCommandList()

	return b
}

// Error message to the log and matrix room
func (b *Bot) Error(ctx context.Context, roomID id.RoomID, message string, args ...interface{}) {
	b.log.Error(message, args...)
	err := fmt.Errorf(message, args...)

	sentry.GetHubFromContext(ctx).CaptureException(err)
	if roomID != "" {
		b.SendError(ctx, roomID, message)
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
