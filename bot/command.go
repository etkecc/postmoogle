package bot

import (
	"context"
	"fmt"
	"strings"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
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
		b.runStop(ctx, evt)
	default:
		b.handleOption(ctx, evt, command)
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

func (b *Bot) sendHelp(ctx context.Context, roomID id.RoomID) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("sendHelp"))
	defer span.Finish()

	var msg strings.Builder
	msg.WriteString("the following commands are supported:\n\n")
	for _, command := range commands {
		msg.WriteString("* **")
		msg.WriteString(command.key)
		msg.WriteString("** - ")
		msg.WriteString(command.description)
		msg.WriteString("\n")
	}

	content := format.RenderMarkdown(msg.String(), true, true)
	content.MsgType = event.MsgNotice
	_, err := b.lp.Send(roomID, content)
	if err != nil {
		b.Error(span.Context(), roomID, "cannot send message: %v", err)
	}
}

func (b *Bot) runStop(ctx context.Context, evt *event.Event) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("runStop"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	mailbox := cfg.Get(optionMailbox)
	if mailbox == "" {
		b.Notice(span.Context(), evt.RoomID, "that room is not configured yet")
		return
	}

	b.roomsmu.Lock()
	delete(b.rooms, mailbox)
	b.roomsmu.Unlock()

	err = b.setSettings(span.Context(), evt.RoomID, settings{})
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.Notice(span.Context(), evt.RoomID, "mailbox has been disabled")
}

func (b *Bot) handleOption(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getOption(ctx, evt, command[0])
		return
	}
	b.setOption(ctx, evt, command[0], command[1])
}

func (b *Bot) getOption(ctx context.Context, evt *event.Event, name string) {
	msg := "`%s` of this room is %s"
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getOption"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	value := cfg.Get(name)
	if value == "" {
		b.Notice(span.Context(), evt.RoomID, "`%s` is not set", name)
		return
	}

	if name == optionMailbox {
		msg = msg + "@" + b.domain
	}

	b.Notice(span.Context(), evt.RoomID, msg, name, value)
}

func (b *Bot) setOption(ctx context.Context, evt *event.Event, name, value string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setOption"))
	defer span.Finish()
	msg := "`%s` of this room set to %s"

	sanitizer, ok := sanitizers[name]
	if ok {
		value = sanitizer(value)
	}

	if name == optionMailbox {
		existingID, ok := b.GetMapping(ctx, value)
		if ok && existingID != "" && existingID != evt.RoomID {
			b.Notice(span.Context(), evt.RoomID, "Mailbox %s@%s already taken", value, b.domain)
			return
		}
	}

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve settings: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.Set(name, value)
	if name == optionMailbox {
		msg = msg + "@" + b.domain
		cfg.Set(optionOwner, evt.Sender.String())
		b.roomsmu.Lock()
		b.rooms[value] = evt.RoomID
		b.roomsmu.Unlock()
	}

	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	b.Notice(span.Context(), evt.RoomID, msg, name, value)
}
