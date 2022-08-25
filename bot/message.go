package bot

import (
	"context"
	"strings"
)

func (b *Bot) handle(ctx context.Context) {
	evt := eventFromContext(ctx)
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
