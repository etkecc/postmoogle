package utils

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/jhillyerd/enmime"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

// IncomingFilteringOptions for incoming mail
type IncomingFilteringOptions interface {
	SpamcheckSMTP() bool
	SpamcheckMX() bool
	Spamlist() []string
}

// Email object
type Email struct {
	Date       string
	MessageID  string
	InReplyTo  string
	References string
	From       string
	To         string
	Subject    string
	Text       string
	HTML       string
	Files      []*File
}

// ContentOptions represents settings that specify how an email is to be converted to a Matrix message
type ContentOptions struct {
	// On/Off
	Sender    bool
	Recipient bool
	Subject   bool
	HTML      bool
	Threads   bool

	// Keys
	MessageIDKey  string
	InReplyToKey  string
	ReferencesKey string
	SubjectKey    string
	FromKey       string
	ToKey         string
}

// AddressValid checks if email address is valid
func AddressValid(email string) bool {
	_, err := mail.ParseAddress(email)
	return err == nil
}

// MessageID generates email Message-Id from matrix event ID
func MessageID(eventID id.EventID, domain string) string {
	return fmt.Sprintf("<%s@%s>", eventID, domain)
}

// NewEmail constructs Email object
func NewEmail(messageID, inReplyTo, references, subject, from, to, text, html string, files []*File) *Email {
	email := &Email{
		Date:       time.Now().UTC().Format(time.RFC1123Z),
		MessageID:  messageID,
		InReplyTo:  inReplyTo,
		References: references,
		From:       from,
		To:         to,
		Subject:    subject,
		Text:       text,
		HTML:       html,
		Files:      files,
	}

	if html != "" {
		var err error
		html, err = StripHTMLTag(html, "style")
		if err == nil {
			email.HTML = html
		}
	}

	return email
}

// Mailbox returns postmoogle's mailbox, parsing it from FROM (if incoming=false) or TO (incoming=true)
func (e *Email) Mailbox(incoming bool) string {
	if incoming {
		return Mailbox(e.To)
	}
	return Mailbox(e.From)
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
	if options.Sender || options.Recipient {
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
	parsed.RelatesTo = RelatesTo(options.Threads, threadID)

	content := event.Content{
		Raw: map[string]interface{}{
			options.MessageIDKey:  e.MessageID,
			options.InReplyToKey:  e.InReplyTo,
			options.ReferencesKey: e.References,
			options.SubjectKey:    e.Subject,
			options.FromKey:       e.From,
			options.ToKey:         e.To,
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
		log.Error("cannot compose email: %v", err)
		return ""
	}
	var data strings.Builder
	err = root.Encode(&data)
	if err != nil {
		log.Error("cannot encode email: %v", err)
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
