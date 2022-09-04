package utils

// MTA is mail transfer agent
type MTA interface {
	Send(from, to, data string) error
}

// Email object
type Email struct {
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
