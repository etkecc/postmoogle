package bot

import (
	"context"
	"errors"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/email"
	"gitlab.com/etke.cc/postmoogle/utils"
)

// account data keys
const (
	acQueueKey        = "cc.etke.postmoogle.mailqueue"
	acMessagePrefix   = "cc.etke.postmoogle.message"
	acLastEventPrefix = "cc.etke.postmoogle.last"
)

// event keys
const (
	eventMessageIDkey  = "cc.etke.postmoogle.messageID"
	eventReferencesKey = "cc.etke.postmoogle.references"
	eventInReplyToKey  = "cc.etke.postmoogle.inReplyTo"
	eventSubjectKey    = "cc.etke.postmoogle.subject"
	eventRcptToKey     = "cc.etke.postmoogle.rcptTo"
	eventFromKey       = "cc.etke.postmoogle.from"
	eventToKey         = "cc.etke.postmoogle.to"
	eventCcKey         = "cc.etke.postmoogle.cc"
)

// SetSendmail sets mail sending func to the bot
func (b *Bot) SetSendmail(sendmail func(string, string, string) error) {
	b.sendmail = sendmail
}

// Sendmail tries to send email immediately, but if it gets 4xx error (greylisting),
// the email will be added to the queue and retried several times after that
func (b *Bot) Sendmail(eventID id.EventID, from, to, data string) (bool, error) {
	err := b.sendmail(from, to, data)
	if err != nil {
		if strings.HasPrefix(err.Error(), "4") {
			b.log.Debug("email %s (from=%s to=%s) was added to the queue: %v", eventID, from, to, err)
			return true, b.enqueueEmail(eventID.String(), from, to, data)
		}
		return false, err
	}

	return false, nil
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
func (b *Bot) GetIFOptions(roomID id.RoomID) email.IncomingFilteringOptions {
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.log.Error("cannot retrieve room settings: %v", err)
		return roomSettings{}
	}

	return cfg
}

// IncomingEmail sends incoming email to matrix room
func (b *Bot) IncomingEmail(ctx context.Context, email *email.Email) error {
	roomID, ok := b.GetMapping(email.Mailbox(true))
	if !ok {
		return errors.New("room not found")
	}
	cfg, err := b.getRoomSettings(roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	b.lock(roomID.String())
	defer b.unlock(roomID.String())

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
	if threadID == "" {
		threadID = eventID
	}

	b.setThreadID(roomID, email.MessageID, threadID)
	b.setLastEventID(roomID, threadID, eventID)
	threadID = eventID

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}

	return nil
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
	domain := utils.SanitizeDomain(cfg.Domain())

	b.lock(evt.RoomID.String())
	defer b.unlock(evt.RoomID.String())

	fromMailbox := mailbox + "@" + domain
	meta := b.getParentEmail(evt, domain)
	// when email was sent from matrix and reply was sent from matrix again
	if fromMailbox != meta.From {
		meta.To = meta.From
	}
	meta.From = fromMailbox

	if meta.To == "" {
		b.Error(ctx, evt.RoomID, "cannot find parent email and continue the thread. Please, start a new email thread")
		return
	}

	if meta.ThreadID == "" {
		meta.ThreadID = b.getThreadID(evt.RoomID, meta.InReplyTo, meta.References)
	}
	content := evt.Content.AsMessage()
	if meta.Subject == "" {
		meta.Subject = strings.SplitN(content.Body, "\n", 1)[0]
	}
	body := content.Body
	htmlBody := content.FormattedBody

	meta.MessageID = email.MessageID(evt.ID, domain)
	meta.References = meta.References + " " + meta.MessageID
	b.log.Debug("send email reply: %+v", meta)
	eml := email.New(meta.MessageID, meta.InReplyTo, meta.References, meta.Subject, meta.From, meta.To, body, htmlBody, nil)
	data := eml.Compose(b.getBotSettings().DKIMPrivateKey())
	if data == "" {
		b.SendError(ctx, evt.RoomID, "email body is empty")
		return
	}

	queued, err := b.Sendmail(evt.ID, meta.From, meta.To, data)
	if queued {
		b.log.Error("cannot send email: %v", err)
		b.saveSentMetadata(ctx, queued, meta.ThreadID, eml, &cfg)
		return
	}

	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot send email: %v", err)
		return
	}

	b.saveSentMetadata(ctx, queued, meta.ThreadID, eml, &cfg)
}

type parentEmail struct {
	MessageID  string
	ThreadID   id.EventID
	From       string
	To         string
	InReplyTo  string
	References string
	Subject    string
}

func (b *Bot) getParentEvent(evt *event.Event) (id.EventID, *event.Event) {
	content := evt.Content.AsMessage()
	threadID := utils.EventParent(evt.ID, content)
	b.log.Debug("looking up for the parent event of %s within thread %s", evt.ID, threadID)
	if threadID == evt.ID {
		b.log.Debug("event %s is the thread itself")
		return threadID, evt
	}
	lastEventID := b.getLastEventID(evt.RoomID, threadID)
	b.log.Debug("the last event of the thread %s (and parent of the %s) is %s", threadID, evt.ID, lastEventID)
	if lastEventID == evt.ID {
		return threadID, evt
	}
	parentEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, lastEventID)
	if err != nil {
		b.log.Error("cannot get parent event: %v", err)
		return threadID, nil
	}
	utils.ParseContent(parentEvt, parentEvt.Type)
	b.log.Debug("type of the parsed content is: %T", parentEvt.Content.Parsed)

	if !b.lp.GetStore().IsEncrypted(evt.RoomID) {
		b.log.Debug("found the last event (plaintext) of the thread %s (and parent of the %s): %+v", threadID, evt.ID, parentEvt)
		return threadID, parentEvt
	}

	decrypted, err := b.lp.GetMachine().DecryptMegolmEvent(parentEvt)
	if err != nil {
		b.log.Error("cannot decrypt parent event: %v", err)
		return threadID, nil
	}

	b.log.Debug("found the last event (decrypted) of the thread %s (and parent of the %s): %+v", threadID, evt.ID, parentEvt)
	return threadID, decrypted
}

func (b *Bot) getParentEmail(evt *event.Event, domain string) parentEmail {
	var parent parentEmail
	threadID, parentEvt := b.getParentEvent(evt)
	parent.ThreadID = threadID
	if parentEvt == nil {
		return parent
	}
	if parentEvt.ID == evt.ID {
		return parent
	}

	parent.MessageID = email.MessageID(parentEvt.ID, domain)
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

// saveSentMetadata used to save metadata from !pm sent and thread reply events to a separate notice message
// because that metadata is needed to determine email thread relations
func (b *Bot) saveSentMetadata(ctx context.Context, queued bool, threadID id.EventID, eml *email.Email, cfg *roomSettings) {
	text := "Email has been sent to " + eml.RcptTo
	if queued {
		text = "Email to " + eml.RcptTo + " has been queued"
	}

	evt := eventFromContext(ctx)
	content := eml.Content(threadID, cfg.ContentOptions())
	notice := format.RenderMarkdown(text, true, true)
	msgContent, ok := content.Parsed.(*event.MessageEventContent)
	if !ok {
		b.Error(ctx, evt.RoomID, "cannot parse message")
		return
	}
	msgContent.MsgType = event.MsgNotice
	msgContent.Body = notice.Body
	msgContent.FormattedBody = notice.FormattedBody
	content.Parsed = msgContent
	msgID, err := b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot send notice: %v", err)
		return
	}
	domain := utils.SanitizeDomain(cfg.Domain())
	b.setThreadID(evt.RoomID, email.MessageID(evt.ID, domain), threadID)
	b.setThreadID(evt.RoomID, email.MessageID(msgID, domain), threadID)
	b.setLastEventID(evt.RoomID, threadID, msgID)
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
		data, err := b.lp.GetRoomAccountData(roomID, key)
		if err != nil {
			b.log.Error("cannot retrieve thread ID from %s: %v", key, err)
			continue
		}
		if data["eventID"] != "" {
			return id.EventID(data["eventID"])
		}
	}

	return ""
}

func (b *Bot) setThreadID(roomID id.RoomID, messageID string, eventID id.EventID) {
	key := acMessagePrefix + "." + messageID
	err := b.lp.SetRoomAccountData(roomID, key, map[string]string{"eventID": eventID.String()})
	if err != nil {
		b.log.Error("cannot save thread ID to %s: %v", key, err)
	}
}

func (b *Bot) getLastEventID(roomID id.RoomID, threadID id.EventID) id.EventID {
	key := acLastEventPrefix + "." + threadID.String()
	data, err := b.lp.GetRoomAccountData(roomID, key)
	if err != nil {
		b.log.Error("cannot retrieve last event ID from %s: %v", key, err)
		return threadID
	}
	if data["eventID"] != "" {
		return id.EventID(data["eventID"])
	}

	return threadID
}

func (b *Bot) setLastEventID(roomID id.RoomID, threadID id.EventID, eventID id.EventID) {
	key := acLastEventPrefix + "." + threadID.String()
	err := b.lp.SetRoomAccountData(roomID, key, map[string]string{"eventID": eventID.String()})
	if err != nil {
		b.log.Error("cannot save thread ID to %s: %v", key, err)
	}
}
