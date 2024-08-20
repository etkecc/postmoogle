package bot

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"sync"

	"github.com/etkecc/go-linkpearl"
	"github.com/etkecc/go-psd"
	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/bot/queue"
	"github.com/etkecc/postmoogle/internal/utils"
)

// Mailboxes config
type MBXConfig struct {
	Reserved   []string
	Forwarded  []string
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
	sendmail                func(string, string, string, *url.URL) error
	psdc                    *psd.Client
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
	psdc *psd.Client,
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
		psdc:       psdc,
		cfg:        cfg,
		log:        log,
		lp:         lp,
		mu:         utils.NewMutex(),
		q:          q,
	}
	users, err := b.initBotUsers(context.Background())
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
func (b *Bot) Error(ctx context.Context, message string, args ...any) {
	evt := eventFromContext(ctx)
	threadID := threadIDFromContext(ctx)
	if threadID == "" {
		threadID = linkpearl.EventParent(evt.ID, evt.Content.AsMessage())
	}

	err := fmt.Errorf(message, args...) //nolint:goerr113 // we have to
	b.log.Error().Err(err).Msg(err.Error())
	if evt == nil {
		return
	}

	var noThreads bool
	cfg, cerr := b.cfg.GetRoom(ctx, evt.RoomID)
	if cerr == nil {
		noThreads = cfg.NoThreads()
	}

	var relatesTo *event.RelatesTo
	if threadID != "" {
		relatesTo = linkpearl.RelatesTo(threadID, noThreads)
	}

	b.lp.SendNotice(ctx, evt.RoomID, "ERROR: "+err.Error(), relatesTo)
}

// Start performs matrix /sync
func (b *Bot) Start(statusMsg string) error {
	ctx := context.Background()
	if err := b.migrateMautrix015(ctx); err != nil {
		return err
	}

	if err := b.syncRooms(ctx); err != nil {
		return err
	}

	b.initSync()
	b.log.Info().Msg("Postmoogle has been started")
	return b.lp.Start(ctx, statusMsg)
}

// Stop the bot
func (b *Bot) Stop() {
	b.lp.Stop(context.Background())
}
