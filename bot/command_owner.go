package bot

import (
	"context"
	"fmt"

	"github.com/raja/argon2pw"
)

func (b *Bot) runStop(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	mailbox := cfg.Get(roomOptionMailbox)
	if mailbox == "" {
		b.SendNotice(ctx, evt.RoomID, "that room is not configured yet")
		return
	}

	b.rooms.Delete(mailbox)

	err = b.setRoomSettings(evt.RoomID, roomSettings{})
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "mailbox has been disabled")
}

func (b *Bot) handleOption(ctx context.Context, cmd []string) {
	if len(cmd) == 1 {
		b.getOption(ctx, cmd[0])
		return
	}
	b.setOption(ctx, cmd[0], cmd[1])
}

func (b *Bot) getOption(ctx context.Context, name string) {
	evt := eventFromContext(ctx)
	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	value := cfg.Get(name)
	if value == "" {
		msg := fmt.Sprintf("`%s` is not set, kupo.\n"+
			"To set it, send a `%s %s VALUE` command.",
			name, b.prefix, name)
		b.SendNotice(ctx, evt.RoomID, msg)
		return
	}

	if name == roomOptionMailbox {
		value = value + "@" + b.domains[0]
	}

	msg := fmt.Sprintf("`%s` of this room is `%s`\n"+
		"To set it to a new value, send a `%s %s VALUE` command.",
		name, value, b.prefix, name)
	if name == roomOptionPassword {
		msg = fmt.Sprintf("There is an SMTP password already set for this room/mailbox. "+
			"It's stored in a secure hashed manner, so we can't tell you what the original raw password was. "+
			"To find the raw password, try to find your old message which had originally set it, "+
			"or just set a new one with `%s %s NEW_PASSWORD`.",
			b.prefix, name)
	}
	b.SendNotice(ctx, evt.RoomID, msg)
}

//nolint:gocognit
func (b *Bot) setOption(ctx context.Context, name, value string) {
	cmd := b.commands.get(name)
	if cmd != nil && cmd.sanitizer != nil {
		value = cmd.sanitizer(value)
	}

	evt := eventFromContext(ctx)
	if name == roomOptionMailbox {
		existingID, ok := b.getMapping(value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s@%s` already taken, kupo", value, b.domains[0]))
			return
		}
	}

	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if name == roomOptionPassword {
		value = b.parseCommand(evt.Content.AsMessage().Body, false)[1] // get original value, without forced lower case
		value, err = argon2pw.GenerateSaltedHash(value)
		if err != nil {
			b.Error(ctx, evt.RoomID, "failed to hash password: %v", err)
			return
		}
	}

	old := cfg.Get(name)
	cfg.Set(name, value)

	if name == roomOptionMailbox {
		cfg.Set(roomOptionOwner, evt.Sender.String())
		if old != "" {
			b.rooms.Delete(old)
		}
		b.rooms.Store(value, evt.RoomID)
		value = fmt.Sprintf("%s@%s", value, b.domains[0])
	}

	err = b.setRoomSettings(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	msg := fmt.Sprintf("`%s` of this room set to `%s`", name, value)
	if name == roomOptionPassword {
		msg = "SMTP password has been set"
	}
	b.SendNotice(ctx, evt.RoomID, msg)
}
