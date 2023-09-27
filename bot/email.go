package bot

import (
	"context"
	"errors"
	"strings"

	"gitlab.com/etke.cc/linkpearl"
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

func (b *Bot) shouldQueue(msg string) bool {
	errors := strings.Split(msg, ";")
	for _, err := range errors {
		errParts := strings.Split(strings.TrimSpace(err), ":")
		if len(errParts) < 2 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(errParts[1]), "4") {
			return true
		}
	}
	return false
}

// Sendmail tries to send email immediately, but if it gets 4xx error (greylisting),
// the email will be added to the queue and retried several times after that
func (b *Bot) Sendmail(eventID id.EventID, from, to, data string) (bool, error) {
	err := b.sendmail(from, to, data)
	if err != nil {
		if b.shouldQueue(err.Error()) {
			b.log.Info().Err(err).Str("id", eventID.String()).Str("from", from).Str("to", to).Msg("email has been added to the queue")
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
		b.log.Error().Err(err).Msg("cannot retrieve room settings")
	}

	return cfg
}

// IncomingEmail sends incoming email to matrix room
//
//nolint:gocognit // TODO
func (b *Bot) IncomingEmail(ctx context.Context, email *email.Email) error {
	roomID, ok := b.GetMapping(email.Mailbox(true))
	if !ok {
		return errors.New("room not found")
	}
	cfg, err := b.cfg.GetRoom(roomID)
	if err != nil {
		b.Error(ctx, "cannot get settings: %v", err)
	}

	b.mu.Lock(roomID.String())
	defer b.mu.Unlock(roomID.String())

	var threadID id.EventID
	newThread := true
	if email.InReplyTo != "" || email.References != "" {
		threadID = b.getThreadID(roomID, email.InReplyTo, email.References)
		if threadID != "" {
			newThread = false
			ctx = threadIDToContext(ctx, threadID)
			b.setThreadID(roomID, email.MessageID, threadID)
		}
	}
	content := email.Content(threadID, cfg.ContentOptions())
	eventID, serr := b.lp.Send(roomID, content)
	if serr != nil {
		if !strings.Contains(serr.Error(), "M_UNKNOWN") { // 	if it's not an unknown event event error
			return serr
		}
		threadID = "" // unknown event edge case - remove existing thread ID to avoid complications
		newThread = true
	}
	if threadID == "" {
		threadID = eventID
		ctx = threadIDToContext(ctx, threadID)
	}

	b.setThreadID(roomID, email.MessageID, threadID)
	b.setLastEventID(roomID, threadID, eventID)

	if !cfg.NoInlines() {
		b.sendFiles(ctx, roomID, email.InlineFiles, cfg.NoThreads(), threadID)
	}

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, email.Files, cfg.NoThreads(), threadID)
	}

	if newThread && cfg.Autoreply() != "" {
		b.sendAutoreply(roomID, threadID)
	}

	return nil
}

//nolint:gocognit // TODO
func (b *Bot) sendAutoreply(roomID id.RoomID, threadID id.EventID) {
	cfg, err := b.cfg.GetRoom(roomID)
	if err != nil {
		return
	}

	text := cfg.Autoreply()
	if text == "" {
		return
	}

	threadEvt, err := b.lp.GetClient().GetEvent(roomID, threadID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot get thread event for autoreply")
		return
	}

	evt := &event.Event{
		ID:     threadID + "-autoreply",
		RoomID: roomID,
		Content: event.Content{
			Parsed: &event.MessageEventContent{
				RelatesTo: &event.RelatesTo{
					Type:    event.RelThread,
					EventID: threadID,
				},
			},
		},
	}

	meta := b.getParentEmail(evt, cfg.Mailbox())

	if meta.To == "" {
		return
	}

	if meta.ThreadID == "" {
		meta.ThreadID = threadID
	}
	if meta.Subject == "" {
		meta.Subject = "Automatic response"
	}
	content := format.RenderMarkdown(text, true, true)
	signature := format.RenderMarkdown(cfg.Signature(), true, true)
	body := content.Body
	if signature.Body != "" {
		body += "\n\n---\n" + signature.Body
	}
	var htmlBody string
	if !cfg.NoHTML() {
		htmlBody = content.FormattedBody
		if htmlBody != "" && signature.FormattedBody != "" {
			htmlBody += "<br><hr><br>" + signature.FormattedBody
		}
	}

	meta.MessageID = email.MessageID(evt.ID, meta.FromDomain)
	meta.References = meta.References + " " + meta.MessageID
	b.log.Info().Any("meta", meta).Msg("sending automatic reply")
	eml := email.New(meta.MessageID, meta.InReplyTo, meta.References, meta.Subject, meta.From, meta.To, meta.RcptTo, meta.CC, body, htmlBody, nil, nil)
	data := eml.Compose(b.cfg.GetBot().DKIMPrivateKey())
	if data == "" {
		return
	}

	var queued bool
	ctx := newContext(threadEvt)
	recipients := meta.Recipients
	for _, to := range recipients {
		queued, err = b.Sendmail(evt.ID, meta.From, to, data)
		if queued {
			b.log.Info().Err(err).Str("from", meta.From).Str("to", to).Msg("email has been queued")
			b.saveSentMetadata(ctx, queued, meta.ThreadID, recipients, eml, cfg, "Autoreply has been sent (queued)")
			continue
		}

		if err != nil {
			b.Error(ctx, "cannot send email: %v", err)
			continue
		}
	}

	b.saveSentMetadata(ctx, queued, meta.ThreadID, recipients, eml, cfg, "Autoreply has been sent")
}

func (b *Bot) canReply(sender id.UserID, roomID id.RoomID) bool {
	return b.allowSend(sender, roomID) && b.allowReply(sender, roomID)
}

// SendEmailReply sends replies from matrix thread to email thread
//
//nolint:gocognit // TODO
func (b *Bot) SendEmailReply(ctx context.Context) {
	evt := eventFromContext(ctx)
	if !b.canReply(evt.Sender, evt.RoomID) {
		return
	}
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "cannot retrieve room settings: %v", err)
		return
	}
	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.Error(ctx, "mailbox is not configured, kupo")
		return
	}

	b.mu.Lock(evt.RoomID.String())
	defer b.mu.Unlock(evt.RoomID.String())

	meta := b.getParentEmail(evt, mailbox)

	if meta.To == "" {
		b.Error(ctx, "cannot find parent email and continue the thread. Please, start a new email thread")
		return
	}

	if meta.ThreadID == "" {
		meta.ThreadID = b.getThreadID(evt.RoomID, meta.InReplyTo, meta.References)
		ctx = threadIDToContext(ctx, meta.ThreadID)
	}
	content := evt.Content.AsMessage()
	if meta.Subject == "" {
		meta.Subject = strings.SplitN(content.Body, "\n", 1)[0]
	}
	signature := format.RenderMarkdown(cfg.Signature(), true, true)
	body := content.Body
	if signature.Body != "" {
		body += "\n\n---\n" + signature.Body
	}
	var htmlBody string
	if !cfg.NoHTML() {
		htmlBody = content.FormattedBody
		if htmlBody != "" && signature.FormattedBody != "" {
			htmlBody += "<br><hr><br>" + signature.FormattedBody
		}
	}

	meta.MessageID = email.MessageID(evt.ID, meta.FromDomain)
	meta.References = meta.References + " " + meta.MessageID
	b.log.Info().Any("meta", meta).Msg("sending email reply")
	eml := email.New(meta.MessageID, meta.InReplyTo, meta.References, meta.Subject, meta.From, meta.To, meta.RcptTo, meta.CC, body, htmlBody, nil, nil)
	data := eml.Compose(b.cfg.GetBot().DKIMPrivateKey())
	if data == "" {
		b.lp.SendNotice(evt.RoomID, "email body is empty", utils.RelatesTo(!cfg.NoThreads(), meta.ThreadID))
		return
	}

	var queued bool
	recipients := meta.Recipients
	for _, to := range recipients {
		queued, err = b.Sendmail(evt.ID, meta.From, to, data)
		if queued {
			b.log.Info().Err(err).Str("from", meta.From).Str("to", to).Msg("email has been queued")
			b.saveSentMetadata(ctx, queued, meta.ThreadID, recipients, eml, cfg)
			continue
		}

		if err != nil {
			b.Error(ctx, "cannot send email: %v", err)
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

func (e *parentEmail) calculateRecipients(from string, forwardedFrom []string) {
	recipients := map[string]struct{}{}
	recipients[e.From] = struct{}{}

	for _, addr := range strings.Split(email.Address(e.To), ",") {
		recipients[addr] = struct{}{}
	}
	for _, addr := range email.AddressList(e.CC) {
		recipients[addr] = struct{}{}
	}

	for _, addr := range forwardedFrom {
		delete(recipients, addr)
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
	threadID := linkpearl.EventParent(evt.ID, content)
	b.log.Debug().Str("eventID", evt.ID.String()).Str("threadID", threadID.String()).Msg("looking up for the parent event within thread")
	if threadID == evt.ID {
		b.log.Debug().Str("eventID", evt.ID.String()).Msg("event is the thread itself")
		return threadID, evt
	}
	lastEventID := b.getLastEventID(evt.RoomID, threadID)
	b.log.Debug().Str("eventID", evt.ID.String()).Str("threadID", threadID.String()).Str("lastEventID", lastEventID.String()).Msg("the last event of the thread (and parent of the event) has been found")
	if lastEventID == evt.ID {
		return threadID, evt
	}
	parentEvt, err := b.lp.GetClient().GetEvent(evt.RoomID, lastEventID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot get parent event")
		return threadID, nil
	}
	linkpearl.ParseContent(parentEvt, parentEvt.Type, b.log)

	if !b.lp.GetMachine().StateStore.IsEncrypted(evt.RoomID) {
		return threadID, parentEvt
	}

	decrypted, err := b.lp.GetClient().Crypto.Decrypt(parentEvt)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot decrypt parent event")
		return threadID, nil
	}

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

	parent.From = linkpearl.EventField[string](&parentEvt.Content, eventFromKey)
	parent.To = linkpearl.EventField[string](&parentEvt.Content, eventToKey)
	parent.CC = linkpearl.EventField[string](&parentEvt.Content, eventCcKey)
	parent.RcptTo = linkpearl.EventField[string](&parentEvt.Content, eventRcptToKey)
	parent.InReplyTo = linkpearl.EventField[string](&parentEvt.Content, eventMessageIDkey)
	parent.References = linkpearl.EventField[string](&parentEvt.Content, eventReferencesKey)
	senderEmail := parent.fixtofrom(newFromMailbox, b.domains)
	parent.calculateRecipients(senderEmail, b.mbxc.Forwarded)
	parent.MessageID = email.MessageID(parentEvt.ID, parent.FromDomain)
	if parent.InReplyTo == "" {
		parent.InReplyTo = parent.MessageID
	}
	if parent.References == "" {
		parent.References = " " + parent.MessageID
	}

	parent.Subject = linkpearl.EventField[string](&parentEvt.Content, eventSubjectKey)
	if parent.Subject != "" {
		parent.Subject = "Re: " + parent.Subject
	} else {
		parent.Subject = strings.SplitN(evt.Content.AsMessage().Body, "\n", 1)[0]
	}

	return parent
}

// saveSentMetadata used to save metadata from !pm sent and thread reply events to a separate notice message
// because that metadata is needed to determine email thread relations
func (b *Bot) saveSentMetadata(ctx context.Context, queued bool, threadID id.EventID, recipients []string, eml *email.Email, cfg config.Room, textOverride ...string) {
	addrs := strings.Join(recipients, ", ")
	text := "Email has been sent to " + addrs
	if queued {
		text = "Email to " + addrs + " has been queued"
	}
	if len(textOverride) > 0 {
		text = textOverride[0]
	}

	evt := eventFromContext(ctx)
	content := eml.Content(threadID, cfg.ContentOptions())
	notice := format.RenderMarkdown(text, true, true)
	msgContent, ok := content.Parsed.(*event.MessageEventContent)
	if !ok {
		b.Error(ctx, "cannot parse message")
		return
	}
	msgContent.MsgType = event.MsgNotice
	msgContent.Body = notice.Body
	msgContent.FormattedBody = notice.FormattedBody
	msgContent.RelatesTo = utils.RelatesTo(!cfg.NoThreads(), threadID)
	content.Parsed = msgContent
	msgID, err := b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(ctx, "cannot send notice: %v", err)
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
			b.Error(ctx, "cannot upload file %s: %v", req.FileName, err)
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
			b.log.Error().Err(err).Str("key", key).Msg("cannot retrieve thread ID")
			continue
		}
		if data["eventID"] == "" {
			continue
		}
		resp, err := b.lp.GetClient().GetEvent(roomID, id.EventID(data["eventID"]))
		if err != nil {
			b.log.Warn().Err(err).Str("roomID", roomID.String()).Str("eventID", data["eventID"]).Msg("cannot get event by id (may be removed)")
			continue
		}
		return resp.ID
	}

	return ""
}

func (b *Bot) setThreadID(roomID id.RoomID, messageID string, eventID id.EventID) {
	key := acMessagePrefix + "." + messageID
	err := b.lp.SetRoomAccountData(roomID, key, map[string]string{"eventID": eventID.String()})
	if err != nil {
		b.log.Error().Err(err).Str("key", key).Msg("cannot save thread ID")
	}
}

func (b *Bot) getLastEventID(roomID id.RoomID, threadID id.EventID) id.EventID {
	key := acLastEventPrefix + "." + threadID.String()
	data, err := b.lp.GetRoomAccountData(roomID, key)
	if err != nil {
		b.log.Error().Err(err).Str("key", key).Msg("cannot retrieve last event ID")
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
		b.log.Error().Err(err).Str("key", key).Msg("cannot save thread ID")
	}
}
