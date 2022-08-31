package bot

import (
	"context"
	"fmt"
	"strings"
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
		var msg strings.Builder
		msg.WriteString(fmt.Sprintf("`%s` is not set, kupo.", name))
		msg.WriteString("\n")
		msg.WriteString(fmt.Sprintf(
			"To set it, send a `%s %s VALUE` command.",
			b.prefix,
			name,
		))
		b.SendNotice(ctx, evt.RoomID, msg.String())
		return
	}

	if name == roomOptionMailbox {
		value = value + "@" + b.domain
	}

	var msg strings.Builder
	msg.WriteString(fmt.Sprintf("`%s` of this room is `%s`", name, value))
	msg.WriteString("\n")
	msg.WriteString(fmt.Sprintf(
		"To set it to a new value, send a `%s %s VALUE` command.",
		b.prefix,
		name,
	))
	b.SendNotice(ctx, evt.RoomID, msg.String())
}

func (b *Bot) setOption(ctx context.Context, name, value string) {
	cmd := b.commands.get(name)
	if cmd != nil {
		value = cmd.sanitizer(value)
	}

	evt := eventFromContext(ctx)
	if name == roomOptionMailbox {
		existingID, ok := b.GetMapping(value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s@%s` already taken, kupo", value, b.domain))
			return
		}
	}

	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	old := cfg.Get(name)
	cfg.Set(name, value)

	if name == roomOptionMailbox {
		cfg.Set(roomOptionOwner, evt.Sender.String())
		if old != "" {
			b.rooms.Delete(old)
		}
		b.rooms.Store(value, evt.RoomID)
		value = fmt.Sprintf("%s@%s", value, b.domain)
	}

	err = b.setRoomSettings(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	if name == roomOptionMailbox {
		value = value + "@" + b.domain
	}

	b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("`%s` of this room set to `%s`", name, value))
}
