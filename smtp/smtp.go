package smtp

import (
	"context"

	"maunium.net/go/mautrix/id"
)

// Client interface to send emails
type Client interface {
	GetMappings(context.Context) (map[string]id.RoomID, error)
	Send(from, mailbox, subject, body string) error
}
