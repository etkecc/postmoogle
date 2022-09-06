package bot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data keys
const (
	acMessagePrefix   = "cc.etke.postmoogle.message"
	acLastEventPrefix = "cc.etke.postmoogle.last"
)

// event keys
const (
	eventMessageIDkey = "cc.etke.postmoogle.messageID"
	eventInReplyToKey = "cc.etke.postmoogle.inReplyTo"
	eventSubjectKey   = "cc.etke.postmoogle.subject"
	eventFromKey      = "cc.etke.postmoogle.from"
)

// SetMTA sets mail transfer agent instance to the bot
func (b *Bot) SetMTA(mta utils.MTA) {
	b.mta = mta
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
func (b *Bot) Send2Matrix(ctx context.Context, email *utils.Email) error {
	roomID, ok := b.GetMapping(utils.Mailbox(email.To))
	if !ok {
		return errors.New("room not found")
	}
	b.lock(roomID)
	defer b.unlock(roomID)

	cfg, err := b.getRoomSettings(roomID)
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
	content := email.Content(threadID, cfg.ContentOptions())
	eventID, serr := b.lp.Send(roomID, content)
	if serr != nil {
		return utils.UnwrapError(serr)
	}

	if threadID == "" && !cfg.NoThreads() {
		b.setThreadID(roomID, email.MessageID, eventID)
		threadID = eventID
	}
	b.setLastEventID(roomID, threadID, eventID)

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}
	return nil
}

func (b *Bot) getParentEmail(evt *event.Event) (string, string, string) {
	content := evt.Content.AsMessage()
	parentID := utils.EventParent(evt.ID, content)
	if parentID == evt.ID {
		return "", "", ""
	}
	parentID = b.getLastEventID(evt.RoomID, parentID)
	parentEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, parentID)
	if err != nil {
		b.log.Error("cannot get parent event: %v", err)
		return "", "", ""
	}
	if parentEvt.Content.Parsed == nil {
		perr := parentEvt.Content.ParseRaw(event.EventMessage)
		if perr != nil {
			b.log.Error("cannot parse event content: %v", perr)
			return "", "", ""
		}
	}

	to := utils.EventField[string](&parentEvt.Content, eventFromKey)
	inReplyTo := utils.EventField[string](&parentEvt.Content, eventMessageIDkey)
	if inReplyTo == "" {
		inReplyTo = parentID.String()
	}

	subject := utils.EventField[string](&parentEvt.Content, eventSubjectKey)
	if subject != "" {
		subject = "Re: " + subject
	} else {
		subject = strings.SplitN(content.Body, "\n", 1)[0]
	}

	return to, inReplyTo, subject
}

// Send2Email sends message to email
// TODO rewrite to thread replies only
func (b *Bot) Send2Email(ctx context.Context, to, subject, body string) error {
	var inReplyTo string
	evt := eventFromContext(ctx)
	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		return err
	}
	mailbox := cfg.Mailbox()
	if mailbox == "" {
		return fmt.Errorf("mailbox not configured, kupo")
	}
	from := mailbox + "@" + b.domain
	pTo, pInReplyTo, pSubject := b.getParentEmail(evt)
	inReplyTo = pInReplyTo
	if pTo != "" && to == "" {
		to = pTo
	}
	if pSubject != "" && subject == "" {
		subject = pSubject
	}

	content := evt.Content.AsMessage()
	if subject == "" {
		subject = strings.SplitN(content.Body, "\n", 1)[0]
	}
	if body == "" {
		if content.FormattedBody != "" {
			body = content.FormattedBody
		} else {
			body = content.Body
		}
	}

	ID := evt.ID.String()[1:] + "@" + b.domain
	data := utils.
		NewEmail(ID, inReplyTo, subject, from, to, body, "", nil).
		Compose(b.getBotSettings().DKIMPrivateKey())
	return b.mta.Send(from, to, data)
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
			RelatesTo: utils.RelatesTo(!noThreads, parentID),
		})
		if err != nil {
			b.Error(ctx, roomID, "cannot send uploaded file %s: %v", req.FileName, err)
		}
	}
}

func (b *Bot) getThreadID(roomID id.RoomID, messageID string) id.EventID {
	key := acMessagePrefix + "." + messageID
	data := map[string]id.EventID{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, key, &data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot retrieve account data %s: %v", key, err)
			return ""
		}
	}

	return data["eventID"]
}

func (b *Bot) setThreadID(roomID id.RoomID, messageID string, eventID id.EventID) {
	key := acMessagePrefix + "." + messageID
	data := map[string]id.EventID{
		"eventID": eventID,
	}

	err := b.lp.GetClient().SetRoomAccountData(roomID, key, data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot save account data %s: %v", key, err)
		}
	}
}

func (b *Bot) getLastEventID(roomID id.RoomID, threadID id.EventID) id.EventID {
	key := acLastEventPrefix + "." + threadID.String()
	data := map[string]id.EventID{}
	err := b.lp.GetClient().GetRoomAccountData(roomID, key, &data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot retrieve account data %s: %v", key, err)
			return threadID
		}
	}

	return data["eventID"]
}

func (b *Bot) setLastEventID(roomID id.RoomID, threadID id.EventID, eventID id.EventID) {
	key := acLastEventPrefix + "." + threadID.String()
	data := map[string]id.EventID{
		"eventID": eventID,
	}

	err := b.lp.GetClient().SetRoomAccountData(roomID, key, data)
	if err != nil {
		if !strings.Contains(err.Error(), "M_NOT_FOUND") {
			b.log.Error("cannot save account data %s: %v", key, err)
		}
	}
}
