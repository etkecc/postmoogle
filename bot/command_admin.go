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
	mailboxes := map[string]roomSettings{}
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
		config, err := b.getRoomSettings(roomID)
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
	err := b.setRoomSettings(roomID, roomSettings{})
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "mailbox has been deleted")
}

func (b *Bot) runUsers(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.getBotSettings()
	if len(commandSlice) < 2 {
		var msg strings.Builder
		users := cfg.Users()
		if len(users) > 0 {
			msg.WriteString("Currently: `")
			msg.WriteString(strings.Join(users, " "))
			msg.WriteString("`\n\n")
		}
		msg.WriteString("Usage: `")
		msg.WriteString(b.prefix)
		msg.WriteString(" users PATTERN1 PATTERN2 PATTERN3...`")
		msg.WriteString("where each pattern is like `@someone:example.com`, ")
		msg.WriteString("`@bot.*:example.com`, `@*:another.com`, or `@*:*`\n")

		b.SendNotice(ctx, evt.RoomID, msg.String())
		return
	}

	_, homeserver, err := b.lp.GetClient().UserID.Parse()
	if err != nil {
		b.SendError(ctx, evt.RoomID, fmt.Sprintf("invalid userID: %v", err))
	}

	patterns := commandSlice[1:]
	allowedUsers, err := parseMXIDpatterns(patterns, "@*:"+homeserver)
	if err != nil {
		b.SendError(ctx, evt.RoomID, fmt.Sprintf("invalid patterns: %v", err))
		return
	}

	cfg.Set(botOptionUsers, strings.Join(patterns, " "))

	err = b.setBotSettings(cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot set bot config: %v", err)
	}
	b.allowedUsers = allowedUsers
	b.SendNotice(ctx, evt.RoomID, "allowed users updated")
}
