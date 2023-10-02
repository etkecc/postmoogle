package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/raja/argon2pw"
	"gitlab.com/etke.cc/linkpearl"
	"golang.org/x/exp/slices"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/utils"
)

func (b *Bot) runStop(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
		return
	}

	mailbox := cfg.Get(config.RoomMailbox)
	if mailbox == "" {
		b.lp.SendNotice(evt.RoomID, "that room is not configured yet", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
		return
	}

	b.rooms.Delete(mailbox)

	err = b.cfg.SetRoom(evt.RoomID, config.Room{})
	if err != nil {
		b.Error(ctx, "cannot update settings: %v", err)
		return
	}

	b.lp.SendNotice(evt.RoomID, "mailbox has been disabled", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) handleOption(ctx context.Context, cmd []string) {
	if len(cmd) == 1 {
		b.getOption(ctx, cmd[0])
		return
	}
	switch cmd[0] {
	case config.RoomActive:
		return
	case config.RoomMailbox:
		b.setMailbox(ctx, cmd[1])
	case config.RoomPassword:
		b.setPassword(ctx)
	default:
		b.setOption(ctx, cmd[0], cmd[1])
	}
}

func (b *Bot) getOption(ctx context.Context, name string) {
	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
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
		b.lp.SendNotice(evt.RoomID, msg, linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
		return
	}

	if name == config.RoomMailbox {
		value = utils.EmailsList(value, cfg.Domain())
	}

	msg := fmt.Sprintf("`%s` of this room is:\n```\n%s\n```\n"+
		"To set it to a new value, send a `%s %s VALUE` command.",
		name, value, b.prefix, name)
	if name == config.RoomPassword {
		msg = fmt.Sprintf("There is an SMTP password already set for this room/mailbox. "+
			"It's stored in a secure hashed manner, so we can't tell you what the original raw password was. "+
			"To find the raw password, try to find your old message which had originally set it, "+
			"or just set a new one with `%s %s NEW_PASSWORD`.",
			b.prefix, name)
	}
	b.lp.SendNotice(evt.RoomID, msg, linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) setMailbox(ctx context.Context, value string) {
	evt := eventFromContext(ctx)
	existingID, ok := b.getMapping(value)
	if (ok && existingID != "" && existingID != evt.RoomID) || b.isReserved(value) {
		b.lp.SendNotice(evt.RoomID, fmt.Sprintf("Mailbox `%s` (%s) already taken, kupo", value, utils.EmailsList(value, "")))
		return
	}

	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
		return
	}
	old := cfg.Get(config.RoomMailbox)
	cfg.Set(config.RoomMailbox, value)
	cfg.Set(config.RoomOwner, evt.Sender.String())
	if old != "" {
		b.rooms.Delete(old)
	}
	active := b.ActivateMailbox(evt.Sender, evt.RoomID, value)
	cfg.Set(config.RoomActive, strconv.FormatBool(active))
	value = fmt.Sprintf("%s@%s", value, utils.SanitizeDomain(cfg.Domain()))

	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot update settings: %v", err)
		return
	}

	msg := fmt.Sprintf("mailbox of this room set to `%s`", value)
	b.lp.SendNotice(evt.RoomID, msg, linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) setPassword(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
		return
	}

	value := b.parseCommand(evt.Content.AsMessage().Body, false)[1] // get original value, without forced lower case
	value, err = argon2pw.GenerateSaltedHash(value)
	if err != nil {
		b.Error(ctx, "failed to hash password: %v", err)
		return
	}

	cfg.Set(config.RoomPassword, value)
	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot update settings: %v", err)
		return
	}

	b.lp.SendNotice(evt.RoomID, "SMTP password has been set", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) setOption(ctx context.Context, name, value string) {
	cmd := b.commands.get(name)
	if cmd != nil && cmd.sanitizer != nil {
		value = cmd.sanitizer(value)
	}

	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "failed to retrieve settings: %v", err)
		return
	}

	if name == config.RoomAutoreply ||
		name == config.RoomSignature {
		value = strings.Join(b.parseCommand(evt.Content.AsMessage().Body, false)[1:], " ")
	}

	if value == "reset" {
		value = ""
	}

	old := cfg.Get(name)
	if old == value {
		b.lp.SendNotice(evt.RoomID, "nothing changed, kupo.", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
		return
	}

	cfg.Set(name, value)
	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot update settings: %v", err)
		return
	}

	msg := fmt.Sprintf("`%s` of this room set to:\n```\n%s\n```", name, value)
	b.lp.SendNotice(evt.RoomID, msg, linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) runSpamlistAdd(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.getOption(ctx, config.RoomSpamlist)
		return
	}
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "cannot get room settings: %v", err)
		return
	}
	spamlist := utils.StringSlice(cfg[config.RoomSpamlist])
	for _, newItem := range commandSlice[1:] {
		newItem = strings.TrimSpace(newItem)
		if slices.Contains(spamlist, newItem) {
			continue
		}
		spamlist = append(spamlist, newItem)
	}

	cfg.Set(config.RoomSpamlist, utils.SliceString(spamlist))
	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot store room settings: %v", err)
		return
	}

	threadID := threadIDFromContext(ctx)
	if threadID == "" {
		threadID = evt.ID
	}

	b.lp.SendNotice(evt.RoomID, "spamlist has been updated, kupo", linkpearl.RelatesTo(threadID, cfg.NoThreads()))
}

func (b *Bot) runSpamlistRemove(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.getOption(ctx, config.RoomSpamlist)
		return
	}
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "cannot get room settings: %v", err)
		return
	}
	toRemove := map[int]struct{}{}
	spamlist := utils.StringSlice(cfg[config.RoomSpamlist])
	for _, item := range commandSlice[1:] {
		item = strings.TrimSpace(item)
		idx := slices.Index(spamlist, item)
		if idx < 0 {
			continue
		}
		toRemove[idx] = struct{}{}
	}
	if len(toRemove) == 0 {
		b.lp.SendNotice(evt.RoomID, "nothing new, kupo.", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
		return
	}

	updatedSpamlist := []string{}
	for i, item := range spamlist {
		if _, ok := toRemove[i]; ok {
			continue
		}
		updatedSpamlist = append(updatedSpamlist, item)
	}

	cfg.Set(config.RoomSpamlist, utils.SliceString(updatedSpamlist))
	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot store room settings: %v", err)
		return
	}

	b.lp.SendNotice(evt.RoomID, "spamlist has been updated, kupo", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}

func (b *Bot) runSpamlistReset(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, "cannot get room settings: %v", err)
		return
	}
	spamlist := utils.StringSlice(cfg[config.RoomSpamlist])
	if len(spamlist) == 0 {
		b.lp.SendNotice(evt.RoomID, "spamlist is empty, kupo.", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
		return
	}

	cfg.Set(config.RoomSpamlist, "")
	err = b.cfg.SetRoom(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, "cannot store room settings: %v", err)
		return
	}

	b.lp.SendNotice(evt.RoomID, "spamlist has been reset, kupo.", linkpearl.RelatesTo(evt.ID, cfg.NoThreads()))
}
