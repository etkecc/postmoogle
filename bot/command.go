package bot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"gitlab.com/etke.cc/postmoogle/bot/config"
	"gitlab.com/etke.cc/postmoogle/email"
	"gitlab.com/etke.cc/postmoogle/utils"
)

const (
	commandHelp           = "help"
	commandStop           = "stop"
	commandSend           = "send"
	commandDKIM           = "dkim"
	commandCatchAll       = config.BotCatchAll
	commandUsers          = config.BotUsers
	commandQueueBatch     = config.BotQueueBatch
	commandQueueRetries   = config.BotQueueRetries
	commandSpamlist       = "spam:list"
	commandSpamlistAdd    = "spam:add"
	commandSpamlistRemove = "spam:remove"
	commandSpamlistReset  = "spam:reset"
	commandDelete         = "delete"
	commandBanlist        = "banlist"
	commandBanlistAdd     = "banlist:add"
	commandBanlistRemove  = "banlist:remove"
	commandBanlistReset   = "banlist:reset"
	commandMailboxes      = "mailboxes"
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
		{allowed: b.allowOwner, description: "mailbox ownership"}, // delimiter
		// options commands
		{
			key:         config.RoomMailbox,
			description: "Get or set mailbox of the room",
			sanitizer:   utils.Mailbox,
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomDomain,
			description: "Get or set default domain of the room",
			sanitizer:   utils.SanitizeDomain,
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomOwner,
			description: "Get or set owner of the room",
			sanitizer:   func(s string) string { return s },
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomPassword,
			description: "Get or set SMTP password of the room's mailbox",
			allowed:     b.allowOwner,
		},
		{allowed: b.allowOwner, description: "mailbox options"}, // delimiter
		{
			key: config.RoomNoSend,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - disable email sending; `false` - enable email sending)",
				config.RoomNoSend,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoReplies,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore matrix replies; `false` - parse matrix replies)",
				config.RoomNoReplies,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoSender,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email sender; `false` - show email sender)",
				config.RoomNoSender,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoRecipient,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide recipient; `false` - show recipient)",
				config.RoomNoRecipient,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoCC,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide CC; `false` - show CC)",
				config.RoomNoCC,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoSubject,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - hide email subject; `false` - show email subject)",
				config.RoomNoSubject,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoHTML,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore HTML in email; `false` - parse HTML in emails)",
				config.RoomNoHTML,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoThreads,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email threads; `false` - convert email threads into matrix threads)",
				config.RoomNoThreads,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoFiles,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore email attachments; `false` - upload email attachments)",
				config.RoomNoFiles,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{
			key: config.RoomNoInlines,
			description: fmt.Sprintf(
				"Get or set `%s` of the room (`true` - ignore inline attachments; `false` - upload inline attachments)",
				config.RoomNoInlines,
			),
			sanitizer: utils.SanitizeBoolString,
			allowed:   b.allowOwner,
		},
		{allowed: b.allowOwner, description: "mailbox security checks"}, // delimiter
		{
			key:         config.RoomSpamcheckMX,
			description: "only accept email from servers which seem prepared to receive it (those having valid MX records) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomSpamcheckSPF,
			description: "only accept email from senders which authorized to send it (those matching SPF records) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomSpamcheckDKIM,
			description: "only accept correctly authorized emails (without DKIM signature at all or with valid DKIM signature) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{
			key:         config.RoomSpamcheckSMTP,
			description: "only accept email from servers which seem prepared to receive it (those listening on an SMTP port) (`true` - enable, `false` - disable)",
			sanitizer:   utils.SanitizeBoolString,
			allowed:     b.allowOwner,
		},
		{allowed: b.allowOwner, description: "mailbox anti-spam"}, // delimiter
		{
			key:         commandSpamlist,
			description: "Show comma-separated spamlist of the room, eg: `spammer@example.com,*@spammer.org,spam@*`",
			sanitizer:   utils.SanitizeStringSlice,
			allowed:     b.allowOwner,
		},
		{
			key:         commandSpamlistAdd,
			description: "Mark an email address (or pattern) as spam",
			allowed:     b.allowOwner,
		},
		{
			key:         commandSpamlistRemove,
			description: "Unmark an email address (or pattern) as spam",
			allowed:     b.allowOwner,
		},
		{
			key:         commandSpamlistReset,
			description: "Reset spamlist",
			allowed:     b.allowOwner,
		},
		{allowed: b.allowAdmin, description: "server options"}, // delimiter
		{
			key:         config.BotAdminRoom,
			description: "Get or set admin room",
			allowed:     b.allowAdmin,
		},
		{
			key:         config.BotUsers,
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
			key:         commandQueueBatch,
			description: "max amount of emails to process on each queue check",
			sanitizer:   utils.SanitizeIntString,
			allowed:     b.allowAdmin,
		},
		{
			key:         commandQueueRetries,
			description: "max amount of tries per email in queue before removal",
			sanitizer:   utils.SanitizeIntString,
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
		{allowed: b.allowAdmin, description: "server antispam"}, // delimiter
		{
			key:         config.BotGreylist,
			description: "Set automatic greylisting duration in minutes (0 - disabled)",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandBanlist,
			description: "Enable/disable banlist and show current values",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandBanlistAdd,
			description: "Ban an IP",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandBanlistRemove,
			description: "Unban an IP",
			allowed:     b.allowAdmin,
		},
		{
			key:         commandBanlistReset,
			description: "Reset banlist",
			allowed:     b.allowAdmin,
		},
	}
}

func (b *Bot) handle(ctx context.Context) {
	evt := eventFromContext(ctx)
	err := b.lp.GetClient().MarkRead(evt.RoomID, evt.ID)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot send read receipt")
	}

	content := evt.Content.AsMessage()
	if content == nil {
		b.Error(ctx, evt.RoomID, "cannot read message")
		return
	}
	// ignore notices
	if content.MsgType == event.MsgNotice {
		return
	}
	message := strings.TrimSpace(content.Body)
	commandSlice := b.parseCommand(message, true)
	if commandSlice == nil {
		if utils.EventParent("", content) != "" {
			b.SendEmailReply(ctx)
		}
		return
	}

	cmd := b.commands.get(commandSlice[0])
	if cmd == nil {
		return
	}
	_, err = b.lp.GetClient().UserTyping(evt.RoomID, true, 30*time.Second)
	if err != nil {
		b.log.Error().Err(err).Msg("cannot send typing notification")
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
	case commandSpamlistAdd:
		b.runSpamlistAdd(ctx, commandSlice)
	case commandSpamlistRemove:
		b.runSpamlistRemove(ctx, commandSlice)
	case commandSpamlistReset:
		b.runSpamlistReset(ctx)
	case config.BotAdminRoom:
		b.runAdminRoom(ctx, commandSlice)
	case commandUsers:
		b.runUsers(ctx, commandSlice)
	case commandCatchAll:
		b.runCatchAll(ctx, commandSlice)
	case commandDelete:
		b.runDelete(ctx, commandSlice)
	case config.BotGreylist:
		b.runGreylist(ctx, commandSlice)
	case commandBanlist:
		b.runBanlist(ctx, commandSlice)
	case commandBanlistAdd:
		b.runBanlistAdd(ctx, commandSlice)
	case commandBanlistRemove:
		b.runBanlistRemove(ctx, commandSlice)
	case commandBanlistReset:
		b.runBanlistReset(ctx)
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
	msg.WriteString(config.RoomMailbox)
	msg.WriteString(" SOME_INBOX` command.\n")

	msg.WriteString("You will then be able to send emails to ")
	msg.WriteString(utils.EmailsList("SOME_INBOX", ""))
	msg.WriteString("` and have them appear in this room.")

	b.SendNotice(ctx, roomID, msg.String())
}

func (b *Bot) getHelpValue(cfg config.Room, cmd command) string {
	name := cmd.key
	if name == commandSpamlist {
		name = config.RoomSpamlist
	}

	value := cfg.Get(name)
	if cmd.sanitizer != nil {
		switch value != "" {
		case false:
			return "(currently not set)"
		case true:
			txt := "(currently " + value
			if cmd.key == config.RoomMailbox {
				txt += " (" + utils.EmailsList(value, cfg.Domain()) + ")"
			}
			return txt + ")"
		}
	}

	return ""
}

func (b *Bot) sendHelp(ctx context.Context) {
	evt := eventFromContext(ctx)

	cfg, serr := b.cfg.GetRoom(evt.RoomID)
	if serr != nil {
		b.log.Error().Err(serr).Msg("cannot retrieve settings")
	}

	var msg strings.Builder
	msg.WriteString("The following commands are supported and accessible to you:\n\n")
	for _, cmd := range b.commands {
		if !cmd.allowed(evt.Sender, evt.RoomID) {
			continue
		}
		if cmd.key == "" {
			msg.WriteString("\n---\n\n")
			msg.WriteString("#### ")
			msg.WriteString(cmd.description)
			msg.WriteString("\n")
			continue
		}
		msg.WriteString("* **`")
		msg.WriteString(b.prefix)
		msg.WriteString(" ")
		msg.WriteString(cmd.key)
		msg.WriteString("`**")

		msg.WriteString(b.getHelpValue(cfg, cmd))
		msg.WriteString(" - ")

		msg.WriteString(cmd.description)
		msg.WriteString("\n")
	}

	b.SendNotice(ctx, evt.RoomID, msg.String())
}

//nolint:gocognit // TODO
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

	cfg, err := b.cfg.GetRoom(evt.RoomID)
	if err != nil {
		b.Error(ctx, evt.RoomID, "failed to retrieve room settings: %v", err)
		return
	}

	mailbox := cfg.Mailbox()
	if mailbox == "" {
		b.SendNotice(ctx, evt.RoomID, "mailbox is not configured, kupo")
		return
	}

	var htmlBody string
	if !cfg.NoHTML() {
		htmlBody = format.RenderMarkdown(body, true, true).FormattedBody
	}

	tos := strings.Split(to, ",")
	// validate first
	for _, to := range tos {
		if !email.AddressValid(to) {
			b.Error(ctx, evt.RoomID, "email address is not valid")
			return
		}
	}

	b.mu.Lock(evt.RoomID.String())
	defer b.mu.Unlock(evt.RoomID.String())

	domain := utils.SanitizeDomain(cfg.Domain())
	from := mailbox + "@" + domain
	ID := email.MessageID(evt.ID, domain)
	for _, to := range tos {
		recipients := []string{to}
		eml := email.New(ID, "", " "+ID, subject, from, to, to, "", body, htmlBody, nil, nil)
		data := eml.Compose(b.cfg.GetBot().DKIMPrivateKey())
		if data == "" {
			b.SendError(ctx, evt.RoomID, "email body is empty")
			return
		}
		queued, err := b.Sendmail(evt.ID, from, to, data)
		if queued {
			b.log.Error().Err(err).Msg("cannot send email")
			b.saveSentMetadata(ctx, queued, evt.ID, recipients, eml, cfg)
			continue
		}
		if err != nil {
			b.Error(ctx, evt.RoomID, "cannot send email to %s: %v", to, err)
			continue
		}
		b.saveSentMetadata(ctx, false, evt.ID, recipients, eml, cfg)
	}
	if len(tos) > 1 {
		b.SendNotice(ctx, evt.RoomID, "All emails were sent.")
	}
}
