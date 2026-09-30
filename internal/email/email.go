package email

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/etkecc/go-linkpearl"
	"github.com/jhillyerd/enmime/v2"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/etkecc/postmoogle/internal/utils"
)

// Email object
type Email struct {
	Date        string
	Sent        time.Time // time from the Date header, zero if unknown
	MessageID   string
	InReplyTo   string
	References  string
	From        string
	FromName    string
	To          string
	RcptTo      string
	CC          []string
	Subject     string
	Text        string
	HTML        string
	Files       []*utils.File
	InlineFiles []*utils.File
	Images      map[string]*Image // uploaded images by their source in the HTML, see SetImage

	originalHTML string // HTML as received, before any cleanup
	truncated    bool   // the last rendered content was shortened, see Truncated
}

// New constructs Email object
func New(messageID, inReplyTo, references, subject, from, to, rcptto, cc, text, html string, files, inline []*utils.File) *Email {
	email := &Email{
		Date:        dateNow(),
		MessageID:   messageID,
		InReplyTo:   inReplyTo,
		References:  references,
		From:        Address(from),
		To:          Address(to),
		CC:          AddressList(cc),
		RcptTo:      Address(rcptto),
		Subject:     subject,
		Text:        text,
		HTML:        html,
		Files:       files,
		InlineFiles: inline,
	}

	if html != "" {
		html = styleRegex.ReplaceAllString(html, "")
		email.HTML = html
	}

	return email
}

// FromEnvelope constructs Email object from envelope
func FromEnvelope(rcptto string, envelope *enmime.Envelope) *Email {
	datetime, _ := envelope.Date() //nolint:errcheck // handled in dateNow()
	date := dateNow(datetime)

	var html string
	if envelope.HTML != "" {
		html = styleRegex.ReplaceAllString(envelope.HTML, "")
	}

	files := make([]*utils.File, 0, len(envelope.Attachments))
	for _, attachment := range envelope.Attachments {
		file := utils.NewFile(attachment.FileName, attachment.Content)
		// some mail apps mark images shown in the text as attachments, the HTML still links them by cid:
		file.ContentID = attachment.ContentID
		files = append(files, file)
	}

	inlines := make([]*utils.File, 0, len(envelope.Inlines))
	for _, inline := range envelope.Inlines {
		file := utils.NewFile(inline.FileName, inline.Content)
		file.ContentID = inline.ContentID
		inlines = append(inlines, file)
	}
	// images of multipart/related emails often have a Content-ID but no Content-Disposition
	for _, part := range envelope.OtherParts {
		if part.ContentID == "" {
			continue
		}
		file := utils.NewFile(part.FileName, part.Content)
		file.ContentID = part.ContentID
		inlines = append(inlines, file)
	}

	email := &Email{
		Date:         date,
		Sent:         datetime,
		MessageID:    envelope.GetHeader("Message-Id"),
		InReplyTo:    envelope.GetHeader("In-Reply-To"),
		References:   envelope.GetHeader("References"),
		From:         Address(envelope.GetHeader("From")),
		FromName:     senderName(envelope),
		To:           Address(envelope.GetHeader("To")),
		RcptTo:       Address(rcptto),
		CC:           AddressList(envelope.GetHeader("Cc")),
		Subject:      envelope.GetHeader("Subject"),
		Text:         envelope.Text,
		HTML:         html,
		Files:        files,
		InlineFiles:  inlines,
		originalHTML: envelope.HTML,
	}

	return email
}

// senderName returns the display name of the email sender, e.g. "Jane Doe" for "Jane Doe" <jane@example.com>
func senderName(envelope *enmime.Envelope) string {
	from, err := envelope.AddressList("From")
	if err != nil || len(from) == 0 {
		return ""
	}
	return strings.TrimSpace(from[0].Name)
}

// Mailbox returns postmoogle's mailbox, parsing it from FROM (if incoming=false) or TO (incoming=true)
func (e *Email) Mailbox(incoming bool) string {
	if incoming {
		return utils.Mailbox(e.RcptTo)
	}
	return utils.Mailbox(e.From)
}

// Content converts the email object to a Matrix event content
func (e *Email) Content(threadID id.EventID, options *ContentOptions) *event.Content {
	msg := &message{
		raw:       e.raw(options),
		relatesTo: linkpearl.RelatesTo(threadID, !options.Threads),
	}
	msg.headerHTML, msg.headerText = e.header(threadID, options)
	if threadID != "" || !options.Threadify {
		msg.body = e.body(options, options.Stripify && threadID != "") // strip only in thread replies
	}

	var content *event.Content
	content, e.truncated = msg.render()
	return content
}

// ContentBody converts the email to Matrix event content with only the body; nil if threadify is disabled
func (e *Email) ContentBody(threadID id.EventID, options *ContentOptions) *event.Content {
	if !options.Threadify {
		return nil
	}
	msg := &message{
		body:      e.body(options, options.Stripify),
		relatesTo: linkpearl.RelatesTo(threadID, !options.Threads),
	}

	var content *event.Content
	content, e.truncated = msg.render()
	return content
}

// Truncated reports whether the last content returned by Content or ContentBody was shortened to fit Matrix limits
func (e *Email) Truncated() bool {
	return e.truncated
}

// FullVersion returns the complete email body as a file, to be sent along with a truncated message
func (e *Email) FullVersion() *utils.File {
	if e.originalHTML != "" {
		return utils.NewFile("email.html", []byte(e.originalHTML))
	}
	if e.HTML != "" {
		return utils.NewFile("email.html", []byte(e.HTML))
	}
	return utils.NewFile("email.txt", []byte(e.Text))
}

// raw returns the email metadata stored in the event, used to reply and to thread emails
func (e *Email) raw(options *ContentOptions) map[string]any {
	var cc string
	if len(e.CC) > 0 {
		cc = strings.Join(e.CC, ", ")
	}

	return map[string]any{
		options.MessageIDKey:  e.MessageID,
		options.InReplyToKey:  e.InReplyTo,
		options.ReferencesKey: e.References,
		options.SubjectKey:    e.Subject,
		options.RcptToKey:     e.RcptTo,
		options.FromKey:       e.From,
		options.ToKey:         e.To,
		options.CcKey:         cc,
	}
}

// Compose converts the email object to a string (to be used for delivery via SMTP) and possibly DKIM-signs it
func (e *Email) Compose(privkey string) string {
	textSize := len(e.Text)
	htmlSize := len(e.HTML)
	if textSize == 0 && htmlSize == 0 {
		return ""
	}

	mail := enmime.Builder().
		From("", e.From).
		To("", e.To).
		Header("Message-Id", e.MessageID).
		Header("X-PM-Tag", e.From).
		Subject(e.Subject)
	if textSize > 0 {
		mail = mail.Text([]byte(e.Text))
	}
	if htmlSize > 0 {
		mail = mail.HTML([]byte(e.HTML))
	}
	if e.InReplyTo != "" {
		mail = mail.Header("In-Reply-To", e.InReplyTo)
	}
	if e.References != "" {
		mail = mail.Header("References", e.References)
	}
	if len(e.CC) > 0 {
		for _, addr := range e.CC {
			mail = mail.CC("", addr)
		}
	}

	root, err := mail.Build()
	if err != nil {
		return ""
	}
	var data strings.Builder
	err = root.Encode(&data)
	if err != nil {
		return ""
	}

	domain := strings.SplitN(e.From, "@", 2)[1]
	return e.sign(domain, privkey, data)
}

func (e *Email) sign(domain, privkey string, data strings.Builder) string {
	if privkey == "" {
		return data.String()
	}
	pemblock, _ := pem.Decode([]byte(privkey))
	if pemblock == nil {
		return data.String()
	}
	parsedkey, err := x509.ParsePKCS8PrivateKey(pemblock.Bytes)
	if err != nil {
		return data.String()
	}
	signer, ok := parsedkey.(crypto.Signer)
	if !ok {
		return data.String()
	}

	options := &dkim.SignOptions{
		Domain:   domain,
		Selector: "postmoogle",
		Signer:   signer,
	}

	var msg strings.Builder
	err = dkim.Sign(&msg, strings.NewReader(data.String()), options)
	if err != nil {
		return data.String()
	}

	return msg.String()
}
