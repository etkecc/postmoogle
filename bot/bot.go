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
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Bot represents matrix bot
type Bot struct {
	noowner    bool
	federation bool
	prefix     string
	domain     string
	rooms      map[string]id.RoomID
	roomsmu    *sync.Mutex
	log        *logger.Logger
	lp         *linkpearl.Linkpearl
}

// New creates a new matrix bot
func New(lp *linkpearl.Linkpearl, log *logger.Logger, prefix, domain string, noowner, federation bool) *Bot {
	return &Bot{
		noowner:    noowner,
		federation: federation,
		roomsmu:    &sync.Mutex{},
		prefix:     prefix,
		domain:     domain,
		log:        log,
		lp:         lp,
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

// Notice sends a notice message to the matrix room
func (b *Bot) Notice(ctx context.Context, roomID id.RoomID, message string, args ...interface{}) {
	content := format.RenderMarkdown(fmt.Sprintf(message, args...), true, true)
	content.MsgType = event.MsgNotice
	_, err := b.lp.Send(roomID, &content)
	if err != nil {
		if sentry.HasHubOnContext(ctx) {
			sentry.GetHubFromContext(ctx).CaptureException(err)
		} else {
			sentry.CaptureException(err)
		}
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
func (b *Bot) Send(ctx context.Context, from, to, subject, plaintext, html string, files []*utils.File) error {
	roomID, ok := b.GetMapping(ctx, utils.Mailbox(to))
	if !ok {
		return errors.New("room not found")
	}

	settings, err := b.getSettings(ctx, roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	var text strings.Builder
	if !settings.NoSender() {
		text.WriteString("From: ")
		text.WriteString(from)
		text.WriteString("\n\n")
	}
	if !settings.NoSubject() {
		text.WriteString("# ")
		text.WriteString(subject)
		text.WriteString("\n\n")
	}
	if html != "" {
		text.WriteString(format.HTMLToMarkdown(html))
	} else {
		text.WriteString(plaintext)
	}

	content := format.RenderMarkdown(text.String(), true, true)
	_, err = b.lp.Send(roomID, content)
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
func (b *Bot) GetMapping(ctx context.Context, mailbox string) (id.RoomID, bool) {
	if len(b.rooms) == 0 {
		err := b.syncRooms(ctx)
		if err != nil {
			return "", false
		}
	}

	roomID, ok := b.rooms[mailbox]
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
