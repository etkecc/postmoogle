package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	commandHelp      = "help"
	commandStop      = "stop"
	commandSend      = "send"
	commandDKIM      = "dkim"
	commandCatchAll  = botOptionCatchAll
	commandUsers     = botOptionUsers
	commandDelete    = "delete"
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

func (b *Bot) initCommands() commandList {
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
		{
			key:         commandSend,
			description: "Send email",
			allowed:     b.allowSend,
		},
		{allowed: b.allowOwner}, // delimiter
		// options commands
		{
			key:         roomOptionMailbox,
			description: "Get or set mailbox of the room",
			sanitizer:   utils.Mailbox,
			allowed:     b.allowOwner,
		},
		{
			key:         roomOptionOwner,
			description: "Get or set owner of the room",
			sanitizer:   func(s string) string { return s },
			allowed:     b.allowOwner,
		},
		{
			key:         roomOptionPassword,
			description: "Get or set SMTP password of the room's mailbox",
			allowed:     b.allowOwner,
		},
		{allowed: b.allowOwner}, // delimiter
		{
			key: roomOptionNoSend,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - disable email sending; `false` - enable email sending)",
				roomOptionNoSend,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoSender,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email sender; `false` - show email sender)",
				roomOptionNoSender,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoRecipient,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide recipient; `false` - show recipient)",
				roomOptionNoRecipient,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoSubject,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email subject; `false` - show email subject)",
				roomOptionNoSubject,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoHTML,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)",
				roomOptionNoHTML,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoThreads,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)",
				roomOptionNoThreads,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: roomOptionNoFiles,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email attachments; `false` - upload email attachments)",
				roomOptionNoFiles,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{allowed: b.allowOwner}, // delimiter
		{
			key:         roomOptionSpamcheckMX,
			description: "only accept email from servers which seem prepared to receive it (those having valid MX records) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{
			key:         roomOptionSpamcheckSMTP,
			description: "only accept email from servers which seem prepared to receive it (those listening on an SMTP port) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{
			key: roomOptionSpamlist,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (comma-separated list), eg: `spammer@example.com,*@spammer.org,spam@*`",
				roomOptionSpamlist,
			),
			sanitizer: utils.SanitizeStringSlice,
			allowed:   b.allowOwner,
		},
		{allowed: b.allowAdmin}, // delimiter
		{
			key:         botOptionUsers,
			description: "Get or set allowed users",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandDKIM,
			description: "Get DKIM signature",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandCatchAll,
			description: "Get or set catch-all mailbox",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandMailboxes,
			description: "Show the list of all mailboxes",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandDelete,
			description: "Delete specific mailbox",
			allowed:     b.allowAdmin,
		},
	}
}

func (b *Bot) handleCommand(ctx context.Context, evt *event.Event, commandSlice []string) {
	cmd := b.commands.get(commandSlice[0])
	if cmd == nil {
		return
	}
	_, err := b.lp.GetClient().UserTyping(evt.RoomID, true, 30*time.Second)
	if err != nil {
		b.log.Error("cannot send typing notification: %v", err)
	}
	defer b.lp.GetClient().UserTyping(evt.RoomID, false, 30*time.Second) //nolint:errcheck

	if !cmd.allowed(evt.Sender, evt.RoomID) {
		b.SendNotice(ctx, evt.RoomID, "not allowed to do that, kupo")
		return
	}

	switch commandSlice[0] {
	case commandHelp:
		b.sendHelp(ctx)
	case commandStop:
		b.runStop(ctx)
	case commandSend:
		b.runSend(ctx)
	case commandDKIM:
		b.runDKIM(ctx, commandSlice)
	case commandUsers:
		b.runUsers(ctx, commandSlice)
	case commandCatchAll:
		b.runCatchAll(ctx, commandSlice)
	case commandDelete:
		b.runDelete(ctx, commandSlice)
	case commandMailboxes:
		b.sendMailboxes(ctx)
	default:
		b.handleOption(ctx, commandSlice)
	}
}

func (b *Bot) parseCommand(message string, toLower bool) []string {
	if message == "" {
		return nil
	}

	index := strings.LastIndex(message, b.prefix)
	if index == -1 {
		return nil
	}

	message = strings.Replace(message, b.prefix, "", 1)
	if toLower {
		message = strings.ToLower(message)
	}
	return strings.Split(strings.TrimSpace(message), " ")
}

func (b *Bot) sendIntroduction(ctx context.Context, roomID id.RoomID) {
	var msg strings.Builder
	msg.WriteString("Hello, kupo!\n\n")

	msg.WriteString("This is Postmoogle - a bot that bridges Email to Matrix.\n\n")

	msg.WriteString("To get started, assign an email address to this room by sending a `")
	msg.WriteString(b.prefix)
	msg.WriteString(" ")
	msg.WriteString(roomOptionMailbox)
	msg.WriteString(" SOME_INBOX` command.\n")

	msg.WriteString("You will then be able to send emails to `SOME_INBOX@")
	msg.WriteString(b.domain)
	msg.WriteString("` and have them appear in this room.")

	b.SendNotice(ctx, roomID, msg.String())
}

func (b *Bot) sendHelp(ctx context.Context) {
	evt := eventFromContext(ctx)

	cfg, serr := b.getRoomSettings(evt.RoomID)
	if serr != nil {
		b.log.Error("cannot retrieve settings: %v", serr)
	}

	var msg strings.Builder
	msg.WriteString("The following commands are supported and accessible to you:\n\n")
	for _, cmd := range b.commands {
		if !cmd.allowed(evt.Sender, evt.RoomID) {
			continue
		}
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
				if cmd.key == roomOptionMailbox {
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

	b.SendNotice(ctx, evt.RoomID, msg.String())
}

func (b *Bot) runSend(ctx context.Context) {
	evt := eventFromContext(ctx)
	if !b.allowSend(evt.Sender, evt.RoomID) {
		return
	}
	commandSlice := b.parseCommand(evt.Content.AsMessage().Body, false)
	to, subject, body, err := utils.ParseSend(commandSlice)
	if err == utils.ErrInvalidArgs {
		b.SendNotice(ctx, evt.RoomID, fmt.Sprintf(
			"Usage:\n"+
				"```\n"+
				"%s send someone@example.com\n"+
				"Subject goes here on a line of its own\n"+
				"Email content goes here\n"+
				"on as many lines\n"+
				"as you want.\n"+
				"```",
			b.prefix))
		return
	}

	cfg, err := b.getRoomSettings(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve room settings: %v", err)
		return
	}

	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.SendNotice(ctx, evt.RoomID, "mailbox is not configured, kupo")
		return
	}

	tos := strings.Split(to, ",")
	// validate first
	for _, to := range tos {
		if !utils.AddressValid(to) {
			b.Error(ctx, evt.RoomID, "email address is not valid")
			return
		}
	}

	from := mailbox + "@" + b.domain
	ID := fmt.Sprintf("<%s@%s>", evt.ID, b.domain)
	for _, to := range tos {
		data := utils.
			NewEmail(ID, "", subject, from, to, body, "", nil).
			Compose(b.getBotSettings().DKIMPrivateKey())
		err = b.mta.Send(from, to, data)
		if err != nil {
			b.Error(ctx, evt.RoomID, "cannot send email to %s: %v", to, err)
		} else {
			b.SendNotice(ctx, evt.RoomID, "Email has been sent to "+to)
		}
	}
	if len(tos) > 1 {
		b.SendNotice(ctx, evt.RoomID, "All emails were sent.")
	}
}
