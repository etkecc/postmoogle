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

func (b *Bot) email2content(email *utils.Email, cfg settings) *event.MessageEventContent {
	var text strings.Builder
	if !cfg.NoSender() {
		text.WriteString("From: ")
		text.WriteString(email.From)
		text.WriteString("\n\n")
	}
	if !cfg.NoSubject() {
		text.WriteString("# ")
		text.WriteString(email.Subject)
		text.WriteString("\n\n")
	}
	if email.HTML != "" && !cfg.NoHTML() {
		text.WriteString(format.HTMLToMarkdown(email.HTML))
	} else {
		text.WriteString(email.Text)
	}

	content := format.RenderMarkdown(text.String(), true, true)
	return &content
}

// Send email to matrix room
func (b *Bot) Send(ctx context.Context, email *utils.Email) error {
	roomID, ok := b.GetMapping(utils.Mailbox(email.To))
	if !ok {
		return errors.New("room not found")
	}

	cfg, err := b.getSettings(roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	contentParsed := b.email2content(email, cfg)
	content := &event.Content{
		Raw: map[string]interface{}{
			eventMessageIDkey: email.MessageID,
			eventInReplyToKey: email.InReplyTo,
		},
		Parsed: contentParsed,
	}

	var threadID id.EventID
	if email.InReplyTo != "" && !cfg.NoThreads() {
		threadID = b.getThreadID(roomID, email.InReplyTo)
		if threadID != "" {
			contentParsed.SetRelatesTo(&event.RelatesTo{
				Type:    event.RelThread,
				EventID: threadID,
			})
			b.setThreadID(roomID, email.MessageID, threadID)
		}
	}
	eventID, serr := b.lp.Send(roomID, content)
	if serr != nil {
		return serr
	}

	if threadID == "" {
		b.setThreadID(roomID, email.MessageID, eventID)
		threadID = eventID
	}

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, threadID)
	}
	return nil
}

func (b *Bot) sendFiles(ctx context.Context, roomID id.RoomID, files []*utils.File, threadID id.EventID) {
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
			RelatesTo: &event.RelatesTo{
				Type:    event.RelThread,
				EventID: threadID,
			},
		})
		if err != nil {
			b.Error(ctx, roomID, "cannot send uploaded file %s: %v", req.FileName, err)
		}
	}
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
