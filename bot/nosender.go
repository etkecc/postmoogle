package bot

import (
	"context"
	"fmt"

	"github.com/getsentry/sentry-go"
	"gitlab.com/etke.cc/postmoogle/utils"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

func (b *Bot) handleNoSender(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getNoSender(ctx, evt)
		return
	}
	b.setNoSender(ctx, evt, command[1])
}

func (b *Bot) getNoSender(ctx context.Context, evt *event.Event) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getNoSender"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	content := format.RenderMarkdown(fmt.Sprintf("`nosender` of this room is **%t**", cfg.NoSender), true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) setNoSender(ctx context.Context, evt *event.Event, value string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setNoSender"))
	defer span.Finish()

	nosender := utils.Bool(value)
	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Notice(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.NoSender = nosender
	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	content := format.RenderMarkdown(fmt.Sprintf("`nosender` of this room set to **%t**", nosender), true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}
