package bot

import (
	"context"
	"strings"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

var commands = map[string]string{
	"mailbox": "Get or set mailbox of that room",
	"help":    "Get help",
}

func (b *Bot) handleCommand(ctx context.Context, evt *event.Event, command []string) {
	if _, ok := commands[command[0]]; !ok {
		return
	}

	switch command[0] {
	case "help":
		b.sendHelp(ctx, evt.RoomID)
	case "mailbox":
		b.handleMailbox(ctx, evt, command)
	}
}

func (b *Bot) parseCommand(message string) []string {
	if message == "" {
		return nil
	}

	message = strings.TrimSpace(strings.Replace(message, b.prefix, "", 1))
	return strings.Split(message, " ")
}

func (b *Bot) sendHelp(ctx context.Context, roomID id.RoomID) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("sendHelp"))
	defer span.Finish()

	var msg strings.Builder
	msg.WriteString("the following commands are supported:\n\n")
	for name, desc := range commands {
		msg.WriteString("* **")
		msg.WriteString(name)
		msg.WriteString("** - ")
		msg.WriteString(desc)
		msg.WriteString("\n")
	}

	content := format.RenderMarkdown(msg.String(), true, true)
	content.MsgType = event.MsgNotice
	_, err := b.lp.Send(roomID, content)
	if err != nil {
		b.Error(span.Context(), roomID, "cannot send message: %v", err)
	}
}
