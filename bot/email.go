package bot

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
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

func email2content(email *utils.Email, cfg roomSettings, threadID id.EventID) *event.Content {
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
			eventSubjectKey:   email.Subject,
			eventFromKey:      email.From,
		},
		Parsed: parsed,
	}
	return &content
}

// SetSMTPAuth sets dynamic login and password to auth against built-in smtp server
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

	content := email2content(email, cfg, threadID)
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

func (b *Bot) getBody(content *event.MessageEventContent) string {
	if content.FormattedBody != "" {
		return content.FormattedBody
	}

	return content.Body
}

func (b *Bot) getSubject(content *event.MessageEventContent) string {
	if content.Body == "" {
		return ""
	}

	return strings.SplitN(content.Body, "\n", 1)[0]
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
		subject = b.getSubject(content)
	}
	if body == "" {
		body = b.getBody(content)
	}

	var msg strings.Builder
	msg.WriteString("From: ")
	msg.WriteString(from)
	msg.WriteString("\r\n")

	msg.WriteString("To: ")
	msg.WriteString(to)
	msg.WriteString("\r\n")

	msg.WriteString("Message-Id: ")
	msg.WriteString(evt.ID.String()[1:] + "@" + b.domain)
	msg.WriteString("\r\n")

	msg.WriteString("Date: ")
	msg.WriteString(time.Now().UTC().Format(time.RFC1123Z))
	msg.WriteString("\r\n")

	if inReplyTo != "" {
		msg.WriteString("In-Reply-To: ")
		msg.WriteString(inReplyTo)
		msg.WriteString("\r\n")
	}

	msg.WriteString("Subject: ")
	msg.WriteString(subject)
	msg.WriteString("\r\n")

	msg.WriteString("\r\n")

	msg.WriteString(body)
	msg.WriteString("\r\n")

	msg = b.signDKIM(msg)

	return b.mta.Send(from, to, msg.String())
}

func (b *Bot) signDKIM(body strings.Builder) strings.Builder {
	privkey := b.getBotSettings().DKIMPrivateKey()
	if privkey == "" {
		b.log.Warn("DKIM private key not found, email will be sent unsigned")
		return body
	}
	pemblock, _ := pem.Decode([]byte(privkey))
	if pemblock == nil {
		b.log.Error("cannot decode DKIM private key")
		return body
	}
	parsedkey, err := x509.ParsePKCS8PrivateKey(pemblock.Bytes)
	if err != nil {
		b.log.Error("cannot parse PKCS8 private key: %v", err)
		return body
	}
	signer := parsedkey.(crypto.Signer)

	options := &dkim.SignOptions{
		Domain:   b.domain,
		Selector: "postmoogle",
		Signer:   signer,
	}

	var msg strings.Builder
	err = dkim.Sign(&msg, strings.NewReader(body.String()), options)
	if err != nil {
		b.log.Error("cannot sign email: %v", err)
		return body
	}

	return msg
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
