package bot

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

type (
	command struct {
		key         string
		description string
		sanitizer   func(string) string
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

var commands = commandList{
	// special commands
	{
		key:         "help",
		description: "Show this help message",
	},
	{
		key:         "stop",
		description: "Disable bridge for the room and clear all configuration",
	},
	{}, // delimiter
	// options commands
	{
		key:         optionMailbox,
		description: "Get or set mailbox of the room",
		sanitizer:   utils.Mailbox,
	},
	{
		key:         optionOwner,
		description: "Get or set owner of the room",
		sanitizer:   func(s string) string { return s },
	},
	{}, // delimiter
	{
		key: optionNoSender,
		description: fmt.Sprintf(
			"Get or set `%s` of the room (`true` - hide email sender; `false` - show email sender)",
			optionNoSender,
		),
		sanitizer: utils.SanitizeBoolString,
	},
	{
		key: optionNoSubject,
		description: fmt.Sprintf(
			"Get or set `%s` of the room (`true` - hide email subject; `false` - show email subject)",
			optionNoSubject,
		),
		sanitizer: utils.SanitizeBoolString,
	},
	{
		key: optionNoHTML,
		description: fmt.Sprintf(
			"Get or set `%s` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)",
			optionNoHTML,
		),
		sanitizer: utils.SanitizeBoolString,
	},
	{
		key: optionNoThreads,
		description: fmt.Sprintf(
			"Get or set `%s` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)",
			optionNoThreads,
		),
		sanitizer: utils.SanitizeBoolString,
	},
	{
		key: optionNoFiles,
		description: fmt.Sprintf(
			"Get or set `%s` of the room (`true` - ignore email attachments; `false` - upload email attachments)",
			optionNoFiles,
		),
		sanitizer: utils.SanitizeBoolString,
	},
}

func (b *Bot) handleCommand(ctx context.Context, evt *event.Event, commandSlice []string) {
	if cmd := commands.get(commandSlice[0]); cmd == nil {
		return
	}

	// ignore requests over federation if disabled
	if !b.federation && evt.Sender.Homeserver() != b.lp.GetClient().UserID.Homeserver() {
		return
	}

	switch commandSlice[0] {
	case "help":
		b.sendHelp(ctx, evt.RoomID)
	case "stop":
		b.runStop(ctx, true)
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

	b.Notice(ctx, roomID, msg.String())
}

func (b *Bot) sendHelp(ctx context.Context, roomID id.RoomID) {
	evt := eventFromContext(ctx)

	cfg, serr := b.getSettings(evt.RoomID)
	if serr != nil {
		b.log.Error("cannot retrieve settings: %v", serr)
	}

	var msg strings.Builder
	msg.WriteString("The following commands are supported:\n\n")
	for _, cmd := range commands {
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

	b.Notice(ctx, roomID, msg.String())
}

func (b *Bot) runStop(ctx context.Context, checkAllowed bool) {
	evt := eventFromContext(ctx)
	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if checkAllowed && !b.Allowed(evt.Sender, cfg) {
		b.Notice(ctx, evt.RoomID, "you don't have permission to do that")
		return
	}

	mailbox := cfg.Get(optionMailbox)
	if mailbox == "" {
		b.Notice(ctx, evt.RoomID, "that room is not configured yet")
		return
	}

	b.rooms.Delete(mailbox)

	err = b.setSettings(evt.RoomID, settings{})
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.Notice(ctx, evt.RoomID, "mailbox has been disabled")
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
		b.Notice(ctx, evt.RoomID, fmt.Sprintf("`%s` is not set, kupo.", name))
		return
	}

	if name == optionMailbox {
		value = value + "@" + b.domain
	}

	b.Notice(ctx, evt.RoomID, fmt.Sprintf("`%s` of this room is `%s`", name, value))
}

func (b *Bot) setOption(ctx context.Context, name, value string) {
	cmd := commands.get(name)
	if cmd != nil {
		value = cmd.sanitizer(value)
	}

	evt := eventFromContext(ctx)
	if name == optionMailbox {
		existingID, ok := b.GetMapping(value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.Notice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s@%s` already taken, kupo", value, b.domain))
			return
		}
	}

	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !b.Allowed(evt.Sender, cfg) {
		b.Notice(ctx, evt.RoomID, "you don't have permission to do that, kupo")
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

	b.Notice(ctx, evt.RoomID, fmt.Sprintf("`%s` of this room set to `%s`", name, value))
}
