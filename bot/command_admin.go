package bot

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/etke.cc/go/secgen"
	"gitlab.com/etke.cc/linkpearl"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/utils"
)

func (b *Bot) sendMailboxes(ctx context.Context) {
	evt := eventFromContext(ctx)
	mailboxes := map[string]config.Room{}
	slice := []string{}
	b.rooms.Range(func(key, value any) bool {
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
		cfg, err := b.cfg.GetRoom(ctx, roomID)
		if err != nil {
			b.log.Error().Err(err).Msg("cannot retrieve settings")
		}

		mailboxes[mailbox] = cfg
		slice = append(slice, mailbox)
		return true
	})
	sort.Strings(slice)

	if len(slice) == 0 {
		b.lp.SendNotice(ctx, evt.RoomID, "No mailboxes are managed by the bot so far, kupo!", linkpearl.RelatesTo(evt.ID))
		return
	}

	var msg strings.Builder
	msg.WriteString("The following mailboxes are managed by the bot:\n")
	for _, mailbox := range slice {
		cfg := mailboxes[mailbox]
		msg.WriteString("* `")
		msg.WriteString(utils.EmailsList(mailbox, cfg.Domain()))
		msg.WriteString("` by ")
		msg.WriteString(cfg.Owner())
		msg.WriteString("\n")
	}

	b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runDelete(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Usage: `%s delete MAILBOX`", b.prefix), linkpearl.RelatesTo(evt.ID))
		return
	}
	mailbox := utils.Mailbox(commandSlice[1])

	v, ok := b.rooms.Load(mailbox)
	if v == nil || !ok {
		b.lp.SendNotice(ctx, evt.RoomID, "mailbox does not exists, kupo", linkpearl.RelatesTo(evt.ID))
		return
	}
	roomID, ok := v.(id.RoomID)
	if !ok {
		return
	}

	b.rooms.Delete(mailbox)
	err := b.cfg.SetRoom(ctx, roomID, config.Room{})
	if err != nil {
		b.Error(ctx, "cannot update settings: %v", err)
		return
	}

	b.lp.SendNotice(ctx, evt.RoomID, "mailbox has been deleted", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runUsers(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
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

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		return
	}

	_, homeserver, err := b.lp.GetClient().UserID.Parse()
	if err != nil {
		b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf("invalid userID: %v", err), linkpearl.RelatesTo(evt.ID))
	}

	patterns := commandSlice[1:]
	allowedUsers, err := parseMXIDpatterns(patterns, "@*:"+homeserver)
	if err != nil {
		b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf("invalid patterns: %v", err), linkpearl.RelatesTo(evt.ID))
		return
	}

	cfg.Set(config.BotUsers, strings.Join(patterns, " "))

	err = b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot set bot config: %v", err)
	}
	b.allowedUsers = allowedUsers
	b.lp.SendNotice(ctx, evt.RoomID, "allowed users updated", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runDKIM(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) > 1 && commandSlice[1] == "reset" {
		cfg.Set(config.BotDKIMPrivateKey, "")
		cfg.Set(config.BotDKIMSignature, "")
	}

	signature := cfg.DKIMSignature()
	if signature == "" {
		var private string
		var derr error
		signature, private, derr = secgen.DKIM()
		if derr != nil {
			b.Error(ctx, "cannot generate DKIM signature: %v", derr)
			return
		}
		cfg.Set(config.BotDKIMSignature, signature)
		cfg.Set(config.BotDKIMPrivateKey, private)
		err := b.cfg.SetBot(ctx, cfg)
		if err != nil {
			b.Error(ctx, "cannot save bot options: %v", err)
			return
		}
	}

	b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf(
		"DKIM signature is: `%s`.\n"+
			"You need to add it to DNS records of all domains added to postmoogle (if not already):\n"+
			"Add new DNS record with type = `TXT`, key (subdomain/from): `postmoogle._domainkey` and value (to):\n ```\n%s\n```\n"+
			"Without that record other email servers may reject your emails as spam, kupo.\n"+
			"To reset the signature, send `%s dkim reset`",
		signature, signature, b.prefix),
		linkpearl.RelatesTo(evt.ID),
	)
}

func (b *Bot) runCatchAll(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) < 2 {
		var msg strings.Builder
		msg.WriteString("Currently: `")
		if cfg.CatchAll() != "" {
			msg.WriteString(cfg.CatchAll())
			msg.WriteString(" (")
			msg.WriteString(utils.EmailsList(cfg.CatchAll(), ""))
			msg.WriteString(")")
		} else {
			msg.WriteString("not set")
		}
		msg.WriteString("`\n\n")
		msg.WriteString("Usage: `")
		msg.WriteString(b.prefix)
		msg.WriteString(" catch-all MAILBOX`")
		msg.WriteString("where mailbox is valid and existing mailbox name\n")

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		return
	}

	mailbox := utils.Mailbox(commandSlice[1])
	_, ok := b.GetMapping(ctx, mailbox)
	if !ok {
		b.lp.SendNotice(ctx, evt.RoomID, "mailbox does not exist, kupo.", linkpearl.RelatesTo(evt.ID))
		return
	}

	cfg.Set(config.BotCatchAll, mailbox)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot save bot options: %v", err)
		return
	}

	b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Catch-all is set to: `%s` (%s).", mailbox, utils.EmailsList(mailbox, "")), linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runAdminRoom(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) < 2 {
		var msg strings.Builder
		msg.WriteString("Currently: `")
		if cfg.AdminRoom() != "" {
			msg.WriteString(cfg.AdminRoom().String())
		} else {
			msg.WriteString("not set")
		}
		msg.WriteString("`\n\n")
		msg.WriteString("Usage: `")
		msg.WriteString(b.prefix)
		msg.WriteString(" adminroom ROOM_ID`")
		msg.WriteString("where ROOM_ID is valid and existing matrix room id\n")

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		return
	}

	roomID := b.parseCommand(evt.Content.AsMessage().Body, false)[1] // get original value, without forced lower case
	cfg.Set(config.BotAdminRoom, roomID)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot save bot options: %v", err)
		return
	}

	b.adminRooms = append([]id.RoomID{id.RoomID(roomID)}, b.adminRooms...) // make it the first room in list on the fly

	b.lp.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Admin Room is set to: `%s`.", roomID), linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) printGreylist(ctx context.Context, roomID id.RoomID) {
	cfg := b.cfg.GetBot(ctx)
	greylist := b.cfg.GetGreylist(ctx)
	var msg strings.Builder
	size := len(greylist)
	duration := cfg.Greylist()
	msg.WriteString("Currently: `")
	if duration == 0 {
		msg.WriteString("disabled")
	} else {
		msg.WriteString(cfg.Get(config.BotGreylist))
		msg.WriteString("min")
	}
	msg.WriteString("`")
	if size > 0 {
		msg.WriteString(", total known: ")
		msg.WriteString(strconv.Itoa(size))
		msg.WriteString(" hosts (`")
		msg.WriteString(strings.Join(greylist.Slice(), "`, `"))
		msg.WriteString("`)\n\n")
	}
	if duration == 0 {
		msg.WriteString("\n\nTo enable greylist: `")
		msg.WriteString(b.prefix)
		msg.WriteString(" greylist MIN`")
		msg.WriteString("where `MIN` is duration in minutes for automatic greylisting\n")
	}

	b.lp.SendNotice(ctx, roomID, msg.String(), linkpearl.RelatesTo(eventFromContext(ctx).ID))
}

func (b *Bot) runGreylist(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.printGreylist(ctx, evt.RoomID)
		return
	}
	cfg := b.cfg.GetBot(ctx)
	value := utils.SanitizeIntString(commandSlice[1])
	cfg.Set(config.BotGreylist, value)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot set bot config: %v", err)
	}
	b.lp.SendNotice(ctx, evt.RoomID, "greylist duration has been updated", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runBanlist(ctx context.Context, commandSlice []string) {
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) < 2 {
		banlist := b.cfg.GetBanlist(ctx)
		var msg strings.Builder
		size := len(banlist)
		if size > 0 {
			msg.WriteString("Currently: `")
			msg.WriteString(cfg.Get(config.BotBanlistEnabled))
			msg.WriteString("`, total: ")
			msg.WriteString(strconv.Itoa(size))
			msg.WriteString("\n\n")
		}
		if !cfg.BanlistEnabled() {
			msg.WriteString("To enable banlist, send `")
			msg.WriteString(b.prefix)
			msg.WriteString(" banlist true`\n\n")
		}
		msg.WriteString("To ban somebody: `")
		msg.WriteString(b.prefix)
		msg.WriteString(" banlist:add IP1 IP2 IP3...`")
		msg.WriteString("where each ip is IPv4 or IPv6\n\n")
		msg.WriteString("You can find current banlist values below:\n")

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		b.addBanlistTimeline(ctx, false)
		return
	}
	value := utils.SanitizeBoolString(commandSlice[1])
	cfg.Set(config.BotBanlistEnabled, value)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot set bot config: %v", err)
	}
	b.lp.SendNotice(ctx, evt.RoomID, "banlist has been updated", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runBanlistTotals(ctx context.Context) {
	evt := eventFromContext(ctx)
	banlist := b.cfg.GetBanlist(ctx)
	var msg strings.Builder
	size := len(banlist)
	if size == 0 {
		b.lp.SendNotice(ctx, evt.RoomID, "banlist is empty, kupo.", linkpearl.RelatesTo(evt.ID))
		return
	}

	msg.WriteString("Total: ")
	msg.WriteString(strconv.Itoa(size))
	msg.WriteString(" hosts banned\n\n")
	msg.WriteString("You can find daily totals below:\n")
	b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
	b.addBanlistTimeline(ctx, true)
}

func (b *Bot) runBanlistAuth(ctx context.Context, commandSlice []string) { //nolint:dupl // not in that case
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) < 2 {
		var msg strings.Builder
		msg.WriteString("Currently: `")
		msg.WriteString(cfg.Get(config.BotBanlistAuth))
		msg.WriteString("`\n\n")

		if !cfg.BanlistAuth() {
			msg.WriteString("To enable automatic banning for invalid credentials, send `")
			msg.WriteString(b.prefix)
			msg.WriteString(" banlist:auth true` (banlist itself must be enabled!)\n\n")
		}

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		return
	}
	value := utils.SanitizeBoolString(commandSlice[1])
	cfg.Set(config.BotBanlistAuth, value)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot set bot config: %v", err)
	}
	b.lp.SendNotice(ctx, evt.RoomID, "auth banning has been updated", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runBanlistAuto(ctx context.Context, commandSlice []string) { //nolint:dupl // not in that case
	evt := eventFromContext(ctx)
	cfg := b.cfg.GetBot(ctx)
	if len(commandSlice) < 2 {
		var msg strings.Builder
		msg.WriteString("Currently: `")
		msg.WriteString(cfg.Get(config.BotBanlistAuto))
		msg.WriteString("`\n\n")

		if !cfg.BanlistAuto() {
			msg.WriteString("To enable automatic banning for invalid emails, send `")
			msg.WriteString(b.prefix)
			msg.WriteString(" banlist:auto true` (banlist itself must be enabled!)\n\n")
		}

		b.lp.SendNotice(ctx, evt.RoomID, msg.String(), linkpearl.RelatesTo(evt.ID))
		return
	}
	value := utils.SanitizeBoolString(commandSlice[1])
	cfg.Set(config.BotBanlistAuto, value)
	err := b.cfg.SetBot(ctx, cfg)
	if err != nil {
		b.Error(ctx, "cannot set bot config: %v", err)
	}
	b.lp.SendNotice(ctx, evt.RoomID, "auto banning has been updated", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) runBanlistChange(ctx context.Context, mode string, commandSlice []string) {
	evt := eventFromContext(ctx)
	if len(commandSlice) < 2 {
		b.runBanlist(ctx, commandSlice)
		return
	}
	if !b.cfg.GetBot(ctx).BanlistEnabled() {
		b.lp.SendNotice(ctx, evt.RoomID, "banlist is disabled, you have to enable it first, kupo", linkpearl.RelatesTo(evt.ID))
		return
	}
	banlist := b.cfg.GetBanlist(ctx)

	var action func(net.Addr)
	if mode == "remove" {
		action = banlist.Remove
	} else {
		action = banlist.Add
	}

	ips := commandSlice[1:]
	for _, ip := range ips {
		addr, err := net.ResolveIPAddr("ip", ip)
		if err != nil {
			b.Error(ctx, "cannot remove %s from banlist: %v", ip, err)
			return
		}
		action(addr)
	}

	err := b.cfg.SetBanlist(ctx, banlist)
	if err != nil {
		b.Error(ctx, "cannot set banlist: %v", err)
		return
	}

	b.lp.SendNotice(ctx, evt.RoomID, "banlist has been updated, kupo", linkpearl.RelatesTo(evt.ID))
}

func (b *Bot) addBanlistTimeline(ctx context.Context, onlyTotals bool) {
	evt := eventFromContext(ctx)
	banlist := b.cfg.GetBanlist(ctx)
	timeline := map[string][]string{}
	for ip, ts := range banlist {
		key := "???"
		date, _ := time.ParseInLocation(time.RFC1123Z, ts, time.UTC) //nolint:errcheck // stored in that format
		if !date.IsZero() {
			key = date.Truncate(24 * time.Hour).Format(time.DateOnly)
		}
		if _, ok := timeline[key]; !ok {
			timeline[key] = []string{}
		}
		timeline[key] = append(timeline[key], ip)
	}
	keys := utils.MapKeys(timeline)

	for _, chunk := range utils.Chunks(keys, 7) {
		var txt strings.Builder
		for _, day := range chunk {
			data := timeline[day]
			sort.Strings(data)
			txt.WriteString("* `")
			txt.WriteString(day)
			if onlyTotals {
				txt.WriteString("` ")
				txt.WriteString(strconv.Itoa(len(data)))
				txt.WriteString(" hosts banned\n")
				continue
			}
			txt.WriteString("` `")
			txt.WriteString(strings.Join(data, "`, `"))
			txt.WriteString("`\n")
		}
		b.lp.SendNotice(ctx, evt.RoomID, txt.String(), linkpearl.RelatesTo(evt.ID))
	}
}

func (b *Bot) runBanlistReset(ctx context.Context) {
	evt := eventFromContext(ctx)
	if !b.cfg.GetBot(ctx).BanlistEnabled() {
		b.lp.SendNotice(ctx, evt.RoomID, "banlist is disabled, you have to enable it first, kupo", linkpearl.RelatesTo(evt.ID))
		return
	}

	err := b.cfg.SetBanlist(ctx, config.List{})
	if err != nil {
		b.Error(ctx, "cannot set banlist: %v", err)
		return
	}

	b.lp.SendNotice(ctx, evt.RoomID, "banlist has been reset, kupo", linkpearl.RelatesTo(evt.ID))
}
