package bot

import (
	"context"
	"errors"
	"strings"

	"maunium.net/go/mautrix/crypto"
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
	eventMessageIDkey  = "cc.etke.postmoogle.messageID"
	eventReferencesKey = "cc.etke.postmoogle.references"
	eventInReplyToKey  = "cc.etke.postmoogle.inReplyTo"
	eventSubjectKey    = "cc.etke.postmoogle.subject"
	eventFromKey       = "cc.etke.postmoogle.from"
	eventToKey         = "cc.etke.postmoogle.to"
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
	if email.InReplyTo != "" || email.References != "" {
		threadID = b.getThreadID(roomID, email.InReplyTo, email.References)
		if threadID != "" {
			b.setThreadID(roomID, email.MessageID, threadID)
		}
	}
	content := email.Content(threadID, cfg.ContentOptions())
	eventID, serr := b.lp.Send(roomID, content)
	if serr != nil {
		return utils.UnwrapError(serr)
	}

	b.setThreadID(roomID, email.MessageID, eventID)
	b.setLastEventID(roomID, threadID, eventID)
	threadID = eventID

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}

	return nil
}

type parentEmail struct {
	MessageID  string
	From       string
	To         string
	InReplyTo  string
	References string
	Subject    string
}

func (b *Bot) getParentEvent(evt *event.Event) *event.Event {
	content := evt.Content.AsMessage()
	parentID := utils.EventParent(evt.ID, content)
	if parentID == evt.ID {
		return evt
	}
	parentID = b.getLastEventID(evt.RoomID, parentID)
	parentEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, parentID)
	if err != nil {
		b.log.Error("cannot get parent event: %v", err)
		return nil
	}

	if !b.lp.GetStore().IsEncrypted(evt.RoomID) {
		if parentEvt.Content.Parsed == nil {
			perr := parentEvt.Content.ParseRaw(event.EventMessage)
			if perr != nil {
				b.log.Error("cannot parse event content: %v", perr)
				return nil
			}
		}
		return parentEvt
	}

	utils.ParseContent(parentEvt, event.EventEncrypted)
	decrypted, err := b.lp.GetMachine().DecryptMegolmEvent(evt)
	if err != nil {
		if err != crypto.IncorrectEncryptedContentType || err != crypto.UnsupportedAlgorithm {
			b.log.Error("cannot decrypt parent event: %v", err)
			return nil
		}
	}
	if decrypted != nil {
		parentEvt.Content = decrypted.Content
	}

	utils.ParseContent(parentEvt, event.EventMessage)
	return parentEvt
}

func (b *Bot) getParentEmail(evt *event.Event) parentEmail {
	var parent parentEmail
	parentEvt := b.getParentEvent(evt)
	if parentEvt == nil {
		return parent
	}
	if parentEvt.ID == evt.ID {
		return parent
	}

	parent.MessageID = utils.MessageID(parentEvt.ID, b.domains[0])
	parent.From = utils.EventField[string](&parentEvt.Content, eventFromKey)
	parent.To = utils.EventField[string](&parentEvt.Content, eventToKey)
	parent.InReplyTo = utils.EventField[string](&parentEvt.Content, eventMessageIDkey)
	parent.References = utils.EventField[string](&parentEvt.Content, eventReferencesKey)
	if parent.InReplyTo == "" {
		parent.InReplyTo = parent.MessageID
	}
	if parent.References == "" {
		parent.References = " " + parent.MessageID
	}

	parent.Subject = utils.EventField[string](&parentEvt.Content, eventSubjectKey)
	if parent.Subject != "" {
		parent.Subject = "Re: " + parent.Subject
	} else {
		parent.Subject = strings.SplitN(evt.Content.AsMessage().Body, "\n", 1)[0]
	}

	return parent
}

// SendEmailReply sends replies from matrix thread to email thread
func (b *Bot) SendEmailReply(ctx context.Context) {
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
	meta := b.getParentEmail(evt)
	// when email was sent from matrix and reply was sent from matrix again
	if fromMailbox != meta.From {
		meta.To = meta.From
	}

	if meta.To == "" {
		b.Error(ctx, evt.RoomID, "cannot find parent email and continue the thread. Please, start a new email thread")
		return
	}

	content := evt.Content.AsMessage()
	if meta.Subject == "" {
		meta.Subject = strings.SplitN(content.Body, "\n", 1)[0]
	}
	body := content.Body

	ID := utils.MessageID(evt.ID, b.domains[0])
	meta.References = meta.References + " " + ID
	b.log.Debug("send email reply ID=%s meta=%+v", ID, meta)
	email := utils.NewEmail(ID, meta.InReplyTo, meta.References, meta.Subject, fromMailbox, meta.To, body, "", nil)
	data := email.Compose(b.getBotSettings().DKIMPrivateKey())

	err = b.sendmail(meta.From, meta.To, data)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot send email: %v", err)
		return
	}
	b.saveSentMetadata(ctx, email, &cfg)
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

func (b *Bot) getThreadID(roomID id.RoomID, messageID string, references string) id.EventID {
	refs := []string{messageID}
	if references != "" {
		refs = append(refs, strings.Split(references, " ")...)
	}

	for _, refID := range refs {
		key := acMessagePrefix + "." + refID
		data := map[string]id.EventID{}
		err := b.lp.GetClient().GetRoomAccountData(roomID, key, &data)
		if err != nil {
			if !strings.Contains(err.Error(), "M_NOT_FOUND") {
				b.log.Error("cannot retrieve account data %s: %v", key, err)
				continue
			}
		}
		if data["eventID"] != "" {
			return data["eventID"]
		}
	}

	return ""
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
