package bot

import (
	"context"
	"fmt"
	"regexp"
	"sync"

	"github.com/getsentry/sentry-go"
	"github.com/rs/zerolog"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/bot/queue"
	"gitlab.com/etke.cc/postmoogle/utils"
)

// Mailboxes config
type MBXConfig struct {
	Reserved   []string
	Activation string
}

// Bot represents matrix bot
type Bot struct {
	prefix                  string
	mbxc                    MBXConfig
	domains                 []string
	allowedUsers            []*regexp.Regexp
	allowedAdmins           []*regexp.Regexp
	adminRooms              []id.RoomID
	ignoreBefore            int64 // mautrix 0.15.x migration
	commands                commandList
	rooms                   sync.Map
	proxies                 []string
	sendmail                func(string, string, string) error
	cfg                     *config.Manager
	log                     *zerolog.Logger
	lp                      *linkpearl.Linkpearl
	mu                      utils.Mutex
	q                       *queue.Queue
	handledMembershipEvents sync.Map
}

// New creates a new matrix bot
func New(
	q *queue.Queue,
	lp *linkpearl.Linkpearl,
	log *zerolog.Logger,
	cfg *config.Manager,
	proxies []string,
	prefix string,
	domains []string,
	admins []string,
	mbxc MBXConfig,
) (*Bot, error) {
	b := &Bot{
		domains:    domains,
		prefix:     prefix,
		rooms:      sync.Map{},
		adminRooms: []id.RoomID{},
		proxies:    proxies,
		mbxc:       mbxc,
		cfg:        cfg,
		log:        log,
		lp:         lp,
		mu:         utils.NewMutex(),
		q:          q,
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
	err := fmt.Errorf(message, args...)
	b.log.Error().Err(err).Msg("something is wrong")

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
	parsed := format.RenderMarkdown(message, true, true)
	parsed.MsgType = event.MsgNotice
	_, err := b.lp.Send(roomID, &event.Content{Parsed: &parsed})
	if err != nil {
		sentry.GetHubFromContext(ctx).CaptureException(err)
	}
}

// Start performs matrix /sync
func (b *Bot) Start(statusMsg string) error {
	if err := b.migrateMautrix015(); err != nil {
		return err
	}

	if err := b.syncRooms(); err != nil {
		return err
	}

	b.initSync()
	b.log.Info().Msg("Postmoogle has been started")
	return b.lp.Start(statusMsg)
}

// Stop the bot
func (b *Bot) Stop() {
	err := b.lp.GetClient().SetPresence(event.PresenceOffline)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot set presence = offline")
	}
	b.lp.GetClient().StopSync()
}
