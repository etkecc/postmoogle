package bot

import (
	"context"
	"fmt"
	"strings"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

type sanitizerFunc func(string) string

type commandDefinition struct {
	key         string
	description string
}

type commandList []commandDefinition

func (c commandList) get(key string) (*commandDefinition, bool) {
	for _, command := range c {
		if command.key == key {
			return &command, true
		}
	}
	return nil, false
}

var (
	commands = commandList{
		// special commands
		{
			key:         "help",
			description: "Show this help message",
		},
		{
			key:         "stop",
			description: "Disable bridge for the room and clear all configuration",
		},

		// options commands
		{
			key:         optionMailbox,
			description: "Get or set mailbox of the room",
		},
		{
			key:         optionOwner,
			description: "Get or set owner of the room",
		},
		{
			key: optionNoSender,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email sender; `false` - show email sender)",
				optionNoSender,
			),
		},
		{
			key: optionNoSubject,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email subject; `false` - show email subject)",
				optionNoSubject,
			),
		},
	}

	// sanitizers is map of option name => sanitizer function
	sanitizers = map[string]sanitizerFunc{
		optionMailbox:   utils.Mailbox,
		optionNoSender:  utils.SanitizeBoolString,
		optionNoSubject: utils.SanitizeBoolString,
	}
)

func (b *Bot) handleCommand(ctx context.Context, evt *event.Event, command []string) {
	if _, ok := commands.get(command[0]); !ok {
		return
	}

	// ignore requests over federation if disabled
	if !b.federation && evt.Sender.Homeserver() != b.lp.GetClient().UserID.Homeserver() {
		return
	}

	switch command[0] {
	case "help":
		b.sendHelp(ctx, evt.RoomID)
	case "stop":
		b.runStop(ctx)
	default:
		b.handleOption(ctx, command)
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
	msg.WriteString("Hello!\n\n")
	msg.WriteString("This is Postmoogle - a bot that bridges Email to Matrix.\n\n")
	msg.WriteString(fmt.Sprintf(
		"To get started, assign an email address to this room by sending a `%s %s SOME_INBOX` command.\n",
		b.prefix,
		optionMailbox,
	))
	msg.WriteString(fmt.Sprintf(
		"You will then be able to send emails to `SOME_INBOX@%s` and have them appear in this room.",
		b.domain,
	))

	b.Notice(ctx, roomID, msg.String())
}

func (b *Bot) sendHelp(ctx context.Context, roomID id.RoomID) {
	var msg strings.Builder
	msg.WriteString("The following commands are supported:\n\n")
	for _, command := range commands {
		msg.WriteString(fmt.Sprintf("* **`%s %s`** - %s\n", b.prefix, command.key, command.description))
	}

	b.Notice(ctx, roomID, msg.String())
}

func (b *Bot) runStop(ctx context.Context) {
	evt := eventFromContext(ctx)
	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
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

func (b *Bot) handleOption(ctx context.Context, command []string) {
	if len(command) == 1 {
		b.getOption(ctx, command[0])
		return
	}
	b.setOption(ctx, command[0], command[1])
}

func (b *Bot) getOption(ctx context.Context, name string) {
	msg := "`%s` of this room is %s"

	evt := eventFromContext(ctx)
	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	value := cfg.Get(name)
	if value == "" {
		b.Notice(ctx, evt.RoomID, fmt.Sprintf("`%s` is not set", name))
		return
	}

	if name == optionMailbox {
		msg = msg + "@" + b.domain
	}

	b.Notice(ctx, evt.RoomID, fmt.Sprintf(msg, name, value))
}

func (b *Bot) setOption(ctx context.Context, name, value string) {
	msg := "`%s` of this room set to `%s`"

	sanitizer, ok := sanitizers[name]
	if ok {
		value = sanitizer(value)
	}

	evt := eventFromContext(ctx)
	if name == optionMailbox {
		existingID, ok := b.GetMapping(value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.Notice(ctx, evt.RoomID, fmt.Sprintf("Mailbox `%s@%s` already taken", value, b.domain))
			return
		}
	}

	cfg, err := b.getSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(ctx, evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.Set(name, value)
	if name == optionMailbox {
		msg = msg + "@" + b.domain
		cfg.Set(optionOwner, evt.Sender.String())
		b.rooms.Store(value, evt.RoomID)
	}

	err = b.setSettings(evt.RoomID, cfg)
	if err != nil {
		b.Error(ctx, evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.Notice(ctx, evt.RoomID, fmt.Sprintf(msg, name, value))
}
