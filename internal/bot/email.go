package bot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/etkecc/go-linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/bot/config"
	"github.com/etkecc/postmoogle/internal/email"
	"github.com/etkecc/postmoogle/internal/utils"
)

const (
	// account data keys
	acMessagePrefix   = "cc.etke.postmoogle.message"
	acLastEventPrefix = "cc.etke.postmoogle.last"

	// event keys
	eventMessageIDkey  = "cc.etke.postmoogle.messageID"
	eventReferencesKey = "cc.etke.postmoogle.references"
	eventInReplyToKey  = "cc.etke.postmoogle.inReplyTo"
	eventSubjectKey    = "cc.etke.postmoogle.subject"
	eventRcptToKey     = "cc.etke.postmoogle.rcptTo"
	eventFromKey       = "cc.etke.postmoogle.from"
	eventToKey         = "cc.etke.postmoogle.to"
	eventCcKey         = "cc.etke.postmoogle.cc"
)

var ErrNoRoom = errors.New("room not found")

// SetSendmail sets mail sending func to the bot
func (b *Bot) SetSendmail(sendmail func(string, string, string, *url.URL) error) {
	b.sendmail = sendmail
	b.q.SetSendmail(sendmail)
}

func (b *Bot) shouldQueue(msg string) bool {
	msg = strings.TrimSpace(msg)
	if strings.HasPrefix(msg, "4") { // any temporary issue (4xx SMTP code)
		return true
	}

	if strings.Contains(msg, "450") || strings.Contains(msg, "451") { // greylisting
		return true
	}

	if strings.Contains(msg, "greylisted") { // greylisting
		return true
	}

	return false
}

// Sendmail tries to send email immediately, but if it gets 4xx error (greylisting),
// the email will be added to the queue and retried several times after that
func (b *Bot) Sendmail(ctx context.Context, eventID id.EventID, from, to, data string, relayOverride *url.URL) (bool, error) {
	log := b.log.With().Str("from", from).Str("to", to).Str("eventID", eventID.String()).Logger()
	log.Info().Msg("attempting to deliver email")
	err := b.sendmail(from, to, data, relayOverride)
	if err != nil {
		if b.shouldQueue(err.Error()) {
			log.Info().Err(err).Msg("email has been added to the queue")
			return true, b.q.Add(ctx, eventID.String(), from, to, data, relayOverride)
		}
		log.Warn().Err(err).Msg("email delivery failed")
		return false, err
	}

	log.Info().Msg("email delivery succeeded")
	return false, nil
}

// GetDKIMprivkey returns DKIM private key
func (b *Bot) GetDKIMprivkey(ctx context.Context) string {
	return b.cfg.GetBot(ctx).DKIMPrivateKey()
}

// GetRelayConfig returns relay config for specific room (mailbox) if set
func (b *Bot) GetRelayConfig(ctx context.Context, roomID id.RoomID) *url.URL {
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		b.log.Error().Err(err).Str("room_id", roomID.String()).Msg("cannot get room config")
		return nil
	}
	return cfg.Relay()
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
func (b *Bot) GetMapping(ctx context.Context, mailbox string) (id.RoomID, bool) {
	roomID, ok := b.getMapping(mailbox)
	if !ok {
		catchAll := b.cfg.GetBot(ctx).CatchAll()
		if catchAll == "" {
			return roomID, ok
		}
		return b.getMapping(catchAll)
	}

	return roomID, ok
}

// GetIFOptions returns incoming email filtering options (room settings)
func (b *Bot) GetIFOptions(ctx context.Context, roomID id.RoomID) email.IncomingFilteringOptions {
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot retrieve room settings")
	}

	return cfg
}

// IncomingEmail sends incoming email to matrix room
//
//nolint:gocognit // TODO
func (b *Bot) IncomingEmail(ctx context.Context, eml *email.Email) error {
	roomID, ok := b.GetMapping(ctx, eml.Mailbox(true))
	if !ok {
		return ErrNoRoom
	}
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		b.Error(ctx, "cannot get settings: %v", err)
	}

	b.mu.Lock(roomID.String())
	defer b.mu.Unlock(roomID.String())

	var threadID id.EventID
	newThread := true
	if eml.InReplyTo != "" || eml.References != "" {
		threadID = b.getThreadID(ctx, roomID, eml.InReplyTo, eml.References)
		if threadID != "" {
			newThread = false
			ctx = threadIDToContext(ctx, threadID)
			b.setThreadID(ctx, roomID, eml.MessageID, threadID)
		}
	}

	// if automatic stripping is enabled, there is a chance something important may be stripped out
	// to prevent that, we use a hacky way to generate content without stripping and save it as a file fist
	if cfg.Stripify() && !cfg.Threadify() {
		contentOpts := cfg.ContentOptions()
		contentOpts.Stripify = false
		content := eml.Content(threadID, contentOpts, b.psdc)
		eml.Files = append(eml.Files, //nolint:forcetypeassert // that's ok
			utils.NewFile("original.md", []byte(content.Parsed.(*event.MessageEventContent).Body)), //nolint:errcheck // that's ok
		)
	}

	content := eml.Content(threadID, cfg.ContentOptions(), b.psdc)
	eventID, serr := b.lp.Send(ctx, roomID, content)
	if serr != nil {
		if !strings.Contains(serr.Error(), "M_UNKNOWN") { // if it's not an unknown event error
			return serr
		}
		threadID = "" // unknown event edge case - remove existing thread ID to avoid complications
		newThread = true
	}
	if threadID == "" {
		threadID = eventID
		ctx = threadIDToContext(ctx, threadID)
	}

	b.setThreadID(ctx, roomID, eml.MessageID, threadID)
	b.setLastEventID(ctx, roomID, threadID, eventID)

	if newThread && cfg.Threadify() {
		// if automatic stripping is enabled, there is a chance something important may be stripped out
		// to prevent that, we use a hacky way to generate content without stripping and save it as a file fist
		if cfg.Stripify() {
			contentOpts := cfg.ContentOptions()
			contentOpts.Stripify = false
			content := eml.ContentBody(threadID, contentOpts)
			eml.Files = append(eml.Files, //nolint:forcetypeassert // that's ok
				utils.NewFile("original.md", []byte(content.Parsed.(*event.MessageEventContent).Body)), //nolint:errcheck // that's ok
			)
		}
		_, berr := b.lp.Send(ctx, roomID, eml.ContentBody(threadID, cfg.ContentOptions()))
		if berr != nil {
			return berr
		}
	}

	if !cfg.NoInlines() {
		b.sendFiles(ctx, roomID, eml.InlineFiles, cfg.NoThreads(), threadID)
	}

	if !cfg.NoFiles() {
		b.sendFiles(ctx, roomID, eml.Files, cfg.NoThreads(), threadID)
	}

	if newThread && cfg.Autoreply() != "" {
		b.sendAutoreply(ctx, roomID, threadID)
	}

	return nil
}

//nolint:gocognit // TODO
func (b *Bot) sendAutoreply(ctx context.Context, roomID id.RoomID, threadID id.EventID) {
	cfg, err := b.cfg.GetRoom(ctx, roomID)
	if err != nil {
		return
	}

	text := cfg.Autoreply()
	if text == "" {
		return
	}

	threadEvt, err := b.lp.GetClient().GetEvent(ctx, roomID, threadID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot get thread event for autoreply")
		return
	}

	evt := &event.Event{
		ID:     id.EventID(fmt.Sprintf("%s-autoreply-%s", threadID, time.Now().UTC().Format("20060102T150405Z"))),
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

	meta := b.getParentEmail(ctx, evt, cfg.Mailbox())

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
	data := eml.Compose(b.cfg.GetBot(ctx).DKIMPrivateKey())
	if data == "" {
		return
	}

	var queued bool
	ctx = newContext(ctx, threadEvt)
	recipients := meta.Recipients
	for _, to := range recipients {
		queued, err = b.Sendmail(ctx, evt.ID, meta.From, to, data, cfg.Relay())
		if queued {
			b.log.Info().Err(err).Str("from", meta.From).Str("to", to).Msg("email has been queued")
			b.saveSentMetadata(ctx, queued, meta.ThreadID, to, eml, cfg, "Autoreply has been sent to "+to+" (queued)")
			continue
		}

		if err != nil {
			b.Error(ctx, "cannot send email to %q: %v", to, err)
			continue
		}

		b.saveSentMetadata(ctx, queued, meta.ThreadID, to, eml, cfg, "Autoreply has been sent to "+to)
	}
}

func (b *Bot) canReply(ctx context.Context) bool {
	evt := eventFromContext(ctx)
	return b.allowSend(ctx, evt.Sender, evt.RoomID) && b.allowReply(ctx, evt.Sender, evt.RoomID)
}

// SendEmailReply sends replies from matrix thread to email thread
//
//nolint:gocognit // TODO
func (b *Bot) SendEmailReply(ctx context.Context) {
	evt := eventFromContext(ctx)
	if !b.canReply(ctx) {
		return
	}
	cfg, err := b.cfg.GetRoom(ctx, evt.RoomID)
	if err != nil {
		b.Error(ctx, "cannot retrieve room settings: %v", err)
		return
	}
	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.Error(ctx, "mailbox is not configured, kupo")
		return
	}

	b.lock(ctx, evt.RoomID, evt.ID)
	defer b.unlock(ctx, evt.RoomID, evt.ID)

	meta := b.getParentEmail(ctx, evt, mailbox)

	if meta.To == "" {
		b.Error(ctx, "cannot find parent email and continue the thread. Please, start a new email thread")
		return
	}

	if meta.ThreadID == "" {
		meta.ThreadID = b.getThreadID(ctx, evt.RoomID, meta.InReplyTo, meta.References)
		ctx = threadIDToContext(ctx, meta.ThreadID)
	}
	content := evt.Content.AsMessage()
	b.clearReply(content)
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
	data := eml.Compose(b.cfg.GetBot(ctx).DKIMPrivateKey())
	if data == "" {
		b.lp.SendNotice(ctx, evt.RoomID, "email body is empty", linkpearl.RelatesTo(meta.ThreadID, cfg.NoThreads()))
		return
	}

	var queued bool
	recipients := meta.Recipients
	for _, to := range recipients {
		queued, err = b.Sendmail(ctx, evt.ID, meta.From, to, data, cfg.Relay())
		if queued {
			b.log.Info().Err(err).Str("from", meta.From).Str("to", to).Msg("email has been queued")
			b.saveSentMetadata(ctx, queued, meta.ThreadID, to, eml, cfg)
			continue
		}

		if err != nil {
			b.Error(ctx, "cannot send email to %q: %v", to, err)
			continue
		}

		b.saveSentMetadata(ctx, queued, meta.ThreadID, to, eml, cfg)
	}
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
	recipients[email.Address(e.From)] = struct{}{}

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
		rcpts = append(rcpts, email.Address(rcpt))
	}

	e.Recipients = rcpts
}

func (b *Bot) getParentEvent(ctx context.Context, evt *event.Event) (id.EventID, *event.Event) {
	content := evt.Content.AsMessage()
	threadID := linkpearl.EventParent(evt.ID, content)
	b.log.Debug().Str("eventID", evt.ID.String()).Str("threadID", threadID.String()).Msg("looking up for the parent event within thread")
	if threadID == evt.ID {
		b.log.Debug().Str("eventID", evt.ID.String()).Msg("event is the thread itself")
		return threadID, evt
	}
	lastEventID := b.getLastEventID(ctx, evt.RoomID, threadID)
	b.log.Debug().Str("eventID", evt.ID.String()).Str("threadID", threadID.String()).Str("lastEventID", lastEventID.String()).Msg("the last event of the thread (and parent of the event) has been found")
	if lastEventID == evt.ID {
		return threadID, evt
	}
	parentEvt, err := b.lp.GetClient().GetEvent(ctx, evt.RoomID, lastEventID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot get parent event")
		return threadID, nil
	}
	linkpearl.ParseContent(parentEvt, b.log)

	if ok, _ := b.lp.GetMachine().StateStore.IsEncrypted(ctx, evt.RoomID); !ok { //nolint:errcheck // that's fine
		return threadID, parentEvt
	}

	decrypted, err := b.lp.GetClient().Crypto.Decrypt(ctx, parentEvt)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot decrypt parent event")
		return threadID, nil
	}

	return threadID, decrypted
}

func (b *Bot) getParentEmail(ctx context.Context, evt *event.Event, newFromMailbox string) *parentEmail {
	parent := &parentEmail{}
	threadID, parentEvt := b.getParentEvent(ctx, evt)
	parent.ThreadID = threadID
	if parentEvt == nil {
		return parent
	}
	if parentEvt.ID == evt.ID {
		return parent
	}

	parent.From = email.Address(linkpearl.EventField[string](&parentEvt.Content, eventFromKey))
	parent.To = email.Address(linkpearl.EventField[string](&parentEvt.Content, eventToKey))
	parent.CC = email.Address(linkpearl.EventField[string](&parentEvt.Content, eventCcKey))
	parent.RcptTo = email.Address(linkpearl.EventField[string](&parentEvt.Content, eventRcptToKey))
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
func (b *Bot) saveSentMetadata(ctx context.Context, queued bool, threadID id.EventID, to string, eml *email.Email, cfg config.Room, textOverride ...string) {
	text := "Email has been sent to " + to
	if queued {
		text = "Email to " + to + " has been queued"
	}
	if len(textOverride) > 0 {
		text = textOverride[0]
	}

	evt := eventFromContext(ctx)
	content := eml.Content(threadID, cfg.ContentOptions(), b.psdc)
	notice := format.RenderMarkdown(text, true, true)
	msgContent, ok := content.Parsed.(*event.MessageEventContent)
	if !ok {
		b.Error(ctx, "cannot parse message")
		return
	}
	msgContent.MsgType = event.MsgNotice
	msgContent.Body = notice.Body
	msgContent.FormattedBody = notice.FormattedBody
	msgContent.RelatesTo = linkpearl.RelatesTo(threadID, cfg.NoThreads())
	content.Parsed = msgContent
	msgID, err := b.lp.Send(ctx, evt.RoomID, content)
	if err != nil {
		b.Error(ctx, "cannot send notice: %v", err)
		return
	}
	domain := utils.SanitizeDomain(cfg.Domain())
	b.setThreadID(ctx, evt.RoomID, email.MessageID(evt.ID, domain), threadID)
	b.setThreadID(ctx, evt.RoomID, email.MessageID(msgID, domain), threadID)
	b.setLastEventID(ctx, evt.RoomID, threadID, msgID)
}

func (b *Bot) sendFiles(ctx context.Context, roomID id.RoomID, files []*utils.File, noThreads bool, parentID id.EventID) {
	for _, file := range files {
		req := file.Convert()
		err := b.lp.SendFile(ctx, roomID, req, file.MsgType, linkpearl.RelatesTo(parentID, noThreads))
		if err != nil {
			b.Error(ctx, "cannot upload file %s: %v", req.FileName, err)
		}
	}
}

func (b *Bot) getThreadID(ctx context.Context, roomID id.RoomID, messageID, references string) id.EventID {
	refs := []string{messageID}
	if references != "" {
		refs = append(refs, strings.Split(references, " ")...)
	}

	for _, refID := range refs {
		key := acMessagePrefix + "." + refID
		data, err := b.lp.GetRoomAccountData(ctx, roomID, key)
		if err != nil {
			b.log.Error().Err(err).Str("key", key).Msg("cannot retrieve thread ID")
			continue
		}
		if data["eventID"] == "" {
			continue
		}
		resp, err := b.lp.GetClient().GetEvent(ctx, roomID, id.EventID(data["eventID"]))
		if err != nil {
			b.log.Warn().Err(err).Str("roomID", roomID.String()).Str("eventID", data["eventID"]).Msg("cannot get event by id (may be removed)")
			continue
		}
		return resp.ID
	}

	return ""
}

func (b *Bot) setThreadID(ctx context.Context, roomID id.RoomID, messageID string, eventID id.EventID) {
	key := acMessagePrefix + "." + messageID
	err := b.lp.SetRoomAccountData(ctx, roomID, key, map[string]string{"eventID": eventID.String()})
	if err != nil {
		b.log.Error().Err(err).Str("key", key).Msg("cannot save thread ID")
	}
}

func (b *Bot) getLastEventID(ctx context.Context, roomID id.RoomID, threadID id.EventID) id.EventID {
	key := acLastEventPrefix + "." + threadID.String()
	data, err := b.lp.GetRoomAccountData(ctx, roomID, key)
	if err != nil {
		b.log.Error().Err(err).Str("key", key).Msg("cannot retrieve last event ID")
		return threadID
	}
	if data["eventID"] != "" {
		return id.EventID(data["eventID"])
	}

	return threadID
}

func (b *Bot) setLastEventID(ctx context.Context, roomID id.RoomID, threadID, eventID id.EventID) {
	key := acLastEventPrefix + "." + threadID.String()
	err := b.lp.SetRoomAccountData(ctx, roomID, key, map[string]string{"eventID": eventID.String()})
	if err != nil {
		b.log.Error().Err(err).Str("key", key).Msg("cannot save thread ID")
	}
}

// clearReply removes quotation of previous message in reply message, to avoid confusion
func (b *Bot) clearReply(content *event.MessageEventContent) {
	index := strings.Index(content.Body, "> <@")
	formattedIndex := strings.Index(content.FormattedBody, "</mx-reply>")
	if index >= 0 {
		index = strings.Index(content.Body, "\n\n")
		// 2 is length of "\n\n"
		content.Body = content.Body[index+2:]
	}

	if formattedIndex >= 0 {
		// 11 is length of "</mx-reply>"
		content.FormattedBody = content.FormattedBody[formattedIndex+11:]
	}
}
