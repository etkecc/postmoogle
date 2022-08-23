package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"
)

func (b *Bot) handleOwner(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getOwner(ctx, evt)
		return
	}
	b.setOwner(ctx, evt, command[1])
}

func (b *Bot) getOwner(ctx context.Context, evt *event.Event) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getOwner"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if cfg.Owner == "" {
		b.Error(span.Context(), evt.RoomID, "owner is not set yet")
		return
	}

	content := format.RenderMarkdown("Owner of this room is "+cfg.Owner.String(), true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) setOwner(ctx context.Context, evt *event.Event, owner string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setOwner"))
	defer span.Finish()

	ownerID := id.UserID(owner)
	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "failed to retrieve setting: %v", err)
		return
	}

	if !cfg.Allowed(b.noowner, evt.Sender) {
		b.Error(span.Context(), evt.RoomID, "you don't have permission to do that")
		return
	}

	cfg.Owner = ownerID
	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	content := format.RenderMarkdown("Owner of this room set to "+owner, true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}
