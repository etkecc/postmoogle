package email

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/jhillyerd/enmime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Email object
type Email struct {
	Date       string
	MessageID  string
	InReplyTo  string
	References string
	From       string
	To         string
	RcptTo     string
	CC         string
	Subject    string
	Text       string
	HTML       string
	Files      []*utils.File
}

// New constructs Email object
func New(messageID, inReplyTo, references, subject, from, to, text, html string, files []*utils.File) *Email {
	email := &Email{
		Date:       dateNow(),
		MessageID:  messageID,
		InReplyTo:  inReplyTo,
		References: references,
		From:       from,
		To:         to,
		RcptTo:     to,
		Subject:    subject,
		Text:       text,
		HTML:       html,
		Files:      files,
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
		files = append(files, file)
	}

	email := &Email{
		Date:       date,
		MessageID:  envelope.GetHeader("Message-Id"),
		InReplyTo:  envelope.GetHeader("In-Reply-To"),
		References: envelope.GetHeader("References"),
		From:       envelope.GetHeader("From"),
		To:         envelope.GetHeader("To"),
		RcptTo:     rcptto,
		CC:         envelope.GetHeader("Cc"),
		Subject:    envelope.GetHeader("Subject"),
		Text:       envelope.Text,
		HTML:       html,
		Files:      files,
	}

	return email
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
	var text strings.Builder
	if options.Sender {
		text.WriteString(e.From)
	}
	if options.Recipient {
		text.WriteString(" ➡️ ")
		text.WriteString(e.To)
	}
	if options.CC && e.CC != "" {
		text.WriteString("\ncc: ")
		text.WriteString(e.CC)
	}
	if options.Sender || options.Recipient || options.CC {
		text.WriteString("\n\n")
	}
	if options.Subject && threadID == "" {
		text.WriteString("# ")
		text.WriteString(e.Subject)
		text.WriteString("\n\n")
	}
	if e.HTML != "" && options.HTML {
		text.WriteString(format.HTMLToMarkdown(e.HTML))
	} else {
		text.WriteString(e.Text)
	}

	parsed := format.RenderMarkdown(text.String(), true, true)
	parsed.RelatesTo = utils.RelatesTo(options.Threads, threadID)

	content := event.Content{
		Raw: map[string]interface{}{
			options.MessageIDKey:  e.MessageID,
			options.InReplyToKey:  e.InReplyTo,
			options.ReferencesKey: e.References,
			options.SubjectKey:    e.Subject,
			options.RcptToKey:     e.RcptTo,
			options.FromKey:       e.From,
			options.ToKey:         e.To,
			options.CcKey:         e.CC,
		},
		Parsed: &parsed,
	}
	return &content
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
	signer := parsedkey.(crypto.Signer)

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
