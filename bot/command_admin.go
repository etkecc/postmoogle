package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

func (b *Bot) sendMailboxes(ctx context.Context) {
	evt := eventFromContext(ctx)
	mailboxes := map[string]settings{}
	slice := []string{}
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
		config, err := b.getSettings(roomID)
		if err != nil {
			b.log.Error("cannot retrieve settings: %v", err)
		}

		mailboxes[mailbox] = config
		slice = append(slice, mailbox)
		return true
	})
	sort.Strings(slice)

	if len(slice) == 0 {
		b.SendNotice(ctx, evt.RoomID, "No mailboxes are managed by the bot so far, kupo!")
		return
	}

	var msg strings.Builder
	msg.WriteString("The following mailboxes are managed by the bot:\n")
	for _, mailbox := range slice {
		cfg := mailboxes[mailbox]
		msg.WriteString("* `")
		msg.WriteString(mailbox)
		msg.WriteString("@")
		msg.WriteString(b.domain)
		msg.WriteString("` by ")
		msg.WriteString(cfg.Owner())
		msg.WriteString("\n")
	}

	b.SendNotice(ctx, evt.RoomID, msg.String())
}

func (b *Bot) runDelete(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Usage: `%s delete MAILBOX`", b.prefix))
		return
	}
	mailbox := utils.Mailbox(commandSlice[1])

	v, ok := b.rooms.Load(mailbox)
	if v == nil || !ok {
		b.SendError(ctx, evt.RoomID, "mailbox does not exists, kupo")
		return
	}
	roomID := v.(id.RoomID)

	b.rooms.Delete(mailbox)
	err := b.setSettings(roomID, settings{})
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "mailbox has been deleted")
}
