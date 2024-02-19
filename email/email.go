package email

import (
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/emersion/go-msgauth/dkim"
	"github.com/jhillyerd/enmime"
	"gitlab.com/etke.cc/go/psd"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Email object
type Email struct {
	Date        string
	MessageID   string
	InReplyTo   string
	References  string
	From        string
	To          string
	RcptTo      string
	CC          []string
	Subject     string
	Text        string
	HTML        string
	Files       []*utils.File
	InlineFiles []*utils.File
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
		files = append(files, file)
	}

	inlines := make([]*utils.File, 0, len(envelope.Inlines))
	for _, inline := range envelope.Inlines {
		file := utils.NewFile(inline.FileName, inline.Content)
		inlines = append(inlines, file)
	}

	email := &Email{
		Date:        date,
		MessageID:   envelope.GetHeader("Message-Id"),
		InReplyTo:   envelope.GetHeader("In-Reply-To"),
		References:  envelope.GetHeader("References"),
		From:        Address(envelope.GetHeader("From")),
		To:          Address(envelope.GetHeader("To")),
		RcptTo:      Address(rcptto),
		CC:          AddressList(envelope.GetHeader("Cc")),
		Subject:     envelope.GetHeader("Subject"),
		Text:        envelope.Text,
		HTML:        html,
		Files:       files,
		InlineFiles: inlines,
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

func (e *Email) addressStatus(email string, psdc *psd.Client) string {
	targets, _ := psdc.Get(email) //nolint:errcheck // that's ok
	if len(targets) > 0 {
		return "👥" + targets[0].GetDomain() + " 👤"
	}
	return ""
}

func (e *Email) contentHeader(threadID id.EventID, text *strings.Builder, options *ContentOptions, psdc *psd.Client) {
	if options.Sender {
		text.WriteString(e.addressStatus(e.From, psdc))
		text.WriteString(e.From)
	}
	if options.Recipient {
		mailbox, sub, host := utils.EmailParts(e.To)
		text.WriteString(" ➡️ ")
		text.WriteString(mailbox)
		text.WriteString("@")
		text.WriteString(host)
		if sub != "" {
			text.WriteString(" (")
			text.WriteString(sub)
			text.WriteString(")")
		}
	}
	if options.CC && len(e.CC) > 0 {
		ccs := make([]string, 0, len(e.CC))
		for _, addr := range e.CC {
			ccs = append(ccs, e.addressStatus(addr, psdc)+addr)
		}
		text.WriteString("\ncc: ")
		text.WriteString(strings.Join(ccs, ", "))
	}
	if options.Sender || options.Recipient || options.CC {
		text.WriteString("\n\n")
	}
	if options.Subject && threadID == "" {
		if options.Threadify {
			text.WriteString("**")
			text.WriteString(e.Subject)
			text.WriteString("**")
		} else {
			text.WriteString("# ")
			text.WriteString(e.Subject)

		}
		text.WriteString("\n\n")
	}
}

// Content converts the email object to a Matrix event content
func (e *Email) Content(threadID id.EventID, options *ContentOptions, psdc *psd.Client) *event.Content {
	var text strings.Builder

	e.contentHeader(threadID, &text, options, psdc)

	if threadID != "" || (threadID == "" && !options.Threadify) {
		if e.HTML != "" && options.HTML {
			text.WriteString(format.HTMLToMarkdown(e.HTML))
		} else {
			text.WriteString(e.Text)
		}
	}

	parsed := format.RenderMarkdown(text.String(), true, true)
	parsed.RelatesTo = linkpearl.RelatesTo(threadID, !options.Threads)

	var cc string
	if len(e.CC) > 0 {
		cc = strings.Join(e.CC, ", ")
	}

	content := event.Content{
		Raw: map[string]any{
			options.MessageIDKey:  e.MessageID,
			options.InReplyToKey:  e.InReplyTo,
			options.ReferencesKey: e.References,
			options.SubjectKey:    e.Subject,
			options.RcptToKey:     e.RcptTo,
			options.FromKey:       e.From,
			options.ToKey:         e.To,
			options.CcKey:         cc,
		},
		Parsed: &parsed,
	}
	return &content
}

// ContentBody converts the email object to a Matrix event content that contains email body only
// NOTE: returns nil if threadify is disabled
func (e *Email) ContentBody(threadID id.EventID, options *ContentOptions) *event.Content {
	if !options.Threadify {
		return nil
	}
	var text string
	if e.HTML != "" && options.HTML {
		text = format.HTMLToMarkdown(e.HTML)
	} else {
		text = e.Text
	}

	parsed := format.RenderMarkdown(text, true, true)
	parsed.RelatesTo = linkpearl.RelatesTo(threadID, !options.Threads)

	content := event.Content{
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
