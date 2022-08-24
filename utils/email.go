package utils

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
