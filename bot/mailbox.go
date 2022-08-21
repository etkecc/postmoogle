package bot

import (
	"context"

	"github.com/getsentry/sentry-go"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
)

func (b *Bot) handleMailbox(ctx context.Context, evt *event.Event, command []string) {
	if len(command) == 1 {
		b.getMailbox(ctx, evt)
		return
	}
	b.setMailbox(ctx, evt, command[1])
}

func (b *Bot) getMailbox(ctx context.Context, evt *event.Event) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("getMailbox"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil || cfg == nil {
		b.Error(span.Context(), evt.RoomID, "cannot get settings: %v", err)
		return
	}

	if cfg.Mailbox == "" {
		b.Error(span.Context(), evt.RoomID, "mailbox name is not set")
		return
	}

	content := format.RenderMarkdown("Mailbox of this room is **"+cfg.Mailbox+"@"+b.domain+"**", true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}

func (b *Bot) setMailbox(ctx context.Context, evt *event.Event, mailbox string) {
	span := sentry.StartSpan(ctx, "http.server", sentry.TransactionName("setMailbox"))
	defer span.Finish()

	cfg, err := b.getSettings(span.Context(), evt.RoomID)
	if err != nil {
		b.log.Warn("cannot get settings: %v", err)
	}
	if cfg == nil {
		cfg = &settings{}
	}
	cfg.Mailbox = mailbox
	err = b.setSettings(span.Context(), evt.RoomID, cfg)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot update settings: %v", err)
		return
	}

	content := format.RenderMarkdown("Mailbox of this room set to **"+cfg.Mailbox+"@"+b.domain+"**", true, true)
	content.MsgType = event.MsgNotice
	_, err = b.lp.Send(evt.RoomID, content)
	if err != nil {
		b.Error(span.Context(), evt.RoomID, "cannot send message: %v", err)
	}
}
