package bot

import (
	"context"
	"errors"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/email"
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
	eventRcptToKey     = "cc.etke.postmoogle.rcptTo"
	eventFromKey       = "cc.etke.postmoogle.from"
	eventToKey         = "cc.etke.postmoogle.to"
	eventCcKey         = "cc.etke.postmoogle.cc"
)

// SetSendmail sets mail sending func to the bot
func (b *Bot) SetSendmail(sendmail func(string, string, string) error) {
	b.sendmail = sendmail
	b.q.SetSendmail(sendmail)
}

// Sendmail tries to send email immediately, but if it gets 4xx error (greylisting),
// the email will be added to the queue and retried several times after that
func (b *Bot) Sendmail(eventID id.EventID, from, to, data string) (bool, error) {
	err := b.sendmail(from, to, data)
	if err != nil {
		if strings.HasPrefix(err.Error(), "4") {
			b.log.Info("email %s (from=%s to=%s) was added to the queue: %v", eventID, from, to, err)
			return true, b.q.Add(eventID.String(), from, to, data)
		}
		return false, err
	}

	return false, nil
}

// GetDKIMprivkey returns DKIM private key
func (b *Bot) GetDKIMprivkey() string {
	return b.cfg.GetBot().DKIMPrivateKey()
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
		catchAll := b.cfg.GetBot().CatchAll()
		if catchAll == "" {
			return roomID, ok
		}
		return b.getMapping(catchAll)
	}

	return roomID, ok
}

// GetIFOptions returns incoming email filtering options (room settings)
func (b *Bot) GetIFOptions(roomID id.RoomID) email.IncomingFilteringOptions {
	cfg, err := b.cfg.GetRoom(roomID)
	if err != nil {
		b.log.Error("cannot retrieve room settings: %v", err)
	}

	return cfg
}

// IncomingEmail sends incoming email to matrix room
func (b *Bot) IncomingEmail(ctx context.Context, email *email.Email) error {
	roomID, ok := b.GetMapping(email.Mailbox(true))
	if !ok {
		return errors.New("room not found")
	}
	cfg, err := b.cfg.GetRoom(roomID)
	if err != nil {
		b.Error(ctx, roomID, "cannot get settings: %v", err)
	}

	b.mu.Lock(roomID.String())
	defer b.mu.Unlock(roomID.String())

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

	if !cfg.NoInlines() {
		b.sendFiles(ctx, roomID, email.InlineFiles, cfg.NoThreads(), threadID)
	}

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}

	return nil
}

// SendEmailReply sends replies from matrix thread to email thread
func (b *Bot) SendEmailReply(ctx context.Context) {
	evt := eventFromContext(ctx)
	if !b.allowSend(evt.Sender, evt.RoomID) {
		return
	}
	if !b.allowReply(evt.Sender, evt.RoomID) {
		return
	}
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot retrieve room settings: %v", err)
		return
	}
	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.Error(ctx, evt.RoomID, "mailbox is not configured, kupo")
		return
	}

	b.mu.Lock(evt.RoomID.String())
	defer b.mu.Unlock(evt.RoomID.String())

	meta := b.getParentEmail(evt, mailbox)

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
	var htmlBody string
	if !cfg.NoHTML() {
		htmlBody = content.FormattedBody
	}

	meta.MessageID = email.MessageID(evt.ID, meta.FromDomain)
	meta.References = meta.References + " " + meta.MessageID
	b.log.Info("sending email reply: %+v", meta)
	eml := email.New(meta.MessageID, meta.InReplyTo, meta.References, meta.Subject, meta.From, meta.To, meta.RcptTo, meta.CC, body, htmlBody, nil, nil)
	data := eml.Compose(b.cfg.GetBot().DKIMPrivateKey())
	if data == "" {
		b.SendError(ctx, evt.RoomID, "email body is empty")
		return
	}

	var queued bool
	recipients := meta.Recipients
	for _, to := range recipients {
		queued, err = b.Sendmail(evt.ID, meta.From, to, data)
		if queued {
			b.log.Error("cannot send email: %v", err)
			b.saveSentMetadata(ctx, queued, meta.ThreadID, recipients, eml, cfg)
			continue
		}

		if err != nil {
			b.Error(ctx, evt.RoomID, "cannot send email: %v", err)
			continue
		}
	}

	b.saveSentMetadata(ctx, queued, meta.ThreadID, recipients, eml, cfg)
}

type parentEmail struct {
	MessageID  string
	ThreadID   id.EventID
	From       string
	FromDomain string
	To         string
	RcptTo     string
	CC         string
	InReplyTo  string
	References string
	Subject    string
	Recipients []string
}

// fixtofrom attempts to "fix" or rather reverse the To, From and CC headers
// of parent email by using parent email as metadata source for a new email
// that will be sent from postmoogle.
// To do so, we need to reverse From and To headers, but Cc should be adjusted as well,
// thus that hacky workaround below:
func (e *parentEmail) fixtofrom(newSenderMailbox string, domains []string) string {
	newSenders := make(map[string]string, len(domains))
	for _, domain := range domains {
		sender := newSenderMailbox + "@" + domain
		newSenders[sender] = sender
	}

	// try to determine previous email of the room mailbox
	// by matching RCPT TO, To and From fields
	// why? Because of possible multi-domain setup and we won't leak information
	var previousSender string
	rcptToSender, ok := newSenders[e.RcptTo]
	if ok {
		previousSender = rcptToSender
	}
	toSender, ok := newSenders[e.To]
	if ok {
		previousSender = toSender
	}
	fromSender, ok := newSenders[e.From]
	if ok {
		previousSender = fromSender
	}

	// Message-Id should not leak information either
	e.FromDomain = utils.SanitizeDomain(utils.Hostname(previousSender))

	originalFrom := e.From
	// reverse From if needed
	if fromSender == "" {
		e.From = previousSender
	}
	// reverse To if needed
	if toSender != "" {
		e.To = originalFrom
	}
	// replace previous recipient of the email which is sender now with the original From
	for newSender := range newSenders {
		if strings.Contains(e.CC, newSender) {
			e.CC = strings.ReplaceAll(e.CC, newSender, originalFrom)
		}
	}

	return previousSender
}

func (e *parentEmail) calculateRecipients(from string) {
	recipients := map[string]struct{}{}
	recipients[e.From] = struct{}{}

	for _, addr := range strings.Split(email.Address(e.To), ",") {
		recipients[addr] = struct{}{}
	}
	for _, addr := range email.AddressList(e.CC) {
		recipients[addr] = struct{}{}
	}
	delete(recipients, from)

	rcpts := make([]string, 0, len(recipients))
	for rcpt := range recipients {
		rcpts = append(rcpts, rcpt)
	}

	e.Recipients = rcpts
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

func (b *Bot) getParentEmail(evt *event.Event, newFromMailbox string) *parentEmail {
	parent := &parentEmail{}
	threadID, parentEvt := b.getParentEvent(evt)
	parent.ThreadID = threadID
	if parentEvt == nil {
		return parent
	}
	if parentEvt.ID == evt.ID {
		return parent
	}

	parent.From = utils.EventField[string](&parentEvt.Content, eventFromKey)
	parent.To = utils.EventField[string](&parentEvt.Content, eventToKey)
	parent.CC = utils.EventField[string](&parentEvt.Content, eventCcKey)
	parent.RcptTo = utils.EventField[string](&parentEvt.Content, eventRcptToKey)
	parent.InReplyTo = utils.EventField[string](&parentEvt.Content, eventMessageIDkey)
	parent.References = utils.EventField[string](&parentEvt.Content, eventReferencesKey)
	senderEmail := parent.fixtofrom(newFromMailbox, b.domains)
	parent.calculateRecipients(senderEmail)
	parent.MessageID = email.MessageID(parentEvt.ID, parent.FromDomain)
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
func (b *Bot) saveSentMetadata(ctx context.Context, queued bool, threadID id.EventID, recipients []string, eml *email.Email, cfg config.Room) {
	addrs := strings.Join(recipients, ", ")
	text := "Email has been sent to " + addrs
	if queued {
		text = "Email to " + addrs + " has been queued"
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
