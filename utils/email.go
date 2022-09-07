package utils

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

// MTA is mail transfer agent
type MTA interface {
	Send(from, to, data string) error
}

// Email object
type Email struct {
	Date      string
	MessageID string
	InReplyTo string
	From      string
	To        string
	Subject   string
	Text      string
	HTML      string
	Files     []*File
}

// ContentOptions represents settings that specify how an email is to be converted to a Matrix message
type ContentOptions struct {
	// On/Off
	Sender  bool
	Subject bool
	HTML    bool
	Threads bool

	// Keys
	MessageIDKey string
	InReplyToKey string
	SubjectKey   string
	FromKey      string
}

// NewEmail constructs Email object
func NewEmail(messageID, inReplyTo, subject, from, to, text, html string, files []*File) *Email {
	email := &Email{
		Date:      time.Now().UTC().Format(time.RFC1123Z),
		MessageID: messageID,
		InReplyTo: inReplyTo,
		From:      from,
		To:        to,
		Subject:   subject,
		Text:      text,
		HTML:      html,
		Files:     files,
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

// Content converts the email object to a Matrix event content
func (e *Email) Content(threadID id.EventID, options *ContentOptions) *event.Content {
	var text strings.Builder
	if options.Sender {
		text.WriteString("From: ")
		text.WriteString(e.From)
		text.WriteString("\n\n")
	}
	if options.Subject {
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
			options.MessageIDKey: e.MessageID,
			options.InReplyToKey: e.InReplyTo,
			options.SubjectKey:   e.Subject,
			options.FromKey:      e.From,
		},
		Parsed: parsed,
	}
	return &content
}

// Compose converts the email object to a string (to be used for delivery via SMTP) and possibly DKIM-signs it
func (e *Email) Compose(privkey string) string {
	var data strings.Builder

	domain := strings.SplitN(e.From, "@", 2)[1]
	data.WriteString("From: ")
	data.WriteString(e.From)
	data.WriteString("\r\n")

	data.WriteString("To: ")
	data.WriteString(e.To)
	data.WriteString("\r\n")

	data.WriteString("Message-Id: ")
	data.WriteString(e.MessageID)
	data.WriteString("\r\n")

	data.WriteString("Date: ")
	data.WriteString(e.Date)
	data.WriteString("\r\n")

	if e.InReplyTo != "" {
		data.WriteString("In-Reply-To: ")
		data.WriteString(e.InReplyTo)
		data.WriteString("\r\n")
	}

	data.WriteString("Subject: ")
	data.WriteString(e.Subject)
	data.WriteString("\r\n")

	data.WriteString("\r\n")

	data.WriteString(e.Text)
	data.WriteString("\r\n")

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
