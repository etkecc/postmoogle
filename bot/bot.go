package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/go/logger"
	"gitlab.com/etke.cc/linkpearl"
	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

// Bot represents matrix bot
type Bot struct {
	prefix  string
	domain  string
	rooms   map[string]id.RoomID
	roomsmu *sync.Mutex
	log     *logger.Logger
	lp      *linkpearl.Linkpearl
}

// New creates a new matrix bot
func New(lp *linkpearl.Linkpearl, log *logger.Logger, prefix, domain string) *Bot {
	return &Bot{
		roomsmu: &sync.Mutex{},
		prefix:  prefix,
		domain:  domain,
		log:     log,
		lp:      lp,
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
	ctx := sentry.SetHubOnContext(context.Background(), sentry.CurrentHub().Clone())
	if err := b.syncRooms(ctx); err != nil {
		return err
	}

	b.initSync()
	b.log.Info("Postmoogle has been started")
	return b.lp.Start()
}

// Send email to matrix room
func (b *Bot) Send(ctx context.Context, from, to, subject, body string, files []*utils.File) error {
	roomID, ok := b.rooms[utils.Mailbox(to)]
	if !ok || roomID == "" {
		return errors.New("room not found")
	}

	var text strings.Builder
	text.WriteString("From: ")
	text.WriteString(from)
	text.WriteString("\n\n")
	text.WriteString("# ")
	text.WriteString(subject)
	text.WriteString("\n\n")
	text.WriteString(format.HTMLToMarkdown(body))

	content := format.RenderMarkdown(text.String(), true, true)
	_, err := b.lp.Send(roomID, content)
	if err != nil {
		return err
	}

	for _, file := range files {
		req := file.Convert()
		resp, err := b.lp.GetClient().UploadMedia(req)
		if err != nil {
			b.Error(ctx, roomID, "cannot upload file %s: %v", req.FileName, err)
			continue
		}
		_, err = b.lp.Send(roomID, &event.MessageEventContent{
			MsgType: event.MsgFile,
			Body:    req.FileName,
			URL:     resp.ContentURI.CUString(),
		})
		if err != nil {
			b.Error(ctx, roomID, "cannot send uploaded file %s: %v", req.FileName, err)
		}
	}

	return nil
}

// GetMappings returns mapping of mailbox = room
func (b *Bot) GetMappings(ctx context.Context) (map[string]id.RoomID, error) {
	if len(b.rooms) == 0 {
		err := b.syncRooms(ctx)
		if err != nil {
			return nil, err
		}
	}

	return b.rooms, nil
}

// Stop the bot
func (b *Bot) Stop() {
	err := b.lp.GetClient().SetPresence(event.PresenceOffline)
	if err != nil {
		b.log.Error("cannot set presence = offline: %v", err)
	}
	b.lp.GetClient().StopSync()
}
