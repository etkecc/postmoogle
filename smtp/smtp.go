package smtp

import (
	"context"

	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/id"
)

// Client interface to send emails
type Client interface {
	GetMapping(context.Context, string) (id.RoomID, bool)
	Send(ctx context.Context, from, mailbox, subject, body string, files []*utils.File) error
}
