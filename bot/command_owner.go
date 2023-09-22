package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/raja/argon2pw"
	"golang.org/x/exp/slices"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/utils"
)

func (b *Bot) runStop(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	mailbox := cfg.Get(config.RoomMailbox)
	if mailbox == "" {
		b.SendNotice(ctx, evt.RoomID, "that room is not configured yet")
		return
	}

	b.rooms.Delete(mailbox)

	err = b.cfg.SetRoom(evt.RoomID, config.Room{})
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
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if name == commandSpamlist {
		name = config.RoomSpamlist
	}

	value := cfg.Get(name)
	if value == "" {
		msg := fmt.Sprintf("`%s` is not set, kupo.\n"+
			"To set it, send a `%s %s VALUE` command.",
			name, b.prefix, name)
		b.SendNotice(ctx, evt.RoomID, msg)
		return
	}

	if name == config.RoomMailbox {
		value = utils.EmailsList(value, cfg.Domain())
	}

	msg := fmt.Sprintf("`%s` of this room is `%s`\n"+
		"To set it to a new value, send a `%s %s VALUE` command.",
		name, value, b.prefix, name)
	if name == config.RoomPassword {
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
	// ignore request
	if name == config.RoomActive {
		return
	}
	if name == config.RoomMailbox {
		existingID, ok := b.getMapping(value)
		if (ok && existingID != "" && existingID != evt.RoomID) || b.isReserved(value) {
			b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s` (%s) already taken, kupo", value, utils.EmailsList(value, "")))
			return
		}
	}

	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if name == config.RoomPassword {
		value = b.parseCommand(evt.Content.AsMessage().Body, false)[1] // get original value, without forced lower case
		value, err = argon2pw.GenerateSaltedHash(value)
		if err != nil {
			b.Error(ctx, evt.RoomID, "failed to hash password: %v", err)
			return
		}
	}

	old := cfg.Get(name)
	cfg.Set(name, value)

	if name == config.RoomMailbox {
		cfg.Set(config.RoomOwner, evt.Sender.String())
		if old != "" {
			b.rooms.Delete(old)
		}
		active := b.ActivateMailbox(evt.Sender, evt.RoomID, value)
		cfg.Set(config.RoomActive, strconv.FormatBool(active))
		value = fmt.Sprintf("%s@%s", value, utils.SanitizeDomain(cfg.Domain()))
	}

	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	msg := fmt.Sprintf("`%s` of this room set to `%s`", name, value)
	if name == config.RoomPassword {
		msg = "SMTP password has been set"
	}
	b.SendNotice(ctx, evt.RoomID, msg)
}

func (b *Bot) runSpamlistAdd(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.getOption(ctx, config.RoomSpamlist)
		return
	}
	roomCfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot get room settings: %v", err)
		return
	}
	spamlist := utils.StringSlice(roomCfg[config.RoomSpamlist])
	for _, newItem := range commandSlice[1:] {
		newItem = strings.TrimSpace(newItem)
		if slices.Contains(spamlist, newItem) {
			continue
		}
		spamlist = append(spamlist, newItem)
	}

	roomCfg.Set(config.RoomSpamlist, utils.SliceString(spamlist))
	err = b.cfg.SetRoom(evt.RoomID, roomCfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot store room settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "spamlist has been updated, kupo")
}

func (b *Bot) runSpamlistRemove(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.getOption(ctx, config.RoomSpamlist)
		return
	}
	roomCfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot get room settings: %v", err)
		return
	}
	toRemove := map[int]struct{}{}
	spamlist := utils.StringSlice(roomCfg[config.RoomSpamlist])
	for _, item := range commandSlice[1:] {
		item = strings.TrimSpace(item)
		idx := slices.Index(spamlist, item)
		if idx < 0 {
			continue
		}
		toRemove[idx] = struct{}{}
	}
	if len(toRemove) == 0 {
		b.SendNotice(ctx, evt.RoomID, "nothing new, kupo.")
		return
	}

	updatedSpamlist := []string{}
	for i, item := range spamlist {
		if _, ok := toRemove[i]; ok {
			continue
		}
		updatedSpamlist = append(updatedSpamlist, item)
	}

	roomCfg.Set(config.RoomSpamlist, utils.SliceString(updatedSpamlist))
	err = b.cfg.SetRoom(evt.RoomID, roomCfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot store room settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "spamlist has been updated, kupo")
}

func (b *Bot) runSpamlistReset(ctx context.Context) {
	evt := eventFromContext(ctx)
	roomCfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot get room settings: %v", err)
		return
	}
	spamlist := utils.StringSlice(roomCfg[config.RoomSpamlist])
	if len(spamlist) == 0 {
		b.SendNotice(ctx, evt.RoomID, "spamlist is empty, kupo.")
		return
	}

	roomCfg.Set(config.RoomSpamlist, "")
	err = b.cfg.SetRoom(evt.RoomID, roomCfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot store room settings: %v", err)
		return
	}

	b.SendNotice(ctx, evt.RoomID, "spamlist has been reset, kupo.")
}
