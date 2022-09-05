package utils

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"time"

	"github.com/emersion/go-msgauth/dkim"
)

// MTA is mail transfer agent
type MTA interface {
	Send(from, to, data string) error
}

// Email object
type Email struct {
	data strings.Builder

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

// Compose converts email object to string and (optionally) signs it
func (e *Email) Compose(privkey string) string {
	domain := strings.SplitN(e.From, "@", 2)[0]

	e.data.WriteString("From: ")
	e.data.WriteString(e.From)
	e.data.WriteString("\r\n")

	e.data.WriteString("To: ")
	e.data.WriteString(e.To)
	e.data.WriteString("\r\n")

	e.data.WriteString("Message-Id: ")
	e.data.WriteString(e.MessageID)
	e.data.WriteString("\r\n")

	e.data.WriteString("Date: ")
	e.data.WriteString(e.Date)
	e.data.WriteString("\r\n")

	if e.InReplyTo != "" {
		e.data.WriteString("In-Reply-To: ")
		e.data.WriteString(e.InReplyTo)
		e.data.WriteString("\r\n")
	}

	e.data.WriteString("Subject: ")
	e.data.WriteString(e.Subject)
	e.data.WriteString("\r\n")

	e.data.WriteString("\r\n")

	e.data.WriteString(e.Text)
	e.data.WriteString("\r\n")

	e.sign(domain, privkey)
	return e.data.String()
}

func (e *Email) sign(domain, privkey string) {
	if privkey == "" {
		return
	}
	pemblock, _ := pem.Decode([]byte(privkey))
	if pemblock == nil {
		return
	}
	parsedkey, err := x509.ParsePKCS8PrivateKey(pemblock.Bytes)
	if err != nil {
		return
	}
	signer := parsedkey.(crypto.Signer)

	options := &dkim.SignOptions{
		Domain:   domain,
		Selector: "postmoogle",
		Signer:   signer,
	}

	var msg strings.Builder
	err = dkim.Sign(&msg, strings.NewReader(e.data.String()), options)
	if err != nil {
		return
	}

	e.data = msg
}
