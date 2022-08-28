package bot

import (
	"context"
	"errors"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

func email2content(email *utils.Email, cfg settings, threadID id.EventID) *event.Content {
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

	parsed := format.RenderMarkdown(text.String(), true, true)
	parsed.RelatesTo = utils.RelatesTo(cfg.NoThreads(), threadID)

	content := event.Content{
		Raw: map[string]interface{}{
			eventMessageIDkey: email.MessageID,
			eventInReplyToKey: email.InReplyTo,
		},
		Parsed: parsed,
	}
	return &content
}

// GetMapping returns mapping of mailbox = room
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

// Send email to matrix room
func (b *Bot) Send(ctx context.Context, email *utils.Email) error {
	roomID, ok := b.GetMapping(utils.Mailbox(email.To))
	if !ok {
		return errors.New("room not found")
	}
	b.lock(roomID)
	defer b.unlock(roomID)

	cfg, err := b.getSettings(roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	var threadID id.EventID
	if email.InReplyTo != "" && !cfg.NoThreads() {
		threadID = b.getThreadID(roomID, email.InReplyTo)
		if threadID != "" {
			b.setThreadID(roomID, email.MessageID, threadID)
		}
	}

	content := email2content(email, cfg, threadID)
	eventID, serr := b.lp.Send(roomID, content)
	if serr != nil {
		return utils.UnwrapError(serr)
	}

	if threadID == "" && !cfg.NoThreads() {
		b.setThreadID(roomID, email.MessageID, eventID)
		threadID = eventID
	}

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}
	return nil
}

func (b *Bot) sendFiles(ctx context.Context, roomID id.RoomID, files []*utils.File, noThreads bool, parentID id.EventID) {
	for _, file := range files {
		req := file.Convert()
		resp, err := b.lp.GetClient().UploadMedia(req)
		if err != nil {
			b.Error(ctx, roomID, "cannot upload file %s: %v", req.FileName, err)
			continue
		}
		_, err = b.lp.Send(roomID, &event.MessageEventContent{
			MsgType:   event.MsgFile,
			Body:      req.FileName,
			URL:       resp.ContentURI.CUString(),
			RelatesTo: utils.RelatesTo(noThreads, parentID),
		})
		if err != nil {
			b.Error(ctx, roomID, "cannot send uploaded file %s: %v", req.FileName, err)
		}
	}
}
