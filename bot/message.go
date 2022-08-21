package bot

import (
	"context"
	"strings"

	"maunium.net/go/mautrix/event"
)

func (b *Bot) handle(ctx context.Context, evt *event.Event) {
	content := evt.Content.AsMessage()
	if content == nil {
		b.Error(ctx, evt.RoomID, "cannot read message")
		return
	}
	message := strings.TrimSpace(content.Body)
	command := b.parseCommand(message)
	if command == nil {
		return
	}

	b.handleCommand(ctx, evt, command)
}
