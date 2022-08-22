package smtp

import (
	"context"

	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/id"
)

// Client interface to send emails
type Client interface {
	GetMappings(context.Context) (map[string]id.RoomID, error)
	Send(ctx context.Context, from, mailbox, subject, body string, files []*utils.File) error
}
