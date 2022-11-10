package bot

import (
	"context"
	"errors"
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
	eventToKey        = "cc.etke.postmoogle.to"
)

// SetSendmail sets mail sending func to the bot
func (b *Bot) SetSendmail(sendmail func(string, string, string) error) {
	b.sendmail = sendmail
}

// GetDKIMprivkey returns DKIM private key
func (b *Bot) GetDKIMprivkey() string {
	return b.getBotSettings().DKIMPrivateKey()
}

func (b *Bot) getMapping(mailbox string) (id.RoomID, bool) {
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

// GetMapping returns mapping of mailbox = room
func (b *Bot) GetMapping(mailbox string) (id.RoomID, bool) {
	roomID, ok := b.getMapping(mailbox)
	if !ok {
		catchAll := b.getBotSettings().CatchAll()
		if catchAll == "" {
			return roomID, ok
		}
		return b.getMapping(catchAll)
	}

	return roomID, ok
}

// GetIFOptions returns incoming email filtering options (room settings)
func (b *Bot) GetIFOptions(roomID id.RoomID) utils.IncomingFilteringOptions {
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.log.Error("cannot retrieve room settings: %v", err)
		return roomSettings{}
	}

	return cfg
}

// IncomingEmail sends incoming email to matrix room
func (b *Bot) IncomingEmail(ctx context.Context, email *utils.Email) error {
	roomID, ok := b.GetMapping(email.Mailbox(true))
	if !ok {
		return errors.New("room not found")
	}
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	b.lock(roomID)
	defer b.unlock(roomID)

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

func (b *Bot) getParentEmail(evt *event.Event) (string, string, string, string) {
	content := evt.Content.AsMessage()
	parentID := utils.EventParent(evt.ID, content)
	if parentID == evt.ID {
		return "", "", "", ""
	}
	parentID = b.getLastEventID(evt.RoomID, parentID)
	parentEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, parentID)
	if err != nil {
		b.log.Error("cannot get parent event: %v", err)
		return "", "", "", ""
	}
	if parentEvt.Content.Parsed == nil {
		perr := parentEvt.Content.ParseRaw(event.EventMessage)
		if perr != nil {
			b.log.Error("cannot parse event content: %v", perr)
			return "", "", "", ""
		}
	}

	from := utils.EventField[string](&parentEvt.Content, eventFromKey)
	to := utils.EventField[string](&parentEvt.Content, eventToKey)
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

	return from, to, inReplyTo, subject
}

// SendEmailReply sends replies from matrix thread to email thread
func (b *Bot) SendEmailReply(ctx context.Context) {
	var inReplyTo string
	evt := eventFromContext(ctx)
	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot retrieve room settings: %v", err)
		return
	}
	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.Error(ctx, evt.RoomID, "mailbox is not configured, kupo")
		return
	}

	b.lock(evt.RoomID)
	defer b.unlock(evt.RoomID)
	fromMailbox := mailbox + "@" + b.domains[0]
	from, to, inReplyTo, subject := b.getParentEmail(evt)
	// when email was sent from matrix and reply was sent from matrix again
	if fromMailbox != from {
		to = from
	}

	if to == "" {
		b.Error(ctx, evt.RoomID, "cannot find parent email and continue the thread. Please, start a new email thread")
		return
	}

	content := evt.Content.AsMessage()
	if subject == "" {
		subject = strings.SplitN(content.Body, "\n", 1)[0]
	}
	body := content.Body

	ID := evt.ID.String()[1:] + "@" + b.domains[0]
	b.log.Debug("send email reply ID=%s from=%s to=%s inReplyTo=%s subject=%s body=%s", ID, from, to, inReplyTo, subject, body)
	data := utils.
		NewEmail(ID, inReplyTo, subject, from, to, body, "", nil).
		Compose(b.getBotSettings().DKIMPrivateKey())

	err = b.sendmail(from, to, data)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot send email: %v", err)
		return
	}
}

func (b *Bot) sendFiles(ctx context.Context, roomID id.RoomID, files []*utils.File, noThreads bool, parentID id.EventID) {
	for _, file := range files {
		req := file.Convert()
		err := b.lp.SendFile(roomID, req, file.MsgType, utils.RelatesTo(!noThreads, parentID))
		if err != nil {
			b.Error(ctx, roomID, "cannot upload file %s: %v", req.FileName, err)
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
