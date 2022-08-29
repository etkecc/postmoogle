package bot

import (
	"context"
	"strings"

	"maunium.net/go/mautrix/id"
)

func (b *Bot) sendMailboxes(ctx context.Context) {
	evt := eventFromContext(ctx)
	mailboxes := map[string]id.RoomID{}
	b.rooms.Range(func(key any, value any) bool {
		if key == nil {
			return true
		}
		if value == nil {
			return true
		}

		mailbox, ok := key.(string)
		if !ok {
			return true
		}
		roomID, ok := value.(id.RoomID)
		if !ok {
			return true
		}

		mailboxes[mailbox] = roomID
		return true
	})

	if len(mailboxes) == 0 {
		b.SendNotice(ctx, evt.RoomID, "No mailboxes are managed by the bot so far, kupo!")
		return
	}

	var msg strings.Builder
	msg.WriteString("The following mailboxes are managed by the bot:\n")
	for mailbox, roomID := range mailboxes {
		msg.WriteString("* `")
		msg.WriteString(mailbox)
		msg.WriteString("@")
		msg.WriteString(b.domain)
		msg.WriteString("` - `")
		msg.WriteString(roomID.String())
		msg.WriteString("`")
		msg.WriteString("\n")
	}

	b.SendNotice(ctx, evt.RoomID, msg.String())
}
