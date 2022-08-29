package bot

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	commandHelp      = "help"
	commandStop      = "stop"
	commandMailboxes = "mailboxes"
)

type (
	command struct {
		key         string
		description string
		sanitizer   func(string) string
		allowed     func(id.UserID, id.RoomID) bool
	}
	commandList []command
)

func (c commandList) get(key string) *command {
	for _, cmd := range c {
		if cmd.key == key {
			return &cmd
		}
	}
	return nil
}

func (b *Bot) buildCommandList() commandList {
	return commandList{
		// special commands
		{
			key:         commandHelp,
			description: "Show this help message",
			allowed:     b.allowAnyone,
		},
		{
			key:         commandStop,
			description: "Disable bridge for the room and clear all configuration",
			allowed:     b.allowOwner,
		},
		{}, // delimiter
		// options commands
		{
			key:         optionMailbox,
			description: "Get or set mailbox of the room",
			sanitizer:   utils.Mailbox,
			allowed:     b.allowOwner,
		},
		{
			key:         optionOwner,
			description: "Get or set owner of the room",
			sanitizer:   func(s string) string { return s },
			allowed:     b.allowOwner,
		},
		{}, // delimiter
		{
			key: optionNoSender,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email sender; `false` - show email sender)",
				optionNoSender,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: optionNoSubject,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email subject; `false` - show email subject)",
				optionNoSubject,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: optionNoHTML,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)",
				optionNoHTML,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: optionNoThreads,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)",
				optionNoThreads,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: optionNoFiles,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email attachments; `false` - upload email attachments)",
				optionNoFiles,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{}, // delimiter
		{
			key:         commandMailboxes,
			description: "Show the list of all mailboxes",
			allowed:     b.allowAdmin,
		},
	}
}

func (b *Bot) handleCommand(ctx context.Context, evt *event.Event, commandSlice []string) {
	cmd := b.commands.get(commandSlice[0])
	if cmd == nil {
		return
	}

	// ignore requests over federation if disabled
	if !b.federation && evt.Sender.Homeserver() != b.lp.GetClient().UserID.Homeserver() {
		return
	}

	if !cmd.allowed(evt.Sender, evt.RoomID) {
		b.SendNotice(ctx, evt.RoomID, "not allowed to do that, kupo")
		return
	}

	switch commandSlice[0] {
	case commandHelp:
		b.sendHelp(ctx, evt.RoomID)
	case commandStop:
		b.runStop(ctx)
	case commandMailboxes:
		b.sendMailboxes(ctx)
	default:
		b.handleOption(ctx, commandSlice)
	}
}

func (b *Bot) parseCommand(message string) []string {
	if message == "" {
		return nil
	}

	index := strings.LastIndex(message, b.prefix)
	if index == -1 {
		return nil
	}

	message = strings.ToLower(strings.TrimSpace(strings.Replace(message, b.prefix, "", 1)))
	return strings.Split(message, " ")
}

func (b *Bot) sendIntroduction(ctx context.Context, roomID id.RoomID) {
	var msg strings.Builder
	msg.WriteString("Hello, kupo!\n\n")

	msg.WriteString("This is Postmoogle - a bot that bridges Email to Matrix.\n\n")

	msg.WriteString("To get started, assign an email address to this room by sending a `")
	msg.WriteString(b.prefix)
	msg.WriteString(" ")
	msg.WriteString(optionMailbox)
	msg.WriteString("` command.\n")

	msg.WriteString("You will then be able to send emails to `SOME_INBOX@")
	msg.WriteString(b.domain)
	msg.WriteString("` and have them appear in this room.")

	b.SendNotice(ctx, roomID, msg.String())
}

func (b *Bot) sendHelp(ctx context.Context, roomID id.RoomID) {
	evt := eventFromContext(ctx)

	cfg, serr := b.getSettings(evt.RoomID)
	if serr != nil {
		b.log.Error("cannot retrieve settings: %v", serr)
	}

	var msg strings.Builder
	msg.WriteString("The following commands are supported:\n\n")
	for _, cmd := range b.commands {
		if cmd.key == "" {
			msg.WriteString("\n---\n")
			continue
		}
		msg.WriteString("* **`")
		msg.WriteString(b.prefix)
		msg.WriteString(" ")
		msg.WriteString(cmd.key)
		msg.WriteString("`**")
		value := cfg.Get(cmd.key)
		if cmd.sanitizer != nil {
			switch value != "" {
			case false:
				msg.WriteString("(currently not set)")
			case true:
				msg.WriteString("(currently `")
				msg.WriteString(value)
				if cmd.key == optionMailbox {
					msg.WriteString("@")
					msg.WriteString(b.domain)
				}
				msg.WriteString("`)")
			}
		}

		msg.WriteString(" - ")

		msg.WriteString(cmd.description)
		msg.WriteString("\n")
	}

	b.SendNotice(ctx, roomID, msg.String())
}

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

func (b *Bot) runStop(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	mailbox := cfg.Get(optionMailbox)
	if mailbox == "" {
		b.SendNotice(ctx, evt.RoomID, "that room is not configured yet")
		return
	}

	b.rooms.Delete(mailbox)

	err = b.setSettings(evt.RoomID, settings{})
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
	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	value := cfg.Get(name)
	if value == "" {
		b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("`%s` is not set, kupo.", name))
		return
	}

	if name == optionMailbox {
		value = value + "@" + b.domain
	}

	b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("`%s` of this room is `%s`", name, value))
}

func (b *Bot) setOption(ctx context.Context, name, value string) {
	cmd := b.commands.get(name)
	if cmd != nil {
		value = cmd.sanitizer(value)
	}

	evt := eventFromContext(ctx)
	if name == optionMailbox {
		existingID, ok := b.GetMapping(value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s@%s` already taken, kupo", value, b.domain))
			return
		}
	}

	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	old := cfg.Get(name)
	cfg.Set(name, value)

	if name == optionMailbox {
		cfg.Set(optionOwner, evt.Sender.String())
		if old != "" {
			b.rooms.Delete(old)
		}
		b.rooms.Store(value, evt.RoomID)
		value = fmt.Sprintf("%s@%s", value, b.domain)
	}

	err = b.setSettings(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	if name == optionMailbox {
		value = value + "@" + b.domain
	}

	b.SendNotice(ctx, evt.RoomID, fmt.Sprintf("`%s` of this room set to `%s`", name, value))
}
