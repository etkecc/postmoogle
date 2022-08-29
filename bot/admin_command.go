package bot

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/id"
)

func (b *Bot) sendMailboxes(ctx context.Context) {
	evt := eventFromContext(ctx)

	mailboxes := map[string]id.RoomID{}

	b.rooms.Range(func(mailbox any, roomID any) bool {
		mailboxes[mailbox.(string)] = roomID.(id.RoomID)
		return true
	})

	if len(mailboxes) == 0 {
		b.Notice(ctx, evt.RoomID, "No mailboxes are managed by the bot so far, kupo!")
		return
	}

	var msg strings.Builder
	msg.WriteString("The following mailboxes are managed by the bot:\n")
	for mailbox, roomID := range mailboxes {
		email := fmt.Sprintf("%s@%s", mailbox, b.domain)
		msg.WriteString("* `")
		msg.WriteString(email)
		msg.WriteString("` - `")
		msg.WriteString(roomID.String())
		msg.WriteString("`")
		msg.WriteString("\n")
	}

	b.Notice(ctx, evt.RoomID, msg.String())
}
