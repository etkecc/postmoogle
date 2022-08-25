package smtp

import (
	"context"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

// Client interface to send emails
type Client interface {
	GetMapping(string) (id.RoomID, bool)
	Send(ctx context.Context, email *utils.Email) error
}
